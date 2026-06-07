package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const contextKeyUsername = "username"

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

		// 2. Fallback to query parameter "token" (useful for browser media requests)
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": 401, "message": "Authorization token required (via Header or token query param)"},
			})
			return
		}

		claims, err := ParseAccessToken(token, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": 401, "message": "Invalid or expired token: " + err.Error()},
			})
			return
		}

		c.Set(contextKeyUsername, claims.Username)
		c.Next()
	}
}

// GetUsername extracts the authenticated username from the Gin context
func GetUsername(c *gin.Context) string {
	v, _ := c.Get(contextKeyUsername)
	s, _ := v.(string)
	return s
}
