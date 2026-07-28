package handlers

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/ckfindercompatible/backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func newFilesRouter(t *testing.T) (*testutil.FakeS3, *gin.Engine) {
	t.Helper()
	fake, mc, cfg := newStorageFixture(t)
	h := NewFilesHandler(mc, cfg, zap.NewNop())

	r := gin.New()
	r.GET("/api/files", h.ListFiles)
	r.DELETE("/api/files", h.DeleteFiles)
	r.PATCH("/api/file/rename", h.RenameFile)
	r.POST("/api/files/move", h.MoveFiles)
	r.POST("/api/files/copy", h.CopyFiles)
	r.GET("/api/file/download", h.DownloadFile)
	r.POST("/api/upload", h.UploadFile)
	r.POST("/api/upload/ck", h.CKEditorUpload)
	r.POST("/api/files/compress", h.CompressFiles)
	r.POST("/api/files/extract", h.ExtractZip)
	return fake, r
}

// uploadMultipart dựng request multipart với một file đính kèm.
func uploadMultipart(t *testing.T, router *gin.Engine, path, fieldName, fileName string, content []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	part, err := mw.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("tạo form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("ghi nội dung file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("đóng multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// pngBytes trả về một PNG 1x1 hợp lệ để mimetype nhận đúng loại.
func pngBytes() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
}

// ── GET /api/files ────────────────────────────────────────────────────

func TestListFilesReturnsFilesWithThumbForImages(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/anh.png", pngBytes())
	fake.Put("images/tai-lieu.pdf", []byte("pdf"))

	w := doGet(t, router, "/api/files?type=Images&path=/")
	assertStatus(t, w, http.StatusOK)

	var resp models.FilesResponse
	decodeJSON(t, w, &resp)

	byName := map[string]models.FileInfo{}
	for _, f := range resp.Files {
		byName[f.Name] = f
	}
	if len(byName) != 2 {
		t.Fatalf("nhận %d file, muốn 2: %+v", len(byName), resp.Files)
	}
	if byName["anh.png"].Thumb == "" {
		t.Error("file ảnh phải có URL thumbnail")
	}
	if byName["tai-lieu.pdf"].Thumb != "" {
		t.Error("file không phải ảnh không được có thumbnail")
	}
	if byName["anh.png"].URL == "" {
		t.Error("thiếu URL công khai")
	}
}

func TestListFilesRejectsBadInput(t *testing.T) {
	_, router := newFilesRouter(t)

	for _, q := range []string{
		"/api/files?type=Images&path=../../etc",
		"/api/files?type=Secrets&path=/",
	} {
		w := doGet(t, router, q)
		assertStatus(t, w, http.StatusBadRequest)
	}
}

func TestListFilesReportsStorageFailure(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Fail("LIST")

	w := doGet(t, router, "/api/files?type=Images&path=/")
	assertStatus(t, w, http.StatusInternalServerError)
}

// ── DELETE /api/files ─────────────────────────────────────────────────

func TestDeleteFilesRemovesRequestedKeys(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))
	fake.Put("images/b.png", []byte("b"))

	w := doJSON(t, router, http.MethodDelete, "/api/files",
		`{"type":"Images","path":"/","files":["a.png"]}`)
	assertStatus(t, w, http.StatusOK)

	if fake.Has("images/a.png") {
		t.Error("a.png phải bị xoá")
	}
	if !fake.Has("images/b.png") {
		t.Error("b.png không nằm trong yêu cầu, không được xoá")
	}
}

func TestDeleteFilesRejectsBadInput(t *testing.T) {
	_, router := newFilesRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"type":`},
		{"thiếu trường", `{"type":"Images"}`},
		{"path vượt cấp", `{"type":"Images","path":"../","files":["a.png"]}`},
		{"resource type lạ", `{"type":"Secrets","path":"/","files":["a.png"]}`},
		{"tên file có đường dẫn", `{"type":"Images","path":"/","files":["../../secret.png"]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodDelete, "/api/files", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

// ── PATCH /api/file/rename ────────────────────────────────────────────

func TestRenameFileMovesObject(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/cu.png", []byte("x"))

	w := doJSON(t, router, http.MethodPatch, "/api/file/rename",
		`{"type":"Images","path":"/","name":"cu.png","newName":"moi.png"}`)
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/moi.png") {
		t.Fatalf("không thấy tên mới, có: %v", fake.Keys())
	}
	if fake.Has("images/cu.png") {
		t.Error("tên cũ phải bị xoá")
	}
}

// TestRenameFileWithoutExtensionIsRejected ghi nhận hành vi HIỆN TẠI.
// files.go:164 gán newName += oldExt để giữ đuôi file, nhưng dòng 168 lại
// kiểm rt.IsExtensionAllowed(newExt) với newExt rỗng ban đầu, nên nhánh
// "giữ đuôi" không bao giờ dùng được. Xem báo cáo kèm theo.
func TestRenameFileWithoutExtensionIsRejected(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/cu.png", []byte("x"))

	w := doJSON(t, router, http.MethodPatch, "/api/file/rename",
		`{"type":"Images","path":"/","name":"cu.png","newName":"moi"}`)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestRenameFileRejectsDisallowedExtension(t *testing.T) {
	_, router := newFilesRouter(t)

	w := doJSON(t, router, http.MethodPatch, "/api/file/rename",
		`{"type":"Images","path":"/","name":"a.png","newName":"b.exe"}`)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestRenameFileRejectsBadInput(t *testing.T) {
	_, router := newFilesRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"type":`},
		{"path vượt cấp", `{"type":"Images","path":"../","name":"a.png","newName":"b.png"}`},
		{"tên nguồn có đường dẫn", `{"type":"Images","path":"/","name":"../a.png","newName":"b.png"}`},
		{"tên đích có đường dẫn", `{"type":"Images","path":"/","name":"a.png","newName":"../b.png"}`},
		{"resource type lạ", `{"type":"Secrets","path":"/","name":"a.png","newName":"b.png"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodPatch, "/api/file/rename", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

// ── POST /api/files/move + copy ───────────────────────────────────────

func TestMoveFilesMovesBetweenFolders(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))

	w := doJSON(t, router, http.MethodPost, "/api/files/move",
		`{"files":[{"type":"Images","path":"/","name":"a.png"}],
		  "destination":{"type":"Images","path":"/dich/","name":""}}`)
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/dich/a.png") {
		t.Fatalf("không thấy file ở đích, có: %v", fake.Keys())
	}
	if fake.Has("images/a.png") {
		t.Error("move phải xoá bản gốc")
	}
}

func TestMoveFilesSkipsUnknownResourceType(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))

	// Resource type lạ thì bỏ qua file đó chứ không làm hỏng cả request.
	w := doJSON(t, router, http.MethodPost, "/api/files/move",
		`{"files":[{"type":"Secrets","path":"/","name":"a.png"}],
		  "destination":{"type":"Images","path":"/dich/","name":""}}`)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Moved int `json:"moved"`
	}
	decodeJSON(t, w, &resp)
	if resp.Moved != 0 {
		t.Fatalf("moved = %d, muốn 0", resp.Moved)
	}
}

func TestMoveFilesRejectsBadPaths(t *testing.T) {
	_, router := newFilesRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"files":`},
		{"path nguồn vượt cấp", `{"files":[{"type":"Images","path":"../","name":"a.png"}],"destination":{"type":"Images","path":"/","name":""}}`},
		{"path đích vượt cấp", `{"files":[{"type":"Images","path":"/","name":"a.png"}],"destination":{"type":"Images","path":"../","name":""}}`},
		{"tên file có đường dẫn", `{"files":[{"type":"Images","path":"/","name":"../a.png"}],"destination":{"type":"Images","path":"/","name":""}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodPost, "/api/files/move", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestCopyFilesKeepsOriginal(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))

	w := doJSON(t, router, http.MethodPost, "/api/files/copy",
		`{"files":[{"type":"Images","path":"/","name":"a.png"}],
		  "destination":{"type":"Images","path":"/dich/","name":""}}`)
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/dich/a.png") {
		t.Fatalf("không thấy bản sao, có: %v", fake.Keys())
	}
	if !fake.Has("images/a.png") {
		t.Error("copy phải giữ bản gốc")
	}
}

func TestCopyFilesRejectsBadPaths(t *testing.T) {
	_, router := newFilesRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"files":`},
		{"path nguồn vượt cấp", `{"files":[{"type":"Images","path":"../","name":"a.png"}],"destination":{"type":"Images","path":"/","name":""}}`},
		{"path đích vượt cấp", `{"files":[{"type":"Images","path":"/","name":"a.png"}],"destination":{"type":"Images","path":"../","name":""}}`},
		{"tên file có đường dẫn", `{"files":[{"type":"Images","path":"/","name":"a/b.png"}],"destination":{"type":"Images","path":"/","name":""}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodPost, "/api/files/copy", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestCopyFilesSkipsUnknownDestinationType(t *testing.T) {
	_, router := newFilesRouter(t)

	w := doJSON(t, router, http.MethodPost, "/api/files/copy",
		`{"files":[{"type":"Images","path":"/","name":"a.png"}],
		  "destination":{"type":"Secrets","path":"/","name":""}}`)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Copied int `json:"copied"`
	}
	decodeJSON(t, w, &resp)
	if resp.Copied != 0 {
		t.Fatalf("copied = %d, muốn 0", resp.Copied)
	}
}

// ── GET /api/file/download ────────────────────────────────────────────

func TestDownloadFileReturnsPresignedURL(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("files/tai-lieu.pdf", []byte("pdf"))

	w := doGet(t, router, "/api/file/download?type=Files&path=/&name=tai-lieu.pdf")
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		URL string `json:"url"`
	}
	decodeJSON(t, w, &resp)
	if resp.URL == "" {
		t.Fatal("thiếu URL tải về")
	}
	if !bytes.Contains([]byte(resp.URL), []byte("X-Amz-Signature")) {
		t.Errorf("URL phải được ký sẵn, nhận %q", resp.URL)
	}
}

func TestDownloadFileRejectsBadInput(t *testing.T) {
	_, router := newFilesRouter(t)

	tests := []struct{ name, query string }{
		{"path vượt cấp", "/api/file/download?type=Files&path=../&name=a.pdf"},
		{"tên có đường dẫn", "/api/file/download?type=Files&path=/&name=../a.pdf"},
		{"thiếu tên", "/api/file/download?type=Files&path=/&name="},
		{"resource type lạ", "/api/file/download?type=Secrets&path=/&name=a.pdf"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doGet(t, router, tc.query)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

// ── POST /api/upload ──────────────────────────────────────────────────

func TestUploadFileStoresObject(t *testing.T) {
	fake, router := newFilesRouter(t)

	w := uploadMultipart(t, router, "/api/upload", "file", "anh.png", pngBytes(),
		map[string]string{"type": "Images", "path": "/"})
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/anh.png") {
		t.Fatalf("không thấy object đã upload, có: %v", fake.Keys())
	}

	var resp models.UploadResponse
	decodeJSON(t, w, &resp)
	if resp.Uploaded != 1 || resp.FileName != "anh.png" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestUploadFileRejectsDisallowedExtension(t *testing.T) {
	_, router := newFilesRouter(t)

	w := uploadMultipart(t, router, "/api/upload", "file", "malware.exe", []byte("MZ"),
		map[string]string{"type": "Images", "path": "/"})
	assertStatus(t, w, http.StatusBadRequest)
}

// Tên file trong multipart không thoát được ra ngoài thư mục đích.
// mime/multipart của Go đã áp filepath.Base() cho Part.FileName(), nên handler
// nhận "thoat.png" chứ không phải "../../thoat.png" — request thành công
// nhưng object vẫn nằm đúng chỗ. Test này khoá lại lớp bảo vệ đó.
func TestUploadFileNeutralisesPathInFileName(t *testing.T) {
	fake, router := newFilesRouter(t)

	w := uploadMultipart(t, router, "/api/upload", "file", "../../thoat.png", pngBytes(),
		map[string]string{"type": "Images", "path": "/"})
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/thoat.png") {
		t.Fatalf("object phải nằm trong images/, có: %v", fake.Keys())
	}
	for _, k := range fake.Keys() {
		if !strings.HasPrefix(k, "images/") {
			t.Fatalf("object thoát ra ngoài prefix: %q", k)
		}
	}
}

func TestUploadFileRejectsBadFormFields(t *testing.T) {
	_, router := newFilesRouter(t)

	t.Run("path vượt cấp", func(t *testing.T) {
		w := uploadMultipart(t, router, "/api/upload", "file", "a.png", pngBytes(),
			map[string]string{"type": "Images", "path": "../../"})
		assertStatus(t, w, http.StatusBadRequest)
	})
	t.Run("resource type lạ", func(t *testing.T) {
		w := uploadMultipart(t, router, "/api/upload", "file", "a.png", pngBytes(),
			map[string]string{"type": "Secrets", "path": "/"})
		assertStatus(t, w, http.StatusBadRequest)
	})
	t.Run("sai tên field", func(t *testing.T) {
		w := uploadMultipart(t, router, "/api/upload", "khong-phai-file", "a.png", pngBytes(),
			map[string]string{"type": "Images", "path": "/"})
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestUploadFileReportsStorageFailure(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Fail("PUT")

	w := uploadMultipart(t, router, "/api/upload", "file", "anh.png", pngBytes(),
		map[string]string{"type": "Images", "path": "/"})
	assertStatus(t, w, http.StatusInternalServerError)
}

// ── POST /api/upload/ck ───────────────────────────────────────────────

func TestCKEditorUploadUsesUploadField(t *testing.T) {
	fake, router := newFilesRouter(t)

	w := uploadMultipart(t, router, "/api/upload/ck", "upload", "anh.png", pngBytes(), nil)
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/anh.png") {
		t.Fatalf("không thấy object, có: %v", fake.Keys())
	}

	var resp models.CKEditorUploadResponse
	decodeJSON(t, w, &resp)
	if resp.Uploaded != 1 || resp.URL == "" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestCKEditorUploadFallsBackToImagesForUnknownType(t *testing.T) {
	fake, router := newFilesRouter(t)

	w := uploadMultipart(t, router, "/api/upload/ck", "upload", "anh.png", pngBytes(),
		map[string]string{"type": "Secrets"})
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/anh.png") {
		t.Fatalf("phải rơi về Images, có: %v", fake.Keys())
	}
}

func TestCKEditorUploadReportsErrorInCKFormat(t *testing.T) {
	_, router := newFilesRouter(t)

	w := uploadMultipart(t, router, "/api/upload/ck", "upload", "malware.exe", []byte("MZ"), nil)
	if w.Code == http.StatusOK {
		t.Fatal("phần mở rộng không hợp lệ phải bị từ chối")
	}

	var resp models.CKEditorUploadResponse
	decodeJSON(t, w, &resp)
	if resp.Uploaded != 0 || resp.Error == nil || resp.Error.Message == "" {
		t.Fatalf("CKEditor cần lỗi ở dạng {uploaded:0, error:{message}}, nhận %+v", resp)
	}
}

// ── POST /api/files/compress ──────────────────────────────────────────

func TestCompressFilesCreatesZip(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("noi-dung-a"))
	fake.Put("images/b.png", []byte("noi-dung-b"))

	w := doJSON(t, router, http.MethodPost, "/api/files/compress",
		`{"type":"Images","path":"/","files":["a.png","b.png"],"zipName":"goi.zip"}`)
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/goi.zip") {
		t.Fatalf("không thấy file zip, có: %v", fake.Keys())
	}
}

func TestCompressFilesRejectsBadInput(t *testing.T) {
	_, router := newFilesRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"type":`},
		{"thiếu trường", `{"type":"Images","path":"/"}`},
		{"path vượt cấp", `{"type":"Images","path":"../","files":["a.png"],"zipName":"g.zip"}`},
		{"zipName có đường dẫn", `{"type":"Images","path":"/","files":["a.png"],"zipName":"../g.zip"}`},
		{"resource type lạ", `{"type":"Secrets","path":"/","files":["a.png"],"zipName":"g.zip"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodPost, "/api/files/compress", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

// ── POST /api/files/extract ───────────────────────────────────────────

func makeZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("tạo entry %q: %v", name, err)
		}
		if _, err := f.Write(content); err != nil {
			t.Fatalf("ghi entry %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("đóng zip: %v", err)
	}
	return buf.Bytes()
}

func TestExtractZipWritesEntries(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("files/goi.zip", makeZip(t, map[string][]byte{
		"tai-lieu.txt": []byte("noi dung"),
	}))

	w := doJSON(t, router, http.MethodPost, "/api/files/extract",
		`{"type":"Files","path":"/","fileName":"goi.zip"}`)
	assertStatus(t, w, http.StatusOK)

	// Kiểm cả object lẫn số đếm: chỉ xem status 200 thì không phát hiện được
	// trường hợp handler chạy xong mà không ghi ra file nào.
	if !fake.Has("files/tai-lieu.txt") {
		t.Fatalf("không thấy file đã giải nén, có: %v", fake.Keys())
	}
	var resp struct {
		Count int `json:"count"`
	}
	decodeJSON(t, w, &resp)
	if resp.Count != 1 {
		t.Fatalf("count = %d, muốn 1", resp.Count)
	}
}

func TestExtractZipRejectsMissingObject(t *testing.T) {
	_, router := newFilesRouter(t)

	w := doJSON(t, router, http.MethodPost, "/api/files/extract",
		`{"type":"Files","path":"/","fileName":"khong-ton-tai.zip"}`)
	if w.Code == http.StatusOK {
		t.Fatalf("zip không tồn tại phải báo lỗi, nhận %d", w.Code)
	}
}

func TestExtractZipRejectsBadInput(t *testing.T) {
	_, router := newFilesRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"type":`},
		{"thiếu trường", `{"type":"Files"}`},
		{"path vượt cấp", `{"type":"Files","path":"../","fileName":"g.zip"}`},
		{"tên có đường dẫn", `{"type":"Files","path":"/","fileName":"../g.zip"}`},
		{"resource type lạ", `{"type":"Secrets","path":"/","fileName":"g.zip"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodPost, "/api/files/extract", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}
