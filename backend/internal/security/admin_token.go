package security

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const adminIssuer = "bongkey-api"
const adminAudience = "bongkey-admin"
const adminSessionLifetime = 24 * time.Hour

var errInvalidAdminToken = errors.New("invalid admin token")

type adminClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// IssueAdminToken creates a short-lived owner session after credentials are checked.
func IssueAdminToken(secret string, id int64) (string, error) {
	if len(secret) < 32 || id <= 0 {
		return "", errInvalidAdminToken
	}
	now := time.Now()
	claims := adminClaims{
		Role: "owner",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    adminIssuer,
			Subject:   strconv.FormatInt(id, 10),
			Audience:  jwt.ClaimStrings{adminAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(adminSessionLifetime)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// VerifyAdminToken accepts only signed, unexpired owner tokens for this API.
func VerifyAdminToken(secret, raw string) (int64, error) {
	if len(secret) < 32 {
		return 0, errInvalidAdminToken
	}
	claims := &adminClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(),
		jwt.WithIssuer(adminIssuer), jwt.WithAudience(adminAudience), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.Role != "owner" || claims.IssuedAt == nil {
		return 0, errInvalidAdminToken
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidAdminToken
	}
	return id, nil
}
