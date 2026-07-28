package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ckfindercompatible/backend/internal/config"
	minioclient "github.com/ckfindercompatible/backend/internal/minio"
	"github.com/ckfindercompatible/backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// testStorageConfig trả về config trỏ vào fake S3 đang chạy ở endpoint cho trước.
func testStorageConfig(endpoint string) *config.Config {
	return &config.Config{
		ServerEnv:          "test",
		MinioEndpoint:      endpoint,
		MinioAccessKey:     "test-access",
		MinioSecretKey:     "test-secret",
		MinioBucket:        "media",
		MinioUseSSL:        false,
		MinioPublicBaseURL: "http://cdn.example.test",
		MaxUploadSizeMB:    5,
		AllowedImageExts:   []string{"jpg", "jpeg", "png", "gif", "webp"},
		AllowedFileExts:    []string{"pdf", "txt", "zip"},
		AllowedVideoExts:   []string{"mp4", "webm"},
	}
}

// newStorageFixture dựng fake S3 cùng một minio client thật trỏ vào nó.
func newStorageFixture(t *testing.T) (*testutil.FakeS3, *minioclient.Client, *config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Không retry: các test nhánh lỗi cố tình trả 500, để mặc định thì
	// SDK thử lại kèm backoff làm suite chậm hàng giây mỗi test.
	t.Setenv("AWS_MAX_ATTEMPTS", "1")
	t.Setenv("AWS_RETRY_MODE", "standard")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	fake, endpoint := testutil.StartFakeS3(t)
	cfg := testStorageConfig(endpoint)

	mc, err := minioclient.New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("khởi tạo minio client: %v", err)
	}
	return fake, mc, cfg
}

// doJSON gửi request có body JSON và trả về recorder.
func doJSON(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// doGet gửi request GET không body.
func doGet(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// decodeJSON giải mã body của recorder, fail test nếu không phải JSON hợp lệ.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
		t.Fatalf("giải mã body %q: %v", w.Body.String(), err)
	}
}

// assertStatus kiểm mã trạng thái, in body ra khi lệch để dễ chẩn đoán.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, muốn %d: %s", w.Code, want, w.Body.String())
	}
}
