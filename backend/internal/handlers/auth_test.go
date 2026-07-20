package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ckfindercompatible/backend/internal/config"
	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/gin-gonic/gin"
)

func TestTokenRateLimitsFailedLoginAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(testAuthConfig())
	handler.loginLimiter = newLoginRateLimiter(5, time.Minute)
	router := gin.New()
	router.POST("/auth/token", handler.Token)

	for i := 0; i < 5; i++ {
		w := postToken(t, router, "admin", "wrong-password", "192.0.2.10")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d returned %d, want %d", i+1, w.Code, http.StatusUnauthorized)
		}
	}

	w := postToken(t, router, "admin", "wrong-password", "192.0.2.10")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("limited attempt returned %d, want %d", w.Code, http.StatusTooManyRequests)
	}
	if retryAfter := w.Header().Get("Retry-After"); retryAfter == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestTokenSuccessfulLoginResetsFailedAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(testAuthConfig())
	handler.loginLimiter = newLoginRateLimiter(5, time.Minute)
	router := gin.New()
	router.POST("/auth/token", handler.Token)

	for i := 0; i < 3; i++ {
		w := postToken(t, router, "admin", "wrong-password", "192.0.2.11")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d returned %d, want %d", i+1, w.Code, http.StatusUnauthorized)
		}
	}

	w := postToken(t, router, "admin", "secret", "192.0.2.11")
	if w.Code != http.StatusOK {
		t.Fatalf("successful login returned %d, want %d", w.Code, http.StatusOK)
	}

	for i := 0; i < 5; i++ {
		w := postToken(t, router, "admin", "wrong-password", "192.0.2.11")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("post-reset attempt %d returned %d, want %d", i+1, w.Code, http.StatusUnauthorized)
		}
	}
}

func TestEmbedLoginRequiresTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testAuthConfig()
	handler := NewAuthHandler(cfg)
	router := gin.New()
	router.POST("/auth/embed/login", handler.EmbedLogin)

	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/embed/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing ticket returned %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestEmbedLoginRequiresVerifierConfigInDevelopment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testAuthConfig()
	cfg.ServerEnv = "development"
	handler := NewAuthHandler(cfg)
	router := gin.New()
	router.POST("/auth/embed/login", handler.EmbedLogin)

	body := []byte(`{"ticket":"any-ticket"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/embed/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing verifier config returned %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestEmbedLoginVerifiesTicketAndReturnsTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotServiceKey, gotBody string
	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotServiceKey = r.Header.Get("X-Service-Key")
		bodyBytes, _ := io.ReadAll(r.Body)
		gotBody = string(bodyBytes)

		w.Header().Set("Content-Type", "application/json")
		// Response format as per requirement/integrate.md
		_, _ = w.Write([]byte(`{
			"data": {
				"adminId": "a587bb8d-84c3-463a-ab05-eadf949e91a5",
				"userName": "manage",
				"fullName": "Quản trị viên",
				"roles": []
			},
			"message": "",
			"code": ""
		}`))
	}))
	defer verifyServer.Close()

	cfg := testAuthConfig()
	cfg.EmbedTicketVerifyURL = verifyServer.URL
	cfg.EmbedTicketServiceKey = "test-service-key"

	handler := NewAuthHandler(cfg)
	router := gin.New()
	router.POST("/auth/embed/login", handler.EmbedLogin)

	body := []byte(`{"ticket":"test-ticket-123"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/embed/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("successful login returned status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	if gotServiceKey != "test-service-key" {
		t.Fatalf("verify X-Service-Key = %q, want %q", gotServiceKey, "test-service-key")
	}

	if !strings.Contains(gotBody, `"ticket":"test-ticket-123"`) {
		t.Fatalf("verify request body = %q, should contain ticket %q", gotBody, "test-ticket-123")
	}

	var resp models.TokenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if resp.TokenType != "Bearer" || resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("unexpected token response: %+v", resp)
	}
}

func TestEmbedLoginHandlesInvalidTicket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer verifyServer.Close()

	cfg := testAuthConfig()
	cfg.EmbedTicketVerifyURL = verifyServer.URL
	cfg.EmbedTicketServiceKey = "test-service-key"
	handler := NewAuthHandler(cfg)
	router := gin.New()
	router.POST("/auth/embed/login", handler.EmbedLogin)

	body := []byte(`{"ticket":"invalid-ticket"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/embed/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("invalid ticket returned status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func testAuthConfig() *config.Config {
	return &config.Config{
		AdminUsername:          "admin",
		AdminPassword:          "secret",
		JWTSecret:              "test-secret",
		JWTExpiryHours:         1,
		JWTRefreshExpiryHours:  24,
		LoginRateLimitMax:      5,
		LoginRateLimitWindow:   time.Minute,
		LoginRateLimitDisabled: false,
	}
}

func postToken(t *testing.T, router *gin.Engine, username, password, ip string) *httptest.ResponseRecorder {
	t.Helper()

	body := []byte(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
