package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const middlewareSecret = "middleware-test-secret"

// protectedRouter dựng route được Middleware bảo vệ, ghi lại username mà
// middleware gán vào context để test xác nhận danh tính đi đúng.
func protectedRouter(t *testing.T, gotUsername *string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/protected", Middleware(middlewareSecret), func(c *gin.Context) {
		*gotUsername = GetUsername(c)
		c.Status(http.StatusNoContent)
	})
	return r
}

func accessTokenFor(t *testing.T, username string) string {
	t.Helper()
	pair, err := GenerateTokenPair(username, middlewareSecret, time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	return pair.AccessToken
}

func TestMiddlewareAcceptsBearerHeader(t *testing.T) {
	var username string
	router := protectedRouter(t, &username)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessTokenFor(t, "alice"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, muốn %d: %s", w.Code, http.StatusNoContent, w.Body.String())
	}
	if username != "alice" {
		t.Fatalf("username trong context = %q, muốn %q", username, "alice")
	}
}

func TestMiddlewareBearerSchemeIsCaseInsensitive(t *testing.T) {
	var username string
	router := protectedRouter(t, &username)

	for _, scheme := range []string{"Bearer", "bearer", "BEARER"} {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", scheme+" "+accessTokenFor(t, "alice"))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("scheme %q trả về %d", scheme, w.Code)
		}
	}
}

func TestMiddlewareAcceptsSessionCookie(t *testing.T) {
	var username string
	router := protectedRouter(t, &username)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "file_session", Value: accessTokenFor(t, "bob")})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if username != "bob" {
		t.Fatalf("username = %q, muốn %q", username, "bob")
	}
}

func TestMiddlewareAcceptsLegacyTokenCookie(t *testing.T) {
	var username string
	router := protectedRouter(t, &username)

	// Cookie "token" là tên cũ, vẫn phải nhận để phiên đang mở không bị đá ra.
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: accessTokenFor(t, "carol")})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if username != "carol" {
		t.Fatalf("username = %q, muốn %q", username, "carol")
	}
}

func TestMiddlewareRejectsMissingOrMalformedCredentials(t *testing.T) {
	var username string
	router := protectedRouter(t, &username)

	tests := []struct {
		name   string
		header string
	}{
		{"không có header", ""},
		{"thiếu scheme", accessTokenFor(t, "alice")},
		{"scheme sai", "Basic " + accessTokenFor(t, "alice")},
		{"Bearer rỗng", "Bearer "},
		{"token rác", "Bearer khong-phai-jwt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, muốn %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestMiddlewareRejectsTokenSignedWithOtherSecret(t *testing.T) {
	var username string
	router := protectedRouter(t, &username)

	pair, err := GenerateTokenPair("mallory", "secret-khac", time.Hour, time.Hour)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, muốn %d", w.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsExpiredToken(t *testing.T) {
	var username string
	router := protectedRouter(t, &username)

	pair, err := GenerateTokenPair("alice", middlewareSecret, -time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, muốn %d", w.Code, http.StatusUnauthorized)
	}
}

// ── chế độ xác thực từ xa ─────────────────────────────────────────────

func TestMiddlewareRemoteModeAcceptsVerifiedToken(t *testing.T) {
	var gotAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"remote-user"}`))
	}))
	defer server.Close()

	t.Setenv("JWT_VALIDATION_MODE", "remote")
	t.Setenv("API_URL", server.URL)

	var username string
	router := protectedRouter(t, &username)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer token-mo")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if username != "remote-user" {
		t.Fatalf("username = %q, muốn %q", username, "remote-user")
	}
	if gotAuthHeader != "Bearer token-mo" {
		t.Errorf("verifier nhận Authorization = %q", gotAuthHeader)
	}
}

func TestMiddlewareRemoteModeRejectsWhenVerifierSaysNo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	t.Setenv("JWT_VALIDATION_MODE", "remote")
	t.Setenv("API_URL", server.URL)

	var username string
	router := protectedRouter(t, &username)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer token-mo")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, muốn %d", w.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRemoteModeWithoutAPIURLIsServerError(t *testing.T) {
	// Thiếu API_URL là lỗi cấu hình của server, không phải lỗi của client,
	// nên phải là 500 chứ không phải 401.
	t.Setenv("JWT_VALIDATION_MODE", "remote")
	t.Setenv("API_URL", "")

	var username string
	router := protectedRouter(t, &username)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer token-mo")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, muốn %d", w.Code, http.StatusInternalServerError)
	}
}

func TestValidateRemoteRejectsUnreachableVerifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close() // đóng luôn để địa chỉ không còn ai nghe

	if _, err := validateRemote(context.Background(), "token", url); err == nil {
		t.Fatal("verifier không kết nối được phải trả lỗi")
	}
}

func TestValidateRemoteRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`day khong phai json`))
	}))
	defer server.Close()

	if _, err := validateRemote(context.Background(), "token", server.URL); err == nil {
		t.Fatal("phản hồi không phải JSON phải trả lỗi")
	}
}

func TestValidateRemoteRejectsBlankUsername(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"   "}`))
	}))
	defer server.Close()

	if _, err := validateRemote(context.Background(), "token", server.URL); err == nil {
		t.Fatal("username toàn khoảng trắng phải bị từ chối")
	}
}

func TestValidateRemoteRejectsBadURL(t *testing.T) {
	if _, err := validateRemote(context.Background(), "token", "://khong-hop-le"); err == nil {
		t.Fatal("URL hỏng phải trả lỗi")
	}
}

func TestValidateLocalReturnsUsername(t *testing.T) {
	username, err := validateLocal(accessTokenFor(t, "dave"), middlewareSecret)
	if err != nil {
		t.Fatalf("validateLocal: %v", err)
	}
	if username != "dave" {
		t.Fatalf("username = %q, muốn %q", username, "dave")
	}
}

func TestGetUsernameReturnsEmptyWhenAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	if got := GetUsername(c); got != "" {
		t.Fatalf("GetUsername = %q, muốn chuỗi rỗng", got)
	}
}
