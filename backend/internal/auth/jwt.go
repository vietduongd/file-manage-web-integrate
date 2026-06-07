package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT payload
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// TokenPair holds access + refresh tokens
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // seconds
}

// GenerateTokenPair creates an access token and refresh token for the given username
func GenerateTokenPair(username, secret string, accessTTL, refreshTTL time.Duration) (*TokenPair, error) {
	now := time.Now()

	// Access token
	accessClaims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			Issuer:    "ckfinder-compatible",
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Refresh token (longer lived, no extra claims needed)
	refreshClaims := jwt.RegisteredClaims{
		Subject:   username,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(refreshTTL)),
		Issuer:    "ckfinder-compatible-refresh",
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessTTL.Seconds()),
	}, nil
}

// ParseAccessToken validates and parses an access token
func ParseAccessToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

// ParseRefreshToken validates a refresh token and returns the subject (username)
func ParseRefreshToken(tokenStr, secret string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid refresh token")
	}
	return claims.Subject, nil
}
