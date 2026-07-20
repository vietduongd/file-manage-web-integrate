package handlers

import (
	"net/http"
	"time"

	"github.com/ckfindercompatible/backend/internal/auth"
	"github.com/ckfindercompatible/backend/internal/config"
	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	cfg          *config.Config
	loginLimiter *loginRateLimiter
	httpClient   *http.Client
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	var limiter *loginRateLimiter
	if !cfg.LoginRateLimitDisabled {
		limiter = newLoginRateLimiter(cfg.LoginRateLimitMax, cfg.LoginRateLimitWindow)
	}
	timeout := cfg.ExternalAuthTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &AuthHandler{
		cfg:          cfg,
		loginLimiter: limiter,
		httpClient:   &http.Client{Timeout: timeout},
	}
}

// Token handles POST /auth/token
func (h *AuthHandler) Token(c *gin.Context) {
	var req models.TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: models.ErrorDetail{Code: 400, Message: "Invalid request body: " + err.Error()},
		})
		return
	}

	clientIP := c.ClientIP()
	if h.loginLimiter != nil {
		if limit := h.loginLimiter.Check(clientIP, req.Username); limit.Limited {
			c.Header("Retry-After", retryAfterSeconds(limit.RetryAfter))
			c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
				Error: models.ErrorDetail{Code: 429, Message: "Too many failed login attempts. Please try again later."},
			})
			return
		}
	}

	// Validate credentials (simple check — replace with DB in production)
	if req.Username != h.cfg.AdminUsername || req.Password != h.cfg.AdminPassword {
		if h.loginLimiter != nil {
			h.loginLimiter.AddFailure(clientIP, req.Username)
		}
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: models.ErrorDetail{Code: 401, Message: "Invalid username or password"},
		})
		return
	}
	if h.loginLimiter != nil {
		h.loginLimiter.Reset(clientIP, req.Username)
	}

	h.writeTokenPair(c, req.Username)
}

// Refresh handles POST /auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_session")
	if err != nil || refreshToken == "" {
		var req models.RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error: models.ErrorDetail{Code: 400, Message: "Refresh token required"},
			})
			return
		}
		refreshToken = req.RefreshToken
	}

	username, err := auth.ParseRefreshToken(refreshToken, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: models.ErrorDetail{Code: 401, Message: "Invalid or expired refresh token"},
		})
		return
	}

	h.writeTokenPair(c, username)
}

// Logout clears auth cookies.
func (h *AuthHandler) Logout(c *gin.Context) {
	h.clearAuthCookies(c)
	c.JSON(http.StatusOK, gin.H{"loggedOut": true})
}

func (h *AuthHandler) writeTokenPair(c *gin.Context, username string) {
	accessTTL := time.Duration(h.cfg.JWTExpiryHours) * time.Hour
	refreshTTL := time.Duration(h.cfg.JWTRefreshExpiryHours) * time.Hour
	pair, err := auth.GenerateTokenPair(username, h.cfg.JWTSecret, accessTTL, refreshTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: models.ErrorDetail{Code: 500, Message: "Failed to generate token"},
		})
		return
	}

	h.setAuthCookies(c, pair.AccessToken, pair.RefreshToken, accessTTL, refreshTTL)
	c.JSON(http.StatusOK, models.TokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		TokenType:    "Bearer",
	})
}

func (h *AuthHandler) setAuthCookies(c *gin.Context, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) {
	secure := h.cfg.ServerEnv == "production"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("file_session", accessToken, int(accessTTL.Seconds()), "/", "", secure, true)
	c.SetCookie("refresh_session", refreshToken, int(refreshTTL.Seconds()), "/", "", secure, true)
}

func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	secure := h.cfg.ServerEnv == "production"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("file_session", "", -1, "/", "", secure, true)
	c.SetCookie("refresh_session", "", -1, "/", "", secure, true)
}
