package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/ckfindercompatible/backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ── media.go ──────────────────────────────────────────────────────────

func newMediaRouter(t *testing.T) (*testutil.FakeS3, *gin.Engine) {
	t.Helper()
	fake, mc, cfg := newStorageFixture(t)
	h := NewMediaHandler(mc, cfg, zap.NewNop())

	r := gin.New()
	r.GET("/api/thumbnail", h.Thumbnail)
	r.GET("/api/preview", h.Preview)
	return fake, r
}

func TestParseIntDefault(t *testing.T) {
	tests := []struct {
		in   string
		def  int
		want int
	}{
		{"", 150, 150},
		{"300", 150, 300},
		{"khong-phai-so", 150, 150},
		{"0", 150, 150},   // 0 vô nghĩa với kích thước ảnh
		{"-10", 150, 150}, // số âm cũng vậy
		{"1", 150, 1},
	}
	for _, tc := range tests {
		if got := parseIntDefault(tc.in, tc.def); got != tc.want {
			t.Errorf("parseIntDefault(%q, %d) = %d, muốn %d", tc.in, tc.def, got, tc.want)
		}
	}
}

func TestThumbnailGeneratesImage(t *testing.T) {
	fake, router := newMediaRouter(t)
	fake.Put("images/anh.png", pngBytes())

	w := doGet(t, router, "/api/thumbnail?type=Images&path=/&name=anh.png")
	assertStatus(t, w, http.StatusOK)

	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Errorf("Content-Type = %q, muốn ảnh", ct)
	}
	if w.Header().Get("Cache-Control") == "" {
		t.Error("thumbnail nên đặt Cache-Control vì nó bất biến theo key")
	}
	if w.Body.Len() == 0 {
		t.Error("body rỗng")
	}
}

func TestPreviewGeneratesImage(t *testing.T) {
	fake, router := newMediaRouter(t)
	fake.Put("images/anh.png", pngBytes())

	w := doGet(t, router, "/api/preview?type=Images&path=/&name=anh.png&w=400")
	assertStatus(t, w, http.StatusOK)
}

func TestThumbnailReturns404WhenOriginalMissing(t *testing.T) {
	_, router := newMediaRouter(t)

	w := doGet(t, router, "/api/thumbnail?type=Images&path=/&name=khong-co.png")
	assertStatus(t, w, http.StatusNotFound)
}

func TestThumbnailRedirectsForNonImage(t *testing.T) {
	fake, router := newMediaRouter(t)
	fake.Put("files/tai-lieu.pdf", []byte("pdf"))

	w := doGet(t, router, "/api/thumbnail?type=Files&path=/&name=tai-lieu.pdf")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, muốn %d (redirect sang URL gốc)", w.Code, http.StatusFound)
	}
	if loc := w.Header().Get("Location"); loc == "" {
		t.Error("thiếu header Location")
	}
}

func TestThumbnailRejectsBadInput(t *testing.T) {
	_, router := newMediaRouter(t)

	tests := []struct{ name, query string }{
		{"path vượt cấp", "/api/thumbnail?type=Images&path=../&name=a.png"},
		{"tên có đường dẫn", "/api/thumbnail?type=Images&path=/&name=../a.png"},
		{"thiếu tên", "/api/thumbnail?type=Images&path=/&name="},
		{"resource type lạ", "/api/thumbnail?type=Secrets&path=/&name=a.png"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doGet(t, router, tc.query)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

// ── stats.go ──────────────────────────────────────────────────────────

func newStatsRouter(t *testing.T) (*testutil.FakeS3, *gin.Engine) {
	t.Helper()
	fake, mc, cfg := newStorageFixture(t)
	h := NewStatsHandler(mc, cfg, zap.NewNop())

	r := gin.New()
	r.GET("/api/stats", h.GetStats)
	return fake, r
}

func TestGetStatsCountsPerResourceType(t *testing.T) {
	fake, router := newStatsRouter(t)
	fake.Put("images/a.png", []byte("12345"))
	fake.Put("images/sub/b.png", []byte("123"))
	fake.Put("files/c.pdf", []byte("1234567890"))

	w := doGet(t, router, "/api/stats")
	assertStatus(t, w, http.StatusOK)

	var resp StatsResponse
	decodeJSON(t, w, &resp)

	if got := resp.Breakdown["Images"]; got.Count != 2 || got.Size != 8 {
		t.Errorf("Images = %+v, muốn count 2 size 8", got)
	}
	if got := resp.Breakdown["Files"]; got.Count != 1 || got.Size != 10 {
		t.Errorf("Files = %+v, muốn count 1 size 10", got)
	}
	if got := resp.Breakdown["Videos"]; got.Count != 0 {
		t.Errorf("Videos = %+v, muốn count 0", got)
	}
	if resp.TotalCount != 3 || resp.TotalSize != 18 {
		t.Errorf("tổng = %d file / %d byte, muốn 3 / 18", resp.TotalCount, resp.TotalSize)
	}
}

func TestGetStatsIgnoresFolderPlaceholders(t *testing.T) {
	fake, router := newStatsRouter(t)
	fake.Put("images/thu-muc-rong/.keep", []byte(""))
	fake.Put("images/a.png", []byte("12345"))

	w := doGet(t, router, "/api/stats")
	assertStatus(t, w, http.StatusOK)

	var resp StatsResponse
	decodeJSON(t, w, &resp)

	// .keep chỉ để đánh dấu thư mục, không phải file người dùng.
	if resp.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, muốn 1 (bỏ qua .keep)", resp.TotalCount)
	}
}

func TestGetStatsReportsStorageFailure(t *testing.T) {
	fake, router := newStatsRouter(t)
	fake.Fail("LIST")

	w := doGet(t, router, "/api/stats")
	assertStatus(t, w, http.StatusInternalServerError)
}

// ── config.go ─────────────────────────────────────────────────────────

func TestGetConfigListsAllResourceTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testStorageConfig("localhost:9000")
	h := NewConfigHandler(cfg)

	r := gin.New()
	r.GET("/api/config", h.GetConfig)

	w := doGet(t, r, "/api/config")
	assertStatus(t, w, http.StatusOK)

	var resp models.ConfigResponse
	decodeJSON(t, w, &resp)

	if len(resp.ResourceTypes) != 3 {
		t.Fatalf("nhận %d resource type, muốn 3", len(resp.ResourceTypes))
	}
	if resp.MaxUploadMB != cfg.MaxUploadSizeMB {
		t.Errorf("MaxUploadMB = %d, muốn %d", resp.MaxUploadMB, cfg.MaxUploadSizeMB)
	}

	byName := map[string]models.ResourceTypeInfo{}
	for _, rt := range resp.ResourceTypes {
		byName[rt.Name] = rt
	}
	if !byName["Images"].PublicRead {
		t.Error("Images phải là public-read")
	}
	if byName["Files"].PublicRead {
		t.Error("Files phải là private")
	}
	if want := "http://cdn.example.test/media/images/"; byName["Images"].URL != want {
		t.Errorf("URL = %q, muốn %q", byName["Images"].URL, want)
	}
	if len(byName["Images"].AllowedExtensions) == 0 {
		t.Error("thiếu danh sách phần mở rộng cho phép")
	}
}

// ── auth_errors.go ────────────────────────────────────────────────────

func TestExternalAuthErrorMessage(t *testing.T) {
	err := externalAuthError{status: http.StatusForbidden, message: "bị từ chối"}
	if err.Error() != "bị từ chối" {
		t.Fatalf("Error() = %q, muốn %q", err.Error(), "bị từ chối")
	}
}
