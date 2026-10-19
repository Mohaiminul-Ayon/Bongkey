package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const emailProofAudience = "bongkey-verified-email"

var errInvalidEmailProof = errors.New("invalid email proof")

// NewEmailCode creates a six-digit code using a cryptographic random source.
func NewEmailCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// EmailCodeHash prevents offline guessing of a short OTP from a database dump.
func EmailCodeHash(secret, email, code string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("email-otp:" + email + ":" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// EmailKey hashes an email or IP before it is stored in OTP rate-limit tables.
func EmailKey(secret, kind, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("email-otp-" + kind + ":" + value))
	return hex.EncodeToString(mac.Sum(nil))
}

// IssueEmailProof grants ten minutes of access to the verified email only.
func IssueEmailProof(secret, email string) (string, error) {
	if len(secret) < 32 || email == "" {
		return "", errInvalidEmailProof
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer: "bongkey-api", Subject: email,
		Audience: jwt.ClaimStrings{emailProofAudience},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// VerifyEmailProof rejects expired, altered, or wrong-audience email proofs.
func VerifyEmailProof(secret, raw string) (string, error) {
	if len(secret) < 32 || raw == "" {
		return "", errInvalidEmailProof
	}
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(),
		jwt.WithIssuer("bongkey-api"), jwt.WithAudience(emailProofAudience), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.Subject == "" || claims.IssuedAt == nil {
		return "", errInvalidEmailProof
	}
	return claims.Subject, nil
}
