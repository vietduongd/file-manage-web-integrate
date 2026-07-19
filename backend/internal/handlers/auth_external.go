package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/gin-gonic/gin"
)

type externalAuthVerifyResponse struct {
	Username string `json:"username"`
}

type externalAuthError struct {
	status  int
	message string
}

func (e externalAuthError) Error() string {
	return e.message
}

// ExternalToken handles POST /auth/external-token
func (h *AuthHandler) ExternalToken(c *gin.Context) {
	token, ok := bearerToken(c.GetHeader("Authorization"))
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: models.ErrorDetail{Code: 401, Message: "Missing bearer token"},
		})
		return
	}

	username, err := h.verifyExternalToken(c.Request.Context(), token)
	if err != nil {
		status := http.StatusBadGateway
		message := "External auth verification failed"
		if authErr, ok := err.(externalAuthError); ok {
			status = authErr.status
			message = authErr.message
		}
		c.JSON(status, models.ErrorResponse{
			Error: models.ErrorDetail{Code: status, Message: message},
		})
		return
	}

	h.writeTokenPair(c, username)
}

func (h *AuthHandler) verifyExternalToken(ctx context.Context, token string) (string, error) {
	verifyURL := strings.TrimSpace(h.cfg.ExternalAuthVerifyURL)
	if verifyURL == "" {
		return "", externalAuthError{
			status:  http.StatusServiceUnavailable,
			message: "External auth verifier is not configured",
		}
	}

	method := strings.TrimSpace(h.cfg.ExternalAuthVerifyMethod)
	if method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(method)

	req, err := http.NewRequestWithContext(ctx, method, verifyURL, nil)
	if err != nil {
		return "", externalAuthError{
			status:  http.StatusServiceUnavailable,
			message: "External auth verifier is misconfigured",
		}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if h.cfg.ExternalAuthAppID != "" {
		req.Header.Set("x-app-id", h.cfg.ExternalAuthAppID)
	}
	if h.cfg.ExternalAuthAPIKey != "" {
		req.Header.Set("x-api-key", h.cfg.ExternalAuthAPIKey)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", externalAuthError{
			status:  http.StatusBadGateway,
			message: "External auth verifier is unavailable",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", externalAuthError{
			status:  http.StatusUnauthorized,
			message: "External token is invalid",
		}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", externalAuthError{
			status:  http.StatusBadGateway,
			message: fmt.Sprintf("External auth verifier returned status %d", resp.StatusCode),
		}
	}

	var verified externalAuthVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verified); err != nil {
		return "", externalAuthError{
			status:  http.StatusBadGateway,
			message: "Invalid external auth response",
		}
	}

	username := strings.TrimSpace(verified.Username)
	if username == "" {
		return "", externalAuthError{
			status:  http.StatusBadGateway,
			message: "External auth response missing username",
		}
	}
	return username, nil
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
