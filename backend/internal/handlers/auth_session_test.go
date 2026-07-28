package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ckfindercompatible/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func newAuthRouter(t *testing.T, mutate func(*gin.Engine, *AuthHandler)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler(testAuthConfig())
	router := gin.New()
	router.POST("/auth/token", handler.Token)
	router.POST("/auth/refresh", handler.Refresh)
	router.POST("/auth/logout", handler.Logout)
	if mutate != nil {
		mutate(router, handler)
	}
	return router
}

func findCookie(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("không tìm thấy cookie %q trong %v", name, w.Result().Cookies())
	return nil
}

func postJSON(t *testing.T, router *gin.Engine, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.7:12345"
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ── POST /auth/token ──────────────────────────────────────────────────

func TestTokenRejectsMalformedBody(t *testing.T) {
	router := newAuthRouter(t, nil)

	tests := []struct {
		name string
		body string
	}{
		{"JSON hỏng", `{"username":`},
		{"thiếu password", `{"username":"admin"}`},
		{"thiếu username", `{"password":"secret"}`},
		{"body rỗng", `{}`},
		{"password rỗng", `{"username":"admin","password":""}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := postJSON(t, router, "/auth/token", tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("trả về %d, muốn %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
			}
		})
	}
}

func TestTokenRejectsWrongUsername(t *testing.T) {
	router := newAuthRouter(t, nil)

	w := postToken(t, router, "not-admin", "secret", "198.51.100.8")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("trả về %d, muốn %d", w.Code, http.StatusUnauthorized)
	}
}

func TestTokenSetsHttpOnlySessionCookies(t *testing.T) {
	router := newAuthRouter(t, nil)

	w := postToken(t, router, "admin", "secret", "198.51.100.9")
	if w.Code != http.StatusOK {
		t.Fatalf("đăng nhập trả về %d: %s", w.Code, w.Body.String())
	}

	for _, name := range []string{"file_session", "refresh_session"} {
		c := findCookie(t, w, name)
		if !c.HttpOnly {
			t.Errorf("cookie %s phải HttpOnly để JS không đọc được token", name)
		}
		if c.Value == "" {
			t.Errorf("cookie %s rỗng", name)
		}
		if c.Path != "/" {
			t.Errorf("cookie %s có Path = %q, muốn \"/\"", name, c.Path)
		}
	}
}

func TestTokenCookieSecureFlagFollowsEnvironment(t *testing.T) {
	tests := []struct {
		env        string
		wantSecure bool
	}{
		{"production", true},
		{"development", false},
	}

	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			cfg := testAuthConfig()
			cfg.ServerEnv = tc.env
			router := gin.New()
			router.POST("/auth/token", NewAuthHandler(cfg).Token)

			w := postToken(t, router, "admin", "secret", "198.51.100.10")
			if got := findCookie(t, w, "file_session").Secure; got != tc.wantSecure {
				t.Fatalf("Secure = %v ở env %q, muốn %v", got, tc.env, tc.wantSecure)
			}
		})
	}
}

func TestTokenSkipsRateLimitingWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testAuthConfig()
	cfg.LoginRateLimitDisabled = true
	handler := NewAuthHandler(cfg)
	if handler.loginLimiter != nil {
		t.Fatal("LoginRateLimitDisabled=true thì loginLimiter phải nil")
	}

	router := gin.New()
	router.POST("/auth/token", handler.Token)

	// Vượt xa ngưỡng 5 mà vẫn phải là 401, không được ra 429.
	for i := 0; i < 20; i++ {
		w := postToken(t, router, "admin", "sai-mat-khau", "198.51.100.11")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("lần thử %d trả về %d, muốn %d", i+1, w.Code, http.StatusUnauthorized)
		}
	}
}

// ── POST /auth/refresh ────────────────────────────────────────────────

func TestRefreshAcceptsCookie(t *testing.T) {
	router := newAuthRouter(t, nil)

	login := postToken(t, router, "admin", "secret", "198.51.100.12")
	refreshCookie := findCookie(t, login, "refresh_session")

	w := postJSON(t, router, "/auth/refresh", `{}`, refreshCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh bằng cookie trả về %d: %s", w.Code, w.Body.String())
	}
	if findCookie(t, w, "file_session").Value == "" {
		t.Fatal("refresh phải cấp access token mới")
	}
}

func TestRefreshFallsBackToBodyWhenNoCookie(t *testing.T) {
	router := newAuthRouter(t, nil)

	pair, err := auth.GenerateTokenPair("admin", "test-secret", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("tạo token: %v", err)
	}

	w := postJSON(t, router, "/auth/refresh", `{"refresh_token":"`+pair.RefreshToken+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh bằng body trả về %d: %s", w.Code, w.Body.String())
	}
}

func TestRefreshRejectsBadTokens(t *testing.T) {
	router := newAuthRouter(t, nil)

	expired, err := auth.GenerateTokenPair("admin", "test-secret", time.Hour, -time.Minute)
	if err != nil {
		t.Fatalf("tạo token hết hạn: %v", err)
	}
	wrongSecret, err := auth.GenerateTokenPair("admin", "secret-khac", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("tạo token sai secret: %v", err)
	}

	tests := []struct {
		name  string
		token string
		want  int
	}{
		{"token rác", "khong-phai-jwt", http.StatusUnauthorized},
		{"đã hết hạn", expired.RefreshToken, http.StatusUnauthorized},
		{"ký bằng secret khác", wrongSecret.RefreshToken, http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := postJSON(t, router, "/auth/refresh", `{"refresh_token":"`+tc.token+`"}`)
			if w.Code != tc.want {
				t.Fatalf("trả về %d, muốn %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestRefreshWithoutAnyTokenIsRejected(t *testing.T) {
	router := newAuthRouter(t, nil)

	w := postJSON(t, router, "/auth/refresh", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trả về %d, muốn %d", w.Code, http.StatusBadRequest)
	}
}

// Access token và refresh token ký cùng secret nên chữ ký hợp lệ chéo nhau;
// chỉ claim Issuer phân biệt được. Nếu endpoint này nhận access token thì
// TTL ngắn của access token thành vô nghĩa vì đổi được cặp token mới mãi.
func TestRefreshRejectsAccessToken(t *testing.T) {
	router := newAuthRouter(t, nil)

	pair, err := auth.GenerateTokenPair("admin", "test-secret", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("tạo token: %v", err)
	}

	w := postJSON(t, router, "/auth/refresh", `{"refresh_token":"`+pair.AccessToken+`"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("trả về %d, muốn %d", w.Code, http.StatusUnauthorized)
	}
}

// ── POST /auth/logout ─────────────────────────────────────────────────

func TestLogoutClearsBothCookies(t *testing.T) {
	router := newAuthRouter(t, nil)

	w := postJSON(t, router, "/auth/logout", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("logout trả về %d", w.Code)
	}

	for _, name := range []string{"file_session", "refresh_session"} {
		c := findCookie(t, w, name)
		if c.Value != "" {
			t.Errorf("cookie %s = %q, phải rỗng", name, c.Value)
		}
		if c.MaxAge >= 0 {
			t.Errorf("cookie %s có MaxAge = %d, phải âm để trình duyệt xoá", name, c.MaxAge)
		}
	}
}
