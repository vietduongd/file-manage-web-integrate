package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareRejectsQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pair, err := GenerateTokenPair("alice", "test-secret", time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	router := gin.New()
	router.GET("/protected", Middleware("test-secret"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected?token="+pair.AccessToken, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("query token returned %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestValidateRemoteRejectsUntrustedTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"alice"}`))
	}))
	defer server.Close()

	if username, err := validateRemote(context.Background(), "opaque-token", server.URL); err == nil {
		t.Fatalf("validateRemote returned username %q with untrusted TLS, want error", username)
	}
}

func TestValidateRemoteRequiresVerifierUsername(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	if username, err := validateRemote(context.Background(), "opaque-token", server.URL); err == nil {
		t.Fatalf("validateRemote returned username %q without verifier identity, want error", username)
	}
}

func TestValidateRemoteUsesVerifierUsername(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"alice"}`))
	}))
	defer server.Close()

	username, err := validateRemote(context.Background(), "opaque-token", server.URL)
	if err != nil {
		t.Fatalf("validateRemote returned error: %v", err)
	}
	if username != "alice" {
		t.Fatalf("validateRemote username = %q, want %q", username, "alice")
	}
}
