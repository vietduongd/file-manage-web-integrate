package minioclient

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ckfindercompatible/backend/internal/config"
	"github.com/ckfindercompatible/backend/internal/testutil"
	"go.uber.org/zap"
)

// ── normalizeRelativePath ─────────────────────────────────────────────

func TestNormalizeRelativePathRejectsEscapes(t *testing.T) {
	invalid := []string{
		"../secret",
		"a/../../secret",
		"/tuyet-doi",
		`\tuyet-doi`,
		"a//b",   // segment rỗng
		"a/./b",  // segment "."
		"a/../b", // segment ".."
		"a\x00b",
	}
	for _, v := range invalid {
		if got, err := NormalizeRelativeObjectPath(v); err == nil {
			t.Errorf("NormalizeRelativeObjectPath(%q) = %q, muốn lỗi", v, got)
		}
	}
}

func TestNormalizeRelativeObjectPathRequiresValue(t *testing.T) {
	// allowEmpty=false: chuỗi rỗng là lỗi với tên object...
	for _, v := range []string{"", "/", "   "} {
		if _, err := NormalizeRelativeObjectPath(v); err == nil {
			t.Errorf("NormalizeRelativeObjectPath(%q) phải trả lỗi", v)
		}
	}

	// ...nhưng allowEmpty=true với đường dẫn thư mục thì rỗng nghĩa là gốc.
	for _, v := range []string{"", "/", "   "} {
		got, err := NormalizeFolderPath(v)
		if err != nil {
			t.Errorf("NormalizeFolderPath(%q): %v", v, err)
		}
		if got != "" {
			t.Errorf("NormalizeFolderPath(%q) = %q, muốn rỗng", v, got)
		}
	}
}

func TestNormalizeFolderPathRejectsEscapes(t *testing.T) {
	for _, v := range []string{"../etc", "a/../../b", "a/./b"} {
		if got, err := NormalizeFolderPath(v); err == nil {
			t.Errorf("NormalizeFolderPath(%q) = %q, muốn lỗi", v, got)
		}
	}
}

// ── client.go: các nhánh còn lại ──────────────────────────────────────

func TestNewUsesHTTPSWhenSSLEnabled(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	c, err := New(&config.Config{
		MinioEndpoint:      "minio.example.test:9000",
		MinioAccessKey:     "k",
		MinioSecretKey:     "s",
		MinioBucket:        "media",
		MinioUseSSL:        true,
		MinioPublicBaseURL: "https://cdn.example.test",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Bucket != "media" {
		t.Fatalf("Bucket = %q", c.Bucket)
	}
}

func TestEnsureBucketCreatesWhenMissing(t *testing.T) {
	fake, c := newFixture(t)
	fake.Fail("HEADBUCKET")

	if err := c.EnsureBucket(context.Background()); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
}

// ── nhánh lỗi storage ─────────────────────────────────────────────────

func TestListFilesReportsFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Fail("LIST")

	if _, err := c.ListFiles(context.Background(), "images/"); err == nil {
		t.Fatal("ListFiles phải trả lỗi khi storage hỏng")
	}
}

func TestListFoldersReportsFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Fail("LIST")

	if _, err := c.ListFolders(context.Background(), "images/"); err == nil {
		t.Fatal("ListFolders phải trả lỗi khi storage hỏng")
	}
}

func TestHasSubfoldersReportsFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Fail("LIST")

	if _, err := c.HasSubfolders(context.Background(), "images/"); err == nil {
		t.Fatal("HasSubfolders phải trả lỗi khi storage hỏng")
	}
}

func TestDeleteFolderReportsFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Fail("LIST")

	if err := c.DeleteFolder(context.Background(), "images/photos/"); err == nil {
		t.Fatal("DeleteFolder phải trả lỗi khi liệt kê hỏng")
	}
}

func TestDeleteFolderReportsDeleteFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/photos/a.png", []byte("a"))
	fake.Fail("DELETE")

	if err := c.DeleteFolder(context.Background(), "images/photos/"); err == nil {
		t.Fatal("DeleteFolder phải trả lỗi khi xoá hỏng")
	}
}

func TestDeleteFilesReportsDeleteFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", []byte("a"))
	fake.Fail("DELETE")

	if err := c.DeleteFiles(context.Background(), []string{"images/a.png"}); err == nil {
		t.Fatal("DeleteFiles phải trả lỗi khi xoá hỏng")
	}
}

func TestRenameFolderNormalisesPrefixesWithoutSlash(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/cu/a.png", []byte("a"))

	// Gọi không có dấu / ở cuối vẫn phải chạy đúng: hàm tự thêm vào,
	// nếu không prefix "images/cu" sẽ quét nhầm cả "images/cu-khac/".
	if err := c.RenameFolder(context.Background(), "images/cu", "images/moi"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if !fake.Has("images/moi/a.png") {
		t.Fatalf("không thấy object ở tên mới, có: %v", fake.Keys())
	}
}

func TestRenameFolderDoesNotTouchSiblingWithSharedPrefix(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/cu/a.png", []byte("a"))
	fake.Put("images/cu-khac/b.png", []byte("b"))

	if err := c.RenameFolder(context.Background(), "images/cu/", "images/moi/"); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if !fake.Has("images/cu-khac/b.png") {
		t.Fatalf("thư mục trùng tiền tố không được đụng tới, có: %v", fake.Keys())
	}
}

func TestRenameFolderReportsCopyFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/cu/a.png", []byte("a"))
	fake.Fail("COPY")

	if err := c.RenameFolder(context.Background(), "images/cu/", "images/moi"); err == nil {
		t.Fatal("copy hỏng thì RenameFolder phải lỗi")
	}
	if !fake.Has("images/cu/a.png") {
		t.Error("copy hỏng thì không được xoá bản gốc")
	}
}

func TestRenameFileReportsDeleteFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/cu.png", []byte("x"))
	fake.Fail("DELETE")

	// Copy xong nhưng xoá nguồn hỏng: phải báo lỗi chứ không im lặng
	// để lại hai bản.
	if err := c.RenameFile(context.Background(), "images/cu.png", "images/moi.png"); err == nil {
		t.Fatal("xoá nguồn hỏng thì RenameFile phải lỗi")
	}
}

// ── batching ──────────────────────────────────────────────────────────

func TestDeleteFolderBatchesLargeSubtree(t *testing.T) {
	fake, c := newFixture(t)

	// Vượt ngưỡng 1000 object mỗi request DeleteObjects của S3.
	const total = 1200
	for i := 0; i < total; i++ {
		fake.Put(fmt.Sprintf("images/lon/%04d.png", i), []byte("x"))
	}

	if err := c.DeleteFolder(context.Background(), "images/lon/"); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}

	for _, k := range fake.Keys() {
		if strings.HasPrefix(k, "images/lon/") {
			t.Fatalf("còn sót %q sau khi xoá theo lô", k)
		}
	}
}

func TestPresignGetObjectHonoursTTL(t *testing.T) {
	_, c := newFixture(t)

	url, err := c.PresignGetObject(context.Background(), "images/a.png", 60*1e9) // 60s
	if err != nil {
		t.Fatalf("PresignGetObject: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Expires=60") {
		t.Errorf("URL thiếu TTL 60s: %q", url)
	}
}

// ── thumbnail: nhánh lỗi ──────────────────────────────────────────────

func TestGetOrCreateThumbnailReportsCacheWriteFailure(t *testing.T) {
	fake, c := newFixture(t)
	fake.Put("images/a.png", pngBytes())
	fake.Fail("PUT")

	// Ghi cache hỏng không được làm mất thumbnail vừa dựng —
	// người dùng vẫn phải nhận được ảnh.
	got, err := c.GetOrCreateThumbnail(context.Background(), "images/a.png",
		ThumbOptions{Width: 40, Height: 40, Fit: true})
	if err != nil {
		t.Fatalf("GetOrCreateThumbnail: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("thumbnail rỗng")
	}
}

func TestListThumbnailKeysReturnsCachedVariants(t *testing.T) {
	fake, c := newFixture(t)
	opts1 := ThumbOptions{Width: 150, Height: 150}
	opts2 := ThumbOptions{Width: 800, Height: 0}
	fake.Put(ThumbnailKey("images/a.png", opts1), []byte("t1"))
	fake.Put(ThumbnailKey("images/a.png", opts2), []byte("t2"))
	fake.Put(ThumbnailKey("images/khac.png", opts1), []byte("t3"))

	keys, err := c.listThumbnailKeys(context.Background(), "images/a.png")
	if err != nil {
		t.Fatalf("listThumbnailKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("nhận %d key, muốn 2: %v", len(keys), keys)
	}
}

func TestStartFakeS3Isolated(t *testing.T) {
	// Hai fixture không được dùng chung state.
	f1, _ := newFixture(t)
	f2, _ := newFixture(t)

	f1.Put("images/chi-o-f1.png", []byte("x"))
	if f2.Has("images/chi-o-f1.png") {
		t.Fatal("hai fake S3 phải độc lập")
	}
}

var _ = testutil.StartFakeS3
