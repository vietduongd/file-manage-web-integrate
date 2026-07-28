package handlers

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/ckfindercompatible/backend/internal/models"
)

// Các test ở đây nhắm vào nhánh lỗi của tầng storage và các đường đi hiếm,
// những chỗ mà test đường-hạnh-phúc không chạm tới.

// ── nhánh lỗi storage ─────────────────────────────────────────────────

func TestDeleteFilesReportsStorageFailure(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))
	fake.Fail("LIST") // DeleteFiles liệt kê thumbnail trước khi xoá

	w := doJSON(t, router, http.MethodDelete, "/api/files",
		`{"type":"Images","path":"/","files":["a.png"]}`)
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestRenameFileReportsStorageFailure(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/cu.png", []byte("x"))
	fake.Fail("COPY")

	w := doJSON(t, router, http.MethodPatch, "/api/file/rename",
		`{"type":"Images","path":"/","name":"cu.png","newName":"moi.png"}`)
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestDeleteFolderReportsStorageFailure(t *testing.T) {
	fake, router := newFoldersRouter(t)
	fake.Fail("LIST")

	w := doJSON(t, router, http.MethodDelete, "/api/folder",
		`{"type":"Images","path":"/photos/"}`)
	assertStatus(t, w, http.StatusInternalServerError)
}

func TestRenameFolderReportsStorageFailure(t *testing.T) {
	fake, router := newFoldersRouter(t)
	fake.Fail("LIST")

	w := doJSON(t, router, http.MethodPatch, "/api/folder/rename",
		`{"type":"Images","path":"/photos/","newName":"moi"}`)
	assertStatus(t, w, http.StatusInternalServerError)
}

// MoveFiles và CopyFiles nuốt lỗi từng file rồi trả về số đếm,
// nên lỗi storage phải hiện ra ở con số 0 chứ không phải mã 500.
func TestMoveFilesCountsZeroWhenStorageFails(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))
	fake.Fail("COPY")

	w := doJSON(t, router, http.MethodPost, "/api/files/move",
		`{"files":[{"type":"Images","path":"/","name":"a.png"}],
		  "destination":{"type":"Images","path":"/dich/","name":""}}`)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Moved int `json:"moved"`
	}
	decodeJSON(t, w, &resp)
	if resp.Moved != 0 {
		t.Fatalf("moved = %d, muốn 0", resp.Moved)
	}
	if !fake.Has("images/a.png") {
		t.Error("move thất bại thì bản gốc phải còn nguyên")
	}
}

func TestCopyFilesCountsZeroWhenStorageFails(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))
	fake.Fail("COPY")

	w := doJSON(t, router, http.MethodPost, "/api/files/copy",
		`{"files":[{"type":"Images","path":"/","name":"a.png"}],
		  "destination":{"type":"Images","path":"/dich/","name":""}}`)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Copied int `json:"copied"`
	}
	decodeJSON(t, w, &resp)
	if resp.Copied != 0 {
		t.Fatalf("copied = %d, muốn 0", resp.Copied)
	}
}

func TestMoveFilesSkipsUnknownDestinationType(t *testing.T) {
	_, router := newFilesRouter(t)

	w := doJSON(t, router, http.MethodPost, "/api/files/move",
		`{"files":[{"type":"Images","path":"/","name":"a.png"}],
		  "destination":{"type":"Secrets","path":"/","name":""}}`)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Moved int `json:"moved"`
	}
	decodeJSON(t, w, &resp)
	if resp.Moved != 0 {
		t.Fatalf("moved = %d, muốn 0", resp.Moved)
	}
}

func TestCopyFilesSkipsUnknownSourceType(t *testing.T) {
	_, router := newFilesRouter(t)

	w := doJSON(t, router, http.MethodPost, "/api/files/copy",
		`{"files":[{"type":"Secrets","path":"/","name":"a.png"}],
		  "destination":{"type":"Images","path":"/","name":""}}`)
	assertStatus(t, w, http.StatusOK)
}

func TestThumbnailReportsGenerationFailure(t *testing.T) {
	fake, router := newMediaRouter(t)
	// Nội dung không phải ảnh hợp lệ: object tồn tại nên không phải 404,
	// nhưng giải mã ảnh sẽ hỏng.
	fake.Put("images/hong.png", []byte("day-khong-phai-anh-png"))

	w := doGet(t, router, "/api/thumbnail?type=Images&path=/&name=hong.png")
	assertStatus(t, w, http.StatusInternalServerError)
}

// ── upload: giới hạn kích thước ───────────────────────────────────────

func TestUploadFileRejectsOversizedFile(t *testing.T) {
	_, router := newFilesRouter(t)

	// MaxSizeMB trong testStorageConfig là 5MB.
	oversized := bytes.Repeat([]byte("x"), 6*1024*1024)
	w := uploadMultipart(t, router, "/api/upload", "file", "to.png", oversized,
		map[string]string{"type": "Images", "path": "/"})
	assertStatus(t, w, http.StatusBadRequest)

	if !strings.Contains(w.Body.String(), "too large") {
		t.Errorf("thông báo lỗi nên nói rõ file quá lớn, nhận %s", w.Body.String())
	}
}

// ── CKEditor upload: các nhánh còn lại ────────────────────────────────

func TestCKEditorUploadRejectsBadPath(t *testing.T) {
	_, router := newFilesRouter(t)

	w := uploadMultipart(t, router, "/api/upload/ck", "upload", "a.png", pngBytes(),
		map[string]string{"path": "../../"})
	assertStatus(t, w, http.StatusBadRequest)

	var resp models.CKEditorUploadResponse
	decodeJSON(t, w, &resp)
	if resp.Error == nil {
		t.Fatal("lỗi phải ở định dạng CKEditor")
	}
}

func TestCKEditorUploadRejectsMissingFile(t *testing.T) {
	_, router := newFilesRouter(t)

	w := uploadMultipart(t, router, "/api/upload/ck", "sai-field", "a.png", pngBytes(), nil)
	if w.Code == http.StatusOK {
		t.Fatal("thiếu field upload phải bị từ chối")
	}
}

func TestCKEditorUploadReportsStorageFailure(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Fail("PUT")

	w := uploadMultipart(t, router, "/api/upload/ck", "upload", "a.png", pngBytes(), nil)
	assertStatus(t, w, http.StatusInternalServerError)
}

// ── compress: các nhánh còn lại ───────────────────────────────────────

func TestCompressFilesAppendsZipExtension(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))

	w := doJSON(t, router, http.MethodPost, "/api/files/compress",
		`{"type":"Images","path":"/","files":["a.png"],"zipName":"khong-duoi"}`)
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/khong-duoi.zip") {
		t.Fatalf("phải tự thêm đuôi .zip, có: %v", fake.Keys())
	}
}

func TestCompressFilesSkipsMissingSourceFiles(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/co-that.png", []byte("a"))

	// File không tồn tại bị bỏ qua chứ không làm hỏng cả gói.
	w := doJSON(t, router, http.MethodPost, "/api/files/compress",
		`{"type":"Images","path":"/","files":["co-that.png","khong-co.png"],"zipName":"goi.zip"}`)
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/goi.zip") {
		t.Fatalf("vẫn phải tạo được zip, có: %v", fake.Keys())
	}
}

func TestCompressFilesRejectsPathInFileList(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))

	w := doJSON(t, router, http.MethodPost, "/api/files/compress",
		`{"type":"Images","path":"/","files":["../../secret.png"],"zipName":"goi.zip"}`)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCompressFilesReportsUploadFailure(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/a.png", []byte("a"))
	fake.Fail("PUT")

	w := doJSON(t, router, http.MethodPost, "/api/files/compress",
		`{"type":"Images","path":"/","files":["a.png"],"zipName":"goi.zip"}`)
	assertStatus(t, w, http.StatusInternalServerError)
}

// ── extract: các nhánh còn lại ────────────────────────────────────────

func TestExtractZipRejectsNonZipContent(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("files/gia.zip", []byte("day-khong-phai-zip"))

	w := doJSON(t, router, http.MethodPost, "/api/files/extract",
		`{"type":"Files","path":"/","fileName":"gia.zip"}`)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestExtractZipSkipsDirectoryEntries(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/goi.zip", makeZip(t, map[string][]byte{
		"thu-muc/":        nil,
		"thu-muc/anh.png": pngBytes(),
	}))

	w := doJSON(t, router, http.MethodPost, "/api/files/extract",
		`{"type":"Images","path":"/","fileName":"goi.zip"}`)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Count int `json:"count"`
	}
	decodeJSON(t, w, &resp)
	// Chỉ đếm file thật, không đếm entry thư mục.
	if resp.Count != 1 {
		t.Fatalf("count = %d, muốn 1", resp.Count)
	}
}

func TestExtractZipRejectsTraversalEntry(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/doc.zip", makeZip(t, map[string][]byte{
		"../../thoat.png": pngBytes(),
	}))

	w := doJSON(t, router, http.MethodPost, "/api/files/extract",
		`{"type":"Images","path":"/","fileName":"doc.zip"}`)
	assertStatus(t, w, http.StatusBadRequest)

	for _, k := range fake.Keys() {
		if !strings.HasPrefix(k, "images/") {
			t.Fatalf("zip-slip: object nằm ngoài prefix: %q", k)
		}
	}
}

func TestExtractZipSkipsDisallowedExtension(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/doc.zip", makeZip(t, map[string][]byte{
		"anh.png":     pngBytes(),
		"malware.exe": []byte("MZ"),
	}))

	w := doJSON(t, router, http.MethodPost, "/api/files/extract",
		`{"type":"Images","path":"/","fileName":"doc.zip"}`)
	assertStatus(t, w, http.StatusOK)

	if fake.Has("images/malware.exe") {
		t.Fatalf("file có đuôi không cho phép không được ghi ra, có: %v", fake.Keys())
	}
	if !fake.Has("images/anh.png") {
		t.Errorf("file hợp lệ vẫn phải được giải nén, có: %v", fake.Keys())
	}
}

func TestExtractZipContinuesWhenUploadFails(t *testing.T) {
	fake, router := newFilesRouter(t)
	fake.Put("images/doc.zip", makeZip(t, map[string][]byte{
		"anh.png": pngBytes(),
	}))
	fake.Fail("PUT")

	w := doJSON(t, router, http.MethodPost, "/api/files/extract",
		`{"type":"Images","path":"/","fileName":"doc.zip"}`)
	assertStatus(t, w, http.StatusOK)

	var resp struct {
		Count int `json:"count"`
	}
	decodeJSON(t, w, &resp)
	if resp.Count != 0 {
		t.Fatalf("count = %d, muốn 0 vì upload đều hỏng", resp.Count)
	}
}
