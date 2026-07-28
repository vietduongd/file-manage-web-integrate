package handlers

import (
	"net/http"
	"testing"

	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/ckfindercompatible/backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func newFoldersRouter(t *testing.T) (*testutil.FakeS3, *gin.Engine) {
	t.Helper()
	fake, mc, cfg := newStorageFixture(t)
	h := NewFoldersHandler(mc, cfg, zap.NewNop())

	r := gin.New()
	r.GET("/api/folders", h.ListFolders)
	r.POST("/api/folder", h.CreateFolder)
	r.DELETE("/api/folder", h.DeleteFolder)
	r.PATCH("/api/folder/rename", h.RenameFolder)
	return fake, r
}

// ── helper thuần ──────────────────────────────────────────────────────

func TestCleanPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "/"},
		{"/", "/"},
		{"photos", "/photos/"},
		{"/photos", "/photos/"},
		{"/photos/", "/photos/"},
		{"photos/2026", "/photos/2026/"},
	}
	for _, tc := range tests {
		if got := cleanPath(tc.in); got != tc.want {
			t.Errorf("cleanPath(%q) = %q, muốn %q", tc.in, got, tc.want)
		}
	}
}

func TestIsValidName(t *testing.T) {
	valid := []string{"photos", "anh-2026", "Thư mục", "a.b", "file name"}
	for _, name := range valid {
		if !isValidName(name) {
			t.Errorf("isValidName(%q) = false, muốn true", name)
		}
	}

	// Ký tự phân tách đường dẫn và ký tự cấm của Windows đều phải chặn,
	// nếu không sẽ tạo được object nằm ngoài thư mục đích.
	invalid := []string{"", ".", "..", "a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "a>b", "a|b"}
	for _, name := range invalid {
		if isValidName(name) {
			t.Errorf("isValidName(%q) = true, muốn false", name)
		}
	}
}

func TestErrorResp(t *testing.T) {
	got := errorResp(404, "không thấy")
	if got.Error.Code != 404 || got.Error.Message != "không thấy" {
		t.Fatalf("errorResp = %+v", got)
	}
}

// ── GET /api/folders ──────────────────────────────────────────────────

func TestListFoldersReturnsChildFolders(t *testing.T) {
	fake, router := newFoldersRouter(t)
	fake.Put("images/photos/.keep", nil)
	fake.Put("images/photos/2026/.keep", nil)
	fake.Put("images/videos-cu/.keep", nil)

	w := doGet(t, router, "/api/folders?type=Images&path=/")
	assertStatus(t, w, http.StatusOK)

	var resp models.FoldersResponse
	decodeJSON(t, w, &resp)

	if resp.ResourceType != "Images" {
		t.Fatalf("ResourceType = %q", resp.ResourceType)
	}
	names := map[string]models.FolderInfo{}
	for _, f := range resp.Folders {
		names[f.Name] = f
	}
	if len(names) != 2 {
		t.Fatalf("nhận %d thư mục, muốn 2: %+v", len(names), resp.Folders)
	}
	if got := names["photos"]; !got.HasChildren {
		t.Error("photos có thư mục con 2026 nên HasChildren phải true")
	}
	if got := names["photos"]; got.Path != "/photos/" {
		t.Errorf("Path = %q, muốn %q", got.Path, "/photos/")
	}
	if got := names["videos-cu"]; got.HasChildren {
		t.Error("videos-cu không có con nên HasChildren phải false")
	}
}

func TestListFoldersEmptyBucketReturnsEmptyArray(t *testing.T) {
	_, router := newFoldersRouter(t)

	w := doGet(t, router, "/api/folders?type=Images&path=/")
	assertStatus(t, w, http.StatusOK)

	var resp models.FoldersResponse
	decodeJSON(t, w, &resp)
	if resp.Folders == nil {
		t.Fatal("Folders phải là mảng rỗng chứ không phải null")
	}
	if len(resp.Folders) != 0 {
		t.Fatalf("nhận %d thư mục, muốn 0", len(resp.Folders))
	}
}

func TestListFoldersRejectsBadInput(t *testing.T) {
	_, router := newFoldersRouter(t)

	tests := []struct{ name, query string }{
		{"path vượt cấp", "/api/folders?type=Images&path=../../etc"},
		{"resource type lạ", "/api/folders?type=Secrets&path=/"},
		{"thiếu resource type", "/api/folders?path=/"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doGet(t, router, tc.query)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestListFoldersReportsStorageFailure(t *testing.T) {
	fake, router := newFoldersRouter(t)
	fake.Fail("LIST")

	w := doGet(t, router, "/api/folders?type=Images&path=/")
	assertStatus(t, w, http.StatusInternalServerError)
}

// ── POST /api/folder ──────────────────────────────────────────────────

func TestCreateFolderWritesPlaceholderObject(t *testing.T) {
	fake, router := newFoldersRouter(t)

	w := doJSON(t, router, http.MethodPost, "/api/folder",
		`{"type":"Images","path":"/","name":"anh-moi"}`)
	assertStatus(t, w, http.StatusOK)

	found := false
	for _, k := range fake.Keys() {
		if len(k) >= len("images/anh-moi/") && k[:len("images/anh-moi/")] == "images/anh-moi/" {
			found = true
		}
	}
	if !found {
		t.Fatalf("không thấy object nào dưới images/anh-moi/, có: %v", fake.Keys())
	}
}

func TestCreateFolderRejectsBadInput(t *testing.T) {
	_, router := newFoldersRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"type":`},
		{"thiếu trường bắt buộc", `{}`},
		{"path vượt cấp", `{"type":"Images","path":"../../","name":"x"}`},
		{"resource type lạ", `{"type":"Secrets","path":"/","name":"x"}`},
		{"tên có dấu gạch chéo", `{"type":"Images","path":"/","name":"a/b"}`},
		{"tên là hai chấm", `{"type":"Images","path":"/","name":".."}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodPost, "/api/folder", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestCreateFolderReportsStorageFailure(t *testing.T) {
	fake, router := newFoldersRouter(t)
	fake.Fail("PUT")

	w := doJSON(t, router, http.MethodPost, "/api/folder",
		`{"type":"Images","path":"/","name":"anh-moi"}`)
	assertStatus(t, w, http.StatusInternalServerError)
}

// ── DELETE /api/folder ────────────────────────────────────────────────

func TestDeleteFolderRemovesEverythingUnderPrefix(t *testing.T) {
	fake, router := newFoldersRouter(t)
	fake.Put("images/photos/a.jpg", []byte("a"))
	fake.Put("images/photos/sub/b.jpg", []byte("b"))
	fake.Put("images/khac/c.jpg", []byte("c"))

	w := doJSON(t, router, http.MethodDelete, "/api/folder",
		`{"type":"Images","path":"/photos/"}`)
	assertStatus(t, w, http.StatusOK)

	if fake.Has("images/photos/a.jpg") || fake.Has("images/photos/sub/b.jpg") {
		t.Fatalf("còn sót object trong thư mục đã xoá: %v", fake.Keys())
	}
	if !fake.Has("images/khac/c.jpg") {
		t.Fatal("thư mục khác không được đụng tới")
	}
}

func TestDeleteFolderRejectsBadInput(t *testing.T) {
	_, router := newFoldersRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"type":`},
		{"path vượt cấp", `{"type":"Images","path":"../../"}`},
		{"resource type lạ", `{"type":"Secrets","path":"/"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodDelete, "/api/folder", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

// ── PATCH /api/folder/rename ──────────────────────────────────────────

func TestRenameFolderMovesObjectsToNewPrefix(t *testing.T) {
	fake, router := newFoldersRouter(t)
	fake.Put("images/photos/a.jpg", []byte("a"))
	fake.Put("images/photos/sub/b.jpg", []byte("b"))

	w := doJSON(t, router, http.MethodPatch, "/api/folder/rename",
		`{"type":"Images","path":"/photos/","newName":"hinh-anh"}`)
	assertStatus(t, w, http.StatusOK)

	if !fake.Has("images/hinh-anh/a.jpg") {
		t.Fatalf("không thấy object ở tên mới, có: %v", fake.Keys())
	}
	if fake.Has("images/photos/a.jpg") {
		t.Fatalf("object ở tên cũ phải bị xoá, có: %v", fake.Keys())
	}
}

func TestRenameFolderKeepsParentPath(t *testing.T) {
	fake, router := newFoldersRouter(t)
	fake.Put("images/2026/thang-01/a.jpg", []byte("a"))

	w := doJSON(t, router, http.MethodPatch, "/api/folder/rename",
		`{"type":"Images","path":"/2026/thang-01/","newName":"thang-mot"}`)
	assertStatus(t, w, http.StatusOK)

	// Đổi tên phải giữ nguyên thư mục cha, không được nhảy lên gốc.
	if !fake.Has("images/2026/thang-mot/a.jpg") {
		t.Fatalf("không thấy object ở đúng thư mục cha, có: %v", fake.Keys())
	}
}

func TestRenameFolderRejectsBadInput(t *testing.T) {
	_, router := newFoldersRouter(t)

	tests := []struct{ name, body string }{
		{"JSON hỏng", `{"type":`},
		{"path vượt cấp", `{"type":"Images","path":"../../","newName":"x"}`},
		{"resource type lạ", `{"type":"Secrets","path":"/a/","newName":"x"}`},
		{"tên mới có gạch chéo", `{"type":"Images","path":"/a/","newName":"b/c"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, router, http.MethodPatch, "/api/folder/rename", tc.body)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}
