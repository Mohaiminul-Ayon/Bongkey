package security

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// googleCertsURL publishes Google's current ID-token signing keys as a JWK set.
const googleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

// ErrInvalidGoogleToken hides why a Google sign-in token was refused.
var ErrInvalidGoogleToken = errors.New("invalid Google sign-in token")

// GoogleIdentity is the verified person behind a Google sign-in.
type GoogleIdentity struct {
	// Subject is Google's stable account ID; unlike the email it never changes.
	Subject string
	Email   string
	Name    string
}

// GoogleKeySource returns the RSA public key Google used for one token, by key ID.
type GoogleKeySource interface {
	Key(ctx context.Context, keyID string) (*rsa.PublicKey, error)
}

// GoogleVerifier checks Google ID tokens from "Sign in with Google" for this shop's OAuth client.
type GoogleVerifier struct {
	ClientID string
	Keys     GoogleKeySource
}

// googleClaims are the ID-token fields the shop relies on.
type googleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	jwt.RegisteredClaims
}

// Verify accepts only an RS256 token signed by a current Google key, issued by Google for this client ID,
// not expired, with a subject and a verified email. A valid token proves who signed in; it grants no admin
// rights and no access to orders by itself.
func (v GoogleVerifier) Verify(ctx context.Context, raw string) (GoogleIdentity, error) {
	if v.ClientID == "" || v.Keys == nil || raw == "" || len(raw) > 8<<10 {
		return GoogleIdentity{}, ErrInvalidGoogleToken
	}
	var claims googleClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		keyID, _ := token.Header["kid"].(string)
		return v.Keys.Key(ctx, keyID)
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithAudience(v.ClientID),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return GoogleIdentity{}, ErrInvalidGoogleToken
	}
	// Google uses either issuer spelling; anything else is not a Google sign-in.
	if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
		return GoogleIdentity{}, ErrInvalidGoogleToken
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if claims.Subject == "" || email == "" || !claims.EmailVerified {
		return GoogleIdentity{}, ErrInvalidGoogleToken
	}
	return GoogleIdentity{Subject: claims.Subject, Email: email, Name: strings.TrimSpace(claims.Name)}, nil
}

// GoogleKeys downloads Google's signing keys and caches them until their published expiry.
type GoogleKeys struct {
	// URL and Client are replaced only in tests.
	URL    string
	Client *http.Client
	// mu protects keys, expires and lastTry, which every sign-in reads and a refresh replaces.
	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
	lastTry time.Time
}

// keyRefreshFloor stops forged unknown key IDs from making every request download the key set again.
const keyRefreshFloor = time.Minute

// Key returns the cached key, refreshing the set when it expired or the key ID is new (Google rotates keys).
// The download happens under the lock, which is acceptable for a rare refresh in a small shop.
func (g *GoogleKeys) Key(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if key, ok := g.keys[keyID]; ok && now.Before(g.expires) {
		return key, nil
	}
	if now.Sub(g.lastTry) >= keyRefreshFloor {
		g.lastTry = now
		if err := g.refresh(ctx, now); err != nil {
			return nil, err
		}
	}
	// Within the floor an expired set is still used: Google keeps old keys published for days.
	if key, ok := g.keys[keyID]; ok {
		return key, nil
	}
	return nil, ErrInvalidGoogleToken
}

// refresh downloads and parses the JWK set; the caller holds mu.
func (g *GoogleKeys) refresh(ctx context.Context, now time.Time) error {
	url, client := g.URL, g.Client
	if url == "" {
		url = googleCertsURL
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download Google keys: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download Google keys: status %d", response.StatusCode)
	}
	var set struct {
		Keys []struct {
			Kty, Kid, N, E, Alg string
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&set); err != nil {
		return fmt.Errorf("parse Google keys: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		key, err := rsaKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = key
	}
	if len(keys) == 0 {
		return errors.New("parse Google keys: no RSA keys")
	}
	// Google publishes about six hours in Cache-Control; one hour keeps rotation quick without extra downloads.
	g.keys, g.expires = keys, now.Add(time.Hour)
	return nil
}

// rsaKey builds a public key from the base64url modulus and exponent of a JWK.
func rsaKey(n, e string) (*rsa.PublicKey, error) {
	modulus, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, err
	}
	exponent, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil || len(exponent) == 0 || len(exponent) > 4 {
		return nil, errors.New("invalid RSA exponent")
	}
	value := 0
	for _, b := range exponent {
		value = value<<8 | int(b)
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: value}
	if key.N.BitLen() < 2048 || key.E < 3 {
		return nil, errors.New("weak RSA key")
	}
	return key, nil
}
