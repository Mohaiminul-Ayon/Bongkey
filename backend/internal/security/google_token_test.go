package security

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testClientID = "shop-client.apps.googleusercontent.com"

// staticKeys serves one generated key under one key ID, like a single-key Google JWK set.
type staticKeys struct {
	id  string
	key *rsa.PublicKey
}

// Key returns the key only for its own ID.
func (s staticKeys) Key(_ context.Context, id string) (*rsa.PublicKey, error) {
	if id != s.id {
		return nil, ErrInvalidGoogleToken
	}
	return s.key, nil
}

// googleToken signs claims the way Google would, with the given key ID.
func googleToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

// TestGoogleVerifier accepts only a current Google token for this client with a verified email.
func TestGoogleVerifier(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	verifier := GoogleVerifier{ClientID: testClientID, Keys: staticKeys{id: "k1", key: &key.PublicKey}}
	good := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss": "https://accounts.google.com", "aud": testClientID, "sub": "1234567890",
			"email": "Rahim@Example.com", "email_verified": true, "name": " Rahim ",
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		}
	}
	identity, err := verifier.Verify(context.Background(), googleToken(t, key, "k1", good()))
	if err != nil || identity != (GoogleIdentity{Subject: "1234567890", Email: "rahim@example.com", Name: "Rahim"}) {
		t.Fatalf("valid token: %+v %v", identity, err)
	}
	short := good()
	short["iss"] = "accounts.google.com"
	if _, err := verifier.Verify(context.Background(), googleToken(t, key, "k1", short)); err != nil {
		t.Fatalf("short issuer refused: %v", err)
	}

	for name, change := range map[string]func(jwt.MapClaims){
		"expired":          func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
		"no expiry":        func(c jwt.MapClaims) { delete(c, "exp") },
		"wrong audience":   func(c jwt.MapClaims) { c["aud"] = "someone-else.apps.googleusercontent.com" },
		"wrong issuer":     func(c jwt.MapClaims) { c["iss"] = "https://evil.example" },
		"unverified email": func(c jwt.MapClaims) { c["email_verified"] = false },
		"no email":         func(c jwt.MapClaims) { delete(c, "email") },
		"no subject":       func(c jwt.MapClaims) { delete(c, "sub") },
	} {
		claims := good()
		change(claims)
		if _, err := verifier.Verify(context.Background(), googleToken(t, key, "k1", claims)); !errors.Is(err, ErrInvalidGoogleToken) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}

	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := verifier.Verify(context.Background(), googleToken(t, other, "k1", good())); err == nil {
		t.Error("token signed by another key accepted")
	}
	if _, err := verifier.Verify(context.Background(), googleToken(t, key, "unknown", good())); err == nil {
		t.Error("unknown key ID accepted")
	}
	// An HMAC token must never be accepted, even if someone signs it with public key bytes.
	hmacToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, good()).SignedString(key.PublicKey.N.Bytes())
	if _, err := verifier.Verify(context.Background(), hmacToken); err == nil {
		t.Error("HS256 token accepted")
	}
	if _, err := (GoogleVerifier{Keys: verifier.Keys}).Verify(context.Background(), googleToken(t, key, "k1", good())); err == nil {
		t.Error("missing client ID accepted a token")
	}
}

// TestGoogleKeysCachesDownloadedSet parses a JWK set once and refreshes at most once a minute for unknown IDs.
func TestGoogleKeysCachesDownloadedSet(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
	downloads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads++
		fmt.Fprintf(w, `{"keys":[{"kty":"RSA","kid":"k1","alg":"RS256","n":%q,"e":%q},{"kty":"EC","kid":"skip"}]}`, n, e)
	}))
	defer server.Close()

	keys := &GoogleKeys{URL: server.URL, Client: server.Client()}
	for range 3 {
		got, err := keys.Key(context.Background(), "k1")
		if err != nil || got.N.Cmp(key.N) != 0 || got.E != key.E {
			t.Fatalf("key not parsed: %v", err)
		}
	}
	if _, err := keys.Key(context.Background(), "forged"); err == nil {
		t.Fatal("unknown key ID returned a key")
	}
	if _, err := keys.Key(context.Background(), "forged-again"); err == nil {
		t.Fatal("unknown key ID returned a key")
	}
	if downloads != 1 {
		t.Fatalf("downloads = %d, want 1 inside the refresh floor", downloads)
	}
}
