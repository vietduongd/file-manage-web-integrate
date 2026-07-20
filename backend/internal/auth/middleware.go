package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const contextKeyUsername = "username"

var remoteAuthClient = &http.Client{
	Timeout: 10 * time.Second,
}

type remoteAuthVerifyResponse struct {
	Username string `json:"username"`
}

// Middleware returns a Gin middleware that validates Bearer JWT tokens
func Middleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// 1. Try to get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = parts[1]
			}
		}

		// 2. Fallback to cookies (both file_session and legacy token cookie)
		if token == "" {
			if cookieToken, err := c.Cookie("file_session"); err == nil {
				token = cookieToken
			} else if cookieToken, err := c.Cookie("token"); err == nil {
				token = cookieToken
			}
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": 401, "message": "Authorization token required"},
			})
			return
		}

		var username string
		var err error

		validationMode := strings.ToLower(os.Getenv("JWT_VALIDATION_MODE"))
		if validationMode == "remote" {
			apiURL := os.Getenv("API_URL")
			username, err = validateRemote(c.Request.Context(), token, apiURL)
			if err != nil {
				if strings.Contains(err.Error(), "environment variable") {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
						"error": gin.H{"code": 500, "message": err.Error()},
					})
				} else {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
						"error": gin.H{"code": 401, "message": err.Error()},
					})
				}
				return
			}
		} else {
			username, err = validateLocal(token, jwtSecret)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{"code": 401, "message": "Invalid or expired token: " + err.Error()},
				})
				return
			}
		}

		c.Set(contextKeyUsername, username)
		c.Next()
	}
}

// validateLocal validates the token locally using the JWT secret and returns the username
func validateLocal(token, jwtSecret string) (string, error) {
	claims, err := ParseAccessToken(token, jwtSecret)
	if err != nil {
		return "", err
	}
	return claims.Username, nil
}

// validateRemote validates the token against an external API_URL and returns the username
func validateRemote(ctx context.Context, token, apiURL string) (string, error) {
	if apiURL == "" {
		return "", errors.New("API_URL environment variable is not configured for remote validation")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create remote validation request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/4.0 (compatible; MSIE 8.0; Windows NT 6.0)")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := remoteAuthClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("remote token validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("invalid or expired token (remote check)")
	}

	var verified remoteAuthVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verified); err != nil {
		return "", fmt.Errorf("invalid remote validation response: %w", err)
	}

	username := strings.TrimSpace(verified.Username)
	if username == "" {
		return "", errors.New("remote validation response missing username")
	}

	return username, nil
}

// GetUsername extracts the authenticated username from the Gin context
func GetUsername(c *gin.Context) string {
	v, _ := c.Get(contextKeyUsername)
	s, _ := v.(string)
	return s
}
