package minioclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ckfindercompatible/backend/internal/config"
	"github.com/ckfindercompatible/backend/internal/testutil"
	"go.uber.org/zap"
)

// newFixture dựng một Client thật trỏ vào fake S3 in-memory.
func newFixture(t *testing.T) (*testutil.FakeS3, *Client) {
	t.Helper()

	// Không retry: test nhánh lỗi cố tình trả 500, để mặc định thì SDK
	// thử lại kèm backoff làm suite chậm hẳn.
	t.Setenv("AWS_MAX_ATTEMPTS", "1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	fake, endpoint := testutil.StartFakeS3(t)
	cfg := &config.Config{
		MinioEndpoint:      endpoint,
		MinioAccessKey:     "test-access",
		MinioSecretKey:     "test-secret",
		MinioBucket:        "media",
		MinioUseSSL:        false,
		MinioPublicBaseURL: "http://cdn.example.test/",
	}

	c, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return fake, c
}

// pngBytes trả về một PNG 1x1 hợp lệ.
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

// ── client.go ─────────────────────────────────────────────────────────

func TestNewSetsSingletonAndBucket(t *testing.T) {
	_, c := newFixture(t)

	if c.Bucket != "media" {
		t.Errorf("Bucket = %q", c.Bucket)
	}
	if Get() != c {
		t.Error("Get() phải trả về client vừa tạo")
	}
}

func TestEnsureBucketIsIdempotent(t *testing.T) {
	_, c := newFixture(t)
	ctx := context.Background()

	// Fake trả 200 cho HeadBucket nên coi như bucket đã có.
	if err := c.EnsureBucket(ctx); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if err := c.EnsureBucket(ctx); err != nil {
		t.Fatalf("EnsureBucket lần hai: %v", err)
	}
}

func TestPublicURLJoinsBaseBucketAndKey(t *testing.T) {
	_, c := newFixture(t)

	// Base URL có dấu "/" thừa ở cuối, không được sinh ra "//".
	want := "http://cdn.example.test/media/images/anh.png"
	if got := c.PublicURL("images/anh.png"); got != want {
		t.Fatalf("PublicURL = %q, muốn %q", got, want)
	}
}

// ── folders.go: hàm thuần ─────────────────────────────────────────────

func TestBuildKey(t *testing.T) {
	tests := []struct {
		prefix, folder, file, want string
	}{
		{"images", "/photos/", "cat.jpg", "images/photos/cat.jpg"},
		{"images", "/", "cat.jpg", "images/cat.jpg"},
		{"images", "", "cat.jpg", "images/cat.jpg"},
		{"/images/", "/photos/", "cat.jpg", "images/photos/cat.jpg"},
		{"images", "photos/2026", "cat.jpg", "images/photos/2026/cat.jpg"},
		{"images", `\photos\`, "cat.jpg", "images/photos/cat.jpg"},
	}
	for _, tc := range tests {
		if got := BuildKey(tc.prefix, tc.folder, tc.file); got != tc.want {
			t.Errorf("BuildKey(%q,%q,%q) = %q, muốn %q", tc.prefix, tc.folder, tc.file, got, tc.want)
		}
	}
}

func TestFolderPrefix(t *testing.T) {
	tests := []struct {
		prefix, folder, want string
	}{
		{"images", "/photos/", "images/photos/"},
		{"images", "/", "images/"},
		{"images", "", "images/"},
		{"/images/", "photos", "images/photos/"},
	}
	for _, tc := range tests {
		if got := FolderPrefix(tc.prefix, tc.folder); got != tc.want {
			t.Errorf("FolderPrefix(%q,%q) = %q, muốn %q", tc.prefix, tc.folder, got, tc.want)
		}
	}
}

func TestNormalizeFolderPathAcceptsValid(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"/", ""},
		{"/photos/", "photos"},
		{"photos/2026", "photos/2026"},
		{`\photos\2026\`, "photos/2026"},
		{"  /photos/  ", "photos"},
	}
	for _, tc := range tests {
		got, err := NormalizeFolderPath(tc.in)
		if err != nil {
			t.Errorf("NormalizeFolderPath(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeFolderPath(%q) = %q, muốn %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeObjectNameAcceptsValid(t *testing.T) {
	for _, name := range []string{"anh.png", "tài liệu.pdf", "a b c.txt", ".gitignore"} {
		got, err := NormalizeObjectName(name)
		if err != nil {
			t.Errorf("NormalizeObjectName(%q): %v", name, err)
			continue
		}
		if got != name {
			t.Errorf("NormalizeObjectName(%q) = %q", name, got)
		}
	}
}

func TestNormalizeObjectNameRejectsInvalid(t *testing.T) {
	invalid := []string{"", "   ", ".", "..", "a/b", `a\b`, "a\x00b"}
	for _, name := range invalid {
		if got, err := NormalizeObjectName(name); err == nil {
			t.Errorf("NormalizeObjectName(%q) = %q, muốn lỗi", name, got)
		}
	}
}

func TestNormalizeRelativeObjectPathAcceptsNested(t *testing.T) {
	got, err := NormalizeRelativeObjectPath("thu-muc/anh.png")
	if err != nil {
		t.Fatalf("NormalizeRelativeObjectPath: %v", err)
	}
	if got != "thu-muc/anh.png" {
		t.Fatalf("= %q", got)
	}
}

// ── files.go / folders.go: có storage ─────────────────────────────────

func TestPutAndGetObjectRoundTrip(t *testing.T) {
	_, c := newFixture(t)
	ctx := context.Background()

	content := []byte("noi dung file")
	if err := c.PutFile(ctx, "images/a.txt", "text/plain", bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("PutFile: %v", err)
	}

	body, size, err := c.GetObject(ctx, "images/a.txt")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer body.Close()

	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("đọc body: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("nội dung = %q, muốn %q", got, content)
	}
	if size != int64(len(content)) {
		t.Errorf("size = %d, muốn %d", size, len(content))
	}
}

func TestPutFileWithoutSizeStillStores(t *testing.T) {
	fake, c := newFixture(t)

	// size <= 0 thì không đặt ContentLength; object vẫn phải ghi được.
	if err := c.PutFile(context.Background(), "images/a.txt", "text/plain", bytes.NewReader([]byte("x")), 0); err != nil {
		t.Fatalf("PutFile: %v", err)
	}
	if !fake.Has("images/a.txt") {
		t.Fatalf("không thấy object, có: %v", fake.Keys())
	}
}

func TestGetObjectMissingKeyErrors(t *testing.T) {
	_, c := newFixture(t)

	if _, _, err := c.GetObject(context.Background(), "images/khong-co.txt"); err == nil {
		t.Fatal("object không tồn tại phải trả lỗi")
	}
}

func TestHeadObjectReportsExistence(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", []byte("12345"))
	ctx := context.Background()

	if _, err := c.HeadObject(ctx, "images/a.png"); err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if _, err := c.HeadObject(ctx, "images/khong-co.png"); err == nil {
		t.Fatal("HeadObject với key không tồn tại phải trả lỗi")
	}
}

func TestListFilesSkipsFolderPlaceholdersAndSubfolders(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", []byte("12345"))
	fake.Put("images/b.png", []byte("12"))
	fake.Put("images/sub/c.png", []byte("123"))

	objs, err := c.ListFiles(context.Background(), "images/")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}

	names := map[string]int64{}
	for _, o := range objs {
		names[o.Name] = o.Size
	}
	if len(names) != 2 {
		t.Fatalf("nhận %d file, muốn 2 (không tính file trong thư mục con): %+v", len(names), objs)
	}
	if names["a.png"] != 5 {
		t.Errorf("size a.png = %d, muốn 5", names["a.png"])
	}
}

func TestDeleteFileRemovesObject(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", []byte("a"))

	if err := c.DeleteFile(context.Background(), "images/a.png"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if fake.Has("images/a.png") {
		t.Fatal("object phải bị xoá")
	}
}

func TestDeleteFilesWithEmptyListIsNoop(t *testing.T) {
	_, c := newFixture(t)

	if err := c.DeleteFiles(context.Background(), nil); err != nil {
		t.Fatalf("DeleteFiles(nil): %v", err)
	}
}

func TestDeleteFilesReportsListFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", []byte("a"))
	fake.Fail("LIST")

	if err := c.DeleteFiles(context.Background(), []string{"images/a.png"}); err == nil {
		t.Fatal("lỗi khi liệt kê thumbnail phải nổi lên")
	}
}

func TestCopyFileDuplicatesContent(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", []byte("noi dung"))

	if err := c.CopyFile(context.Background(), "images/a.png", "images/b.png"); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	if !fake.Has("images/b.png") {
		t.Fatalf("không thấy bản sao, có: %v", fake.Keys())
	}
	if !fake.Has("images/a.png") {
		t.Error("bản gốc phải còn")
	}
}

func TestCopyFileMissingSourceErrors(t *testing.T) {
	_, c := newFixture(t)

	if err := c.CopyFile(context.Background(), "images/khong-co.png", "images/b.png"); err == nil {
		t.Fatal("copy từ key không tồn tại phải trả lỗi")
	}
}

func TestRenameFileMovesAndDeletesSource(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/cu.png", []byte("x"))

	if err := c.RenameFile(context.Background(), "images/cu.png", "images/moi.png"); err != nil {
		t.Fatalf("RenameFile: %v", err)
	}
	if !fake.Has("images/moi.png") || fake.Has("images/cu.png") {
		t.Fatalf("rename không đúng, có: %v", fake.Keys())
	}
}

func TestRenameFileFailsWhenCopyFails(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/cu.png", []byte("x"))
	fake.Fail("COPY")

	if err := c.RenameFile(context.Background(), "images/cu.png", "images/moi.png"); err == nil {
		t.Fatal("copy hỏng thì rename phải lỗi")
	}
	if !fake.Has("images/cu.png") {
		t.Error("copy hỏng thì không được xoá bản gốc")
	}
}

func TestPresignGetObjectProducesSignedURL(t *testing.T) {
	_, c := newFixture(t)

	url, err := c.PresignGetObject(context.Background(), "images/a.png", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGetObject: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Errorf("URL chưa được ký: %q", url)
	}
	if !strings.Contains(url, "images/a.png") {
		t.Errorf("URL thiếu key: %q", url)
	}
}

func TestListFoldersReturnsImmediateChildren(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/photos/.keep", nil)
	fake.Put("images/videos/.keep", nil)
	fake.Put("images/photos/sub/.keep", nil)
	fake.Put("images/a.png", []byte("a"))

	names, err := c.ListFolders(context.Background(), "images/")
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}

	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	if len(set) != 2 || !set["photos"] || !set["videos"] {
		t.Fatalf("nhận %v, muốn photos và videos", names)
	}
}

func TestHasSubfolders(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/co-con/sub/.keep", nil)
	fake.Put("images/khong-con/a.png", []byte("a"))
	ctx := context.Background()

	got, err := c.HasSubfolders(ctx, "images/co-con/")
	if err != nil {
		t.Fatalf("HasSubfolders: %v", err)
	}
	if !got {
		t.Error("thư mục có con phải trả true")
	}

	got, err = c.HasSubfolders(ctx, "images/khong-con/")
	if err != nil {
		t.Fatalf("HasSubfolders: %v", err)
	}
	if got {
		t.Error("thư mục chỉ có file phải trả false")
	}
}

func TestCreateFolderWritesPlaceholder(t *testing.T) {
	fake, c := newFixture(t)

	if err := c.CreateFolder(context.Background(), "images/moi"); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}

	found := false
	for _, k := range fake.Keys() {
		if strings.HasPrefix(k, "images/moi/") {
			found = true
		}
	}
	if !found {
		t.Fatalf("không thấy placeholder, có: %v", fake.Keys())
	}
}

func TestDeleteFolderRemovesSubtree(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/photos/a.png", []byte("a"))
	fake.Put("images/photos/sub/b.png", []byte("b"))
	fake.Put("images/khac/c.png", []byte("c"))

	if err := c.DeleteFolder(context.Background(), "images/photos/"); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}

	for _, k := range fake.Keys() {
		if strings.HasPrefix(k, "images/photos/") {
			t.Fatalf("còn sót %q", k)
		}
	}
	if !fake.Has("images/khac/c.png") {
		t.Error("thư mục khác không được đụng tới")
	}
}

func TestDeleteFolderEmptyPrefixIsNoop(t *testing.T) {
	_, c := newFixture(t)

	if err := c.DeleteFolder(context.Background(), "images/rong/"); err != nil {
		t.Fatalf("DeleteFolder trên thư mục rỗng: %v", err)
	}
}

func TestRenameFolderMovesSubtree(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/cu/a.png", []byte("a"))
	fake.Put("images/cu/sub/b.png", []byte("b"))

	if err := c.RenameFolder(context.Background(), "images/cu/", "images/moi"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}

	if !fake.Has("images/moi/a.png") || !fake.Has("images/moi/sub/b.png") {
		t.Fatalf("không thấy subtree ở tên mới, có: %v", fake.Keys())
	}
	if fake.Has("images/cu/a.png") {
		t.Error("bản cũ phải bị xoá")
	}
}

func TestRenameFolderReportsListFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Fail("LIST")

	if err := c.RenameFolder(context.Background(), "images/cu/", "images/moi"); err == nil {
		t.Fatal("lỗi liệt kê phải nổi lên")
	}
}

// ── thumbnail.go ──────────────────────────────────────────────────────

func TestIsImage(t *testing.T) {
	yes := []string{"a.jpg", "a.JPEG", "a.png", "a.gif", "a.webp", "a.bmp", "thu-muc/a.png"}
	for _, n := range yes {
		if !IsImage(n) {
			t.Errorf("IsImage(%q) = false, muốn true", n)
		}
	}

	no := []string{"a.pdf", "a.mp4", "a.svg", "khong-co-duoi", "", "a."}
	for _, n := range no {
		if IsImage(n) {
			t.Errorf("IsImage(%q) = true, muốn false", n)
		}
	}
}

func TestThumbnailKeyEncodesDimensions(t *testing.T) {
	got := ThumbnailKey("images/a.png", ThumbOptions{Width: 150, Height: 100})
	want := "_thumbs/images/a.png_150x100.jpg"
	if got != want {
		t.Fatalf("ThumbnailKey = %q, muốn %q", got, want)
	}
}

func TestGetOrCreateThumbnailCreatesThenCaches(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", pngBytes())
	ctx := context.Background()
	opts := ThumbOptions{Width: 50, Height: 50, Fit: true}

	first, err := c.GetOrCreateThumbnail(ctx, "images/a.png", opts)
	if err != nil {
		t.Fatalf("GetOrCreateThumbnail: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("thumbnail rỗng")
	}
	if !fake.Has(ThumbnailKey("images/a.png", opts)) {
		t.Fatalf("thumbnail phải được cache lại, có: %v", fake.Keys())
	}

	// Lần hai đọc từ cache, phải ra đúng nội dung đó.
	second, err := c.GetOrCreateThumbnail(ctx, "images/a.png", opts)
	if err != nil {
		t.Fatalf("GetOrCreateThumbnail lần hai: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("lần hai phải trả đúng thumbnail đã cache")
	}
}

func TestGetOrCreateThumbnailFitFalseAlsoWorks(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", pngBytes())

	got, err := c.GetOrCreateThumbnail(context.Background(), "images/a.png",
		ThumbOptions{Width: 80, Height: 0, Fit: false})
	if err != nil {
		t.Fatalf("GetOrCreateThumbnail: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("thumbnail rỗng")
	}
}

func TestGetOrCreateThumbnailMissingOriginal(t *testing.T) {
	_, c := newFixture(t)

	_, err := c.GetOrCreateThumbnail(context.Background(), "images/khong-co.png",
		ThumbOptions{Width: 50, Height: 50})
	if err == nil {
		t.Fatal("ảnh gốc không tồn tại phải trả lỗi")
	}
	if !IsOriginalNotFound(err) {
		t.Fatalf("IsOriginalNotFound = false với lỗi %v", err)
	}
}

func TestGetOrCreateThumbnailUndecodableOriginal(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/hong.png", []byte("day khong phai anh"))

	_, err := c.GetOrCreateThumbnail(context.Background(), "images/hong.png",
		ThumbOptions{Width: 50, Height: 50})
	if err == nil {
		t.Fatal("ảnh hỏng phải trả lỗi")
	}
	// Không phải "thiếu ảnh gốc" — ảnh có đó nhưng giải mã không được.
	if IsOriginalNotFound(err) {
		t.Error("lỗi giải mã không được coi là thiếu ảnh gốc")
	}
}

func TestIsOriginalNotFoundOnlyMatchesSentinel(t *testing.T) {
	if IsOriginalNotFound(nil) {
		t.Error("nil không phải lỗi thiếu ảnh gốc")
	}
	if IsOriginalNotFound(errors.New("lỗi khác")) {
		t.Error("lỗi lạ không phải lỗi thiếu ảnh gốc")
	}
	if !IsOriginalNotFound(ErrOriginalNotFound) {
		t.Error("sentinel phải khớp")
	}
}

func TestIsObjectNotFoundIgnoresUnrelatedErrors(t *testing.T) {
	if isObjectNotFound(nil) {
		t.Error("nil không phải NotFound")
	}
	if isObjectNotFound(errors.New("mất kết nối")) {
		t.Error("lỗi mạng không phải NotFound")
	}
}

func TestAppendUniqueKey(t *testing.T) {
	keys := appendUniqueKey(nil, "a")
	keys = appendUniqueKey(keys, "b")
	keys = appendUniqueKey(keys, "a") // trùng, phải bị bỏ

	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("appendUniqueKey = %v, muốn [a b]", keys)
	}
}

func TestGetExt(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a.png", ".png"},
		{"thu-muc/a.tar.gz", ".gz"},
		{"khong-co-duoi", ""},
		{"", ""},
		{"a.", "."},
	}
	for _, tc := range tests {
		if got := getExt(tc.in); got != tc.want {
			t.Errorf("getExt(%q) = %q, muốn %q", tc.in, got, tc.want)
		}
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestReadAllPropagatesError(t *testing.T) {
	want := errors.New("đọc hỏng")
	if _, err := readAll(errReader{err: want}); !errors.Is(err, want) {
		t.Fatalf("readAll err = %v, muốn %v", err, want)
	}
}

func TestReadAllReadsEverything(t *testing.T) {
	content := bytes.Repeat([]byte("x"), 100*1024) // vượt buffer 32KB
	got, err := readAll(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("readAll trả về %d byte, muốn %d", len(got), len(content))
	}
}
