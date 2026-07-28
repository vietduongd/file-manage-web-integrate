package config

import "testing"

func TestAllowedOriginsParsesCommaSeparatedFrontendURL(t *testing.T) {
	t.Setenv("FRONTEND_URL", "https://file-manager.netecs.vn, https://admin.netecs.vn/ ,")

	cfg := Load()

	want := []string{"https://file-manager.netecs.vn", "https://admin.netecs.vn"}
	if len(cfg.AllowedOrigins) != len(want) {
		t.Fatalf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for i, w := range want {
		if cfg.AllowedOrigins[i] != w {
			t.Fatalf("AllowedOrigins[%d] = %q, want %q", i, cfg.AllowedOrigins[i], w)
		}
	}
}

func TestIsOriginAllowedProduction(t *testing.T) {
	cfg := &Config{
		ServerEnv:      "production",
		AllowedOrigins: []string{"https://file-manager.netecs.vn", "https://admin.netecs.vn"},
	}

	tests := []struct {
		origin string
		want   bool
	}{
		{"https://file-manager.netecs.vn", true},
		{"https://file-manager.netecs.vn/", true},  // trailing slash từ browser cũ
		{"HTTPS://File-Manager.netecs.vn", true},   // origin không phân biệt hoa thường
		{"https://admin.netecs.vn", true},          // origin thứ hai (iframe embed)
		{"https://evil.netecs.vn", false},
		{"https://file-manager.netecs.vn.evil.com", false},
		{"http://file-manager.netecs.vn", false},   // sai scheme
		{"http://localhost:3000", false},           // localhost bị chặn ở production
		{"", false},
	}

	for _, tc := range tests {
		if got := cfg.IsOriginAllowed(tc.origin); got != tc.want {
			t.Errorf("IsOriginAllowed(%q) = %v, want %v", tc.origin, got, tc.want)
		}
	}
}

func TestIsOriginAllowedDevelopmentAlsoHonoursAllowedOrigins(t *testing.T) {
	cfg := &Config{
		ServerEnv:      "development",
		AllowedOrigins: []string{"https://file-manager.netecs.vn"},
	}

	tests := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:3000", true},
		{"http://127.0.0.1:5173", true},
		{"https://localhost:3000", true},
		{"https://file-manager.netecs.vn", true}, // vẫn tôn trọng FRONTEND_URL
		{"https://evil.com", false},
	}

	for _, tc := range tests {
		if got := cfg.IsOriginAllowed(tc.origin); got != tc.want {
			t.Errorf("IsOriginAllowed(%q) = %v, want %v", tc.origin, got, tc.want)
		}
	}
}

func TestProductionRequiresExplicitAllowedOrigins(t *testing.T) {
	t.Setenv("SERVER_ENV", "production")
	t.Setenv("JWT_SECRET", "a-strong-production-secret-with-32-chars")
	t.Setenv("ADMIN_PASSWORD", "not-the-default")
	t.Setenv("FRONTEND_URL", "http://localhost:3000")

	if _, err := LoadValidated(); err == nil {
		t.Fatal("LoadValidated succeeded with default localhost FRONTEND_URL in production, want error")
	}
}
