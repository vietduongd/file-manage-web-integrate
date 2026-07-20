package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/gin-gonic/gin"
)

type embedTicketVerifyRequest struct {
	Ticket string `json:"ticket"`
}

type embedTicketVerifyResponse struct {
	Data struct {
		AdminID  string   `json:"adminId"`
		UserName string   `json:"userName"`
		FullName string   `json:"fullName"`
		Roles    []string `json:"roles"`
	} `json:"data"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// EmbedLogin handles POST /auth/embed/login
func (h *AuthHandler) EmbedLogin(c *gin.Context) {
	var req struct {
		Ticket string `json:"ticket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: models.ErrorDetail{Code: 400, Message: "Missing ticket in request body"},
		})
		return
	}
	ticket := req.Ticket

	username, err := h.verifyEmbedTicket(c.Request.Context(), ticket)
	if err != nil {
		status := http.StatusUnauthorized
		message := "Invalid or expired ticket"
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

func (h *AuthHandler) verifyEmbedTicket(ctx context.Context, ticket string) (string, error) {
	verifyURL := strings.TrimSpace(h.cfg.EmbedTicketVerifyURL)
	if verifyURL == "" {
		return "", externalAuthError{
			status:  http.StatusServiceUnavailable,
			message: "External auth verifier is not configured",
		}
	}

	reqBody := embedTicketVerifyRequest{Ticket: ticket}
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return "", externalAuthError{
			status:  http.StatusInternalServerError,
			message: "Failed to marshal verify request",
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, bytes.NewReader(reqJSON))
	if err != nil {
		return "", externalAuthError{
			status:  http.StatusServiceUnavailable,
			message: "External auth verifier is misconfigured",
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	serviceKey := strings.TrimSpace(h.cfg.EmbedTicketServiceKey)
	if serviceKey == "" {
		return "", externalAuthError{
			status:  http.StatusServiceUnavailable,
			message: "Embed ticket service key is not configured",
		}
	}
	req.Header.Set("X-Service-Key", serviceKey)

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
			message: "Ticket is invalid",
		}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", externalAuthError{
			status:  http.StatusBadGateway,
			message: fmt.Sprintf("External auth verifier returned status %d", resp.StatusCode),
		}
	}

	var verified embedTicketVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verified); err != nil {
		return "", externalAuthError{
			status:  http.StatusBadGateway,
			message: "Invalid external auth response",
		}
	}

	username := strings.TrimSpace(verified.Data.UserName)
	if username == "" {
		return "", externalAuthError{
			status:  http.StatusBadGateway,
			message: "External auth response missing userName",
		}
	}
	return username, nil
}
