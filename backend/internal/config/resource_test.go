package config

import (
	"testing"
	"time"
)

func TestGetPanicsBeforeLoad(t *testing.T) {
	original := cfg
	cfg = nil
	defer func() {
		cfg = original
		if recover() == nil {
			t.Fatal("Get() phải panic khi chưa Load()")
		}
	}()
	Get()
}

func TestGetReturnsLoadedConfig(t *testing.T) {
	loaded := Load()
	if Get() != loaded {
		t.Fatal("Get() phải trả về đúng config vừa Load()")
	}
}

func TestResourceTypesCoverThreeKinds(t *testing.T) {
	c := &Config{
		AllowedImageExts: []string{"png"},
		AllowedFileExts:  []string{"pdf"},
		AllowedVideoExts: []string{"mp4"},
		MaxUploadSizeMB:  10,
	}

	rts := c.ResourceTypes()
	if len(rts) != 3 {
		t.Fatalf("nhận %d resource type, muốn 3", len(rts))
	}

	byName := map[string]ResourceTypeConfig{}
	for _, rt := range rts {
		byName[rt.Name] = rt
	}

	// Images và Videos phục vụ trực tiếp qua CDN nên public;
	// Files có thể chứa tài liệu nội bộ nên phải private.
	if !byName["Images"].PublicRead {
		t.Error("Images phải public-read")
	}
	if !byName["Videos"].PublicRead {
		t.Error("Videos phải public-read")
	}
	if byName["Files"].PublicRead {
		t.Error("Files phải private")
	}

	if byName["Images"].Prefix != "images" {
		t.Errorf("prefix Images = %q", byName["Images"].Prefix)
	}
	if byName["Images"].MaxSizeMB != 10 {
		t.Errorf("MaxSizeMB = %d, muốn 10", byName["Images"].MaxSizeMB)
	}
}

func TestGetResourceTypeIsCaseInsensitive(t *testing.T) {
	c := &Config{AllowedImageExts: []string{"png"}}

	for _, name := range []string{"Images", "images", "IMAGES", "ImAgEs"} {
		rt, err := c.GetResourceType(name)
		if err != nil {
			t.Fatalf("GetResourceType(%q): %v", name, err)
		}
		if rt.Name != "Images" {
			t.Errorf("GetResourceType(%q).Name = %q", name, rt.Name)
		}
	}
}

func TestGetResourceTypeRejectsUnknown(t *testing.T) {
	c := &Config{}

	for _, name := range []string{"Secrets", "", "image"} {
		if _, err := c.GetResourceType(name); err == nil {
			t.Errorf("GetResourceType(%q) phải trả lỗi", name)
		}
	}
}

func TestIsExtensionAllowed(t *testing.T) {
	rt := &ResourceTypeConfig{AllowedExtensions: []string{"jpg", "png"}}

	tests := []struct {
		ext  string
		want bool
	}{
		{"png", true},
		{".png", true}, // chấp nhận cả dạng có dấu chấm
		{".PNG", true}, // không phân biệt hoa thường
		{"JPG", true},
		{"exe", false},
		{".exe", false},
		{"", false},
		{".", false},
	}
	for _, tc := range tests {
		if got := rt.IsExtensionAllowed(tc.ext); got != tc.want {
			t.Errorf("IsExtensionAllowed(%q) = %v, muốn %v", tc.ext, got, tc.want)
		}
	}
}

func TestValidateAllowsNonProduction(t *testing.T) {
	// Ngoài production thì không ràng buộc gì, để dev chạy được ngay.
	c := &Config{ServerEnv: "development", JWTSecret: "yeu", AdminPassword: "admin123"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() ở development trả lỗi: %v", err)
	}
}

func TestValidateProductionChecks(t *testing.T) {
	strongSecret := "mot-secret-that-dai-va-manh-cho-production"

	valid := func() *Config {
		return &Config{
			ServerEnv:      "production",
			JWTSecret:      strongSecret,
			AdminPassword:  "mat-khau-that",
			AllowedOrigins: []string{"https://file-manager.netecs.vn"},
		}
	}

	if err := valid().Validate(); err != nil {
		t.Fatalf("config production hợp lệ lại bị từ chối: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"JWT_SECRET rỗng", func(c *Config) { c.JWTSecret = "" }},
		{"JWT_SECRET mặc định", func(c *Config) { c.JWTSecret = "change-me-in-production" }},
		{"JWT_SECRET mẫu cũ", func(c *Config) { c.JWTSecret = "your-super-secret-jwt-key-change-in-production" }},
		{"JWT_SECRET quá ngắn", func(c *Config) { c.JWTSecret = "ngan" }},
		{"ADMIN_PASSWORD rỗng", func(c *Config) { c.AdminPassword = "" }},
		{"ADMIN_PASSWORD mặc định", func(c *Config) { c.AdminPassword = "admin123" }},
		{"không có origin nào", func(c *Config) { c.AllowedOrigins = nil }},
		{"origin là localhost", func(c *Config) { c.AllowedOrigins = []string{"http://localhost:3000"} }},
		{"origin là 127.0.0.1", func(c *Config) { c.AllowedOrigins = []string{"http://127.0.0.1:3000"} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mutate(c)
			if err := c.Validate(); err == nil {
				t.Fatal("Validate() phải trả lỗi")
			}
		})
	}
}

func TestLoadValidatedReturnsConfigWhenValid(t *testing.T) {
	t.Setenv("SERVER_ENV", "production")
	t.Setenv("JWT_SECRET", "mot-secret-that-dai-va-manh-cho-production")
	t.Setenv("ADMIN_PASSWORD", "mat-khau-that")
	t.Setenv("FRONTEND_URL", "https://file-manager.netecs.vn")

	c, err := LoadValidated()
	if err != nil {
		t.Fatalf("LoadValidated: %v", err)
	}
	if c.ServerEnv != "production" {
		t.Fatalf("ServerEnv = %q", c.ServerEnv)
	}
}

func TestGetEnvHelpersFallBackOnBadValues(t *testing.T) {
	t.Setenv("TEST_BOOL_HONG", "khong-phai-bool")
	t.Setenv("TEST_BOOL_DUNG", "true")
	t.Setenv("TEST_INT_HONG", "khong-phai-so")
	t.Setenv("TEST_INT_DUNG", "42")

	if got := getEnvBool("TEST_BOOL_HONG", true); got != true {
		t.Errorf("getEnvBool giá trị hỏng = %v, muốn giữ mặc định true", got)
	}
	if got := getEnvBool("TEST_BOOL_DUNG", false); got != true {
		t.Errorf("getEnvBool = %v, muốn true", got)
	}
	if got := getEnvBool("TEST_BOOL_KHONG_CO", true); got != true {
		t.Errorf("getEnvBool biến không tồn tại = %v, muốn mặc định", got)
	}

	if got := getEnvInt("TEST_INT_HONG", 7); got != 7 {
		t.Errorf("getEnvInt giá trị hỏng = %d, muốn giữ mặc định 7", got)
	}
	if got := getEnvInt("TEST_INT_DUNG", 7); got != 42 {
		t.Errorf("getEnvInt = %d, muốn 42", got)
	}
	if got := getEnvInt("TEST_INT_KHONG_CO", 7); got != 7 {
		t.Errorf("getEnvInt biến không tồn tại = %d, muốn mặc định", got)
	}
}

func TestLoadReadsRateLimitSettings(t *testing.T) {
	t.Setenv("LOGIN_RATE_LIMIT_MAX", "9")
	t.Setenv("LOGIN_RATE_LIMIT_WINDOW_SECONDS", "120")
	t.Setenv("LOGIN_RATE_LIMIT_DISABLED", "true")

	c := Load()

	if c.LoginRateLimitMax != 9 {
		t.Errorf("LoginRateLimitMax = %d, muốn 9", c.LoginRateLimitMax)
	}
	if c.LoginRateLimitWindow != 2*time.Minute {
		t.Errorf("LoginRateLimitWindow = %v, muốn 2m", c.LoginRateLimitWindow)
	}
	if !c.LoginRateLimitDisabled {
		t.Error("LoginRateLimitDisabled phải true")
	}
}

func TestSplitEnvTrimsAndDropsEmpty(t *testing.T) {
	t.Setenv("ALLOWED_IMAGE_EXTS", " jpg , png ,, webp ,")

	c := Load()

	want := []string{"jpg", "png", "webp"}
	if len(c.AllowedImageExts) != len(want) {
		t.Fatalf("AllowedImageExts = %v, muốn %v", c.AllowedImageExts, want)
	}
	for i, w := range want {
		if c.AllowedImageExts[i] != w {
			t.Errorf("AllowedImageExts[%d] = %q, muốn %q", i, c.AllowedImageExts[i], w)
		}
	}
}
