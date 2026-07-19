package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ckfindercompatible/backend/internal/auth"
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

func TestExternalTokenGeneratesInternalTokenAfterExternalVerify(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotAuth, gotAppID, gotAPIKey string
	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAppID = r.Header.Get("x-app-id")
		gotAPIKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"admin"}`))
	}))
	defer verifyServer.Close()

	cfg := testAuthConfig()
	cfg.ExternalAuthVerifyURL = verifyServer.URL
	cfg.ExternalAuthAppID = "file-manager"
	cfg.ExternalAuthAPIKey = "verify-secret"
	handler := NewAuthHandler(cfg)
	router := gin.New()
	router.POST("/auth/external-token", handler.ExternalToken)

	w := postExternalToken(router, "Bearer external-token")
	if w.Code != http.StatusOK {
		t.Fatalf("external token returned %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if gotAuth != "Bearer external-token" {
		t.Fatalf("verify Authorization = %q, want %q", gotAuth, "Bearer external-token")
	}
	if gotAppID != "file-manager" {
		t.Fatalf("verify x-app-id = %q, want %q", gotAppID, "file-manager")
	}
	if gotAPIKey != "verify-secret" {
		t.Fatalf("verify x-api-key = %q, want %q", gotAPIKey, "verify-secret")
	}

	var resp models.TokenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if resp.TokenType != "Bearer" || resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("unexpected token response: %+v", resp)
	}

	claims, err := auth.ParseAccessToken(resp.AccessToken, cfg.JWTSecret)
	if err != nil {
		t.Fatalf("parse internal access token: %v", err)
	}
	if claims.Username != "admin" {
		t.Fatalf("internal token username = %q, want %q", claims.Username, "admin")
	}
}

func TestExternalTokenRequiresBearerWithoutCallingVerify(t *testing.T) {
	gin.SetMode(gin.TestMode)

	called := false
	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer verifyServer.Close()

	cfg := testAuthConfig()
	cfg.ExternalAuthVerifyURL = verifyServer.URL
	handler := NewAuthHandler(cfg)
	router := gin.New()
	router.POST("/auth/external-token", handler.ExternalToken)

	w := postExternalToken(router, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer returned %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("verify service was called without bearer token")
	}
}

func TestExternalTokenRejectsUnauthorizedExternalToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer verifyServer.Close()

	cfg := testAuthConfig()
	cfg.ExternalAuthVerifyURL = verifyServer.URL
	handler := NewAuthHandler(cfg)
	router := gin.New()
	router.POST("/auth/external-token", handler.ExternalToken)

	w := postExternalToken(router, "Bearer bad-token")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized external token returned %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestExternalTokenReturnsBadGatewayWhenVerifyFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer verifyServer.Close()

	cfg := testAuthConfig()
	cfg.ExternalAuthVerifyURL = verifyServer.URL
	handler := NewAuthHandler(cfg)
	router := gin.New()
	router.POST("/auth/external-token", handler.ExternalToken)

	w := postExternalToken(router, "Bearer external-token")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("failed verify returned %d, want %d", w.Code, http.StatusBadGateway)
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
		ExternalAuthTimeout:    time.Second,
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

func postExternalToken(router *gin.Engine, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/external-token", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
