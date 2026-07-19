package minioclient

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestDeleteFilesDeletesThumbnailObjects(t *testing.T) {
	key := "images/kickone/photo.png"
	thumbs := []string{
		ThumbnailKey(key, ThumbOptions{Width: 150, Height: 150}),
		ThumbnailKey(key, ThumbOptions{Width: 800, Height: 0}),
	}
	var deleted []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			if got, want := r.URL.Query().Get("prefix"), thumbnailPrefix(key); got != want {
				t.Fatalf("listed prefix %q, want %q", got, want)
			}
			writeListObjectsResponse(w, thumbs)
		case r.Method == http.MethodPost && hasDeleteQuery(r):
			deleted = append(deleted, readDeleteObjectKeys(t, r.Body)...)
			writeDeleteObjectsResponse(w)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	client := newTestS3Client(server.URL)

	if err := client.DeleteFiles(context.Background(), []string{key}); err != nil {
		t.Fatalf("DeleteFiles returned error: %v", err)
	}

	want := []string{key, thumbs[0], thumbs[1]}
	assertSameStrings(t, deleted, want)
}

func TestDeleteFilesSucceedsWhenNoThumbnailObjectsExist(t *testing.T) {
	key := "images/kickone/no-thumb.png"
	var deleted []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			if got, want := r.URL.Query().Get("prefix"), thumbnailPrefix(key); got != want {
				t.Fatalf("listed prefix %q, want %q", got, want)
			}
			writeListObjectsResponse(w, nil)
		case r.Method == http.MethodPost && hasDeleteQuery(r):
			deleted = append(deleted, readDeleteObjectKeys(t, r.Body)...)
			writeDeleteObjectsResponse(w)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	client := newTestS3Client(server.URL)

	if err := client.DeleteFiles(context.Background(), []string{key}); err != nil {
		t.Fatalf("DeleteFiles returned error: %v", err)
	}

	assertSameStrings(t, deleted, []string{key})
}

func TestRenameFileRenamesThumbnailObjects(t *testing.T) {
	srcKey := "images/kickone/old.png"
	dstKey := "images/kickone/new.png"
	srcThumbs := []string{
		ThumbnailKey(srcKey, ThumbOptions{Width: 150, Height: 150}),
		ThumbnailKey(srcKey, ThumbOptions{Width: 800, Height: 0}),
	}
	wantCopied := []string{
		dstKey,
		ThumbnailKey(dstKey, ThumbOptions{Width: 150, Height: 150}),
		ThumbnailKey(dstKey, ThumbOptions{Width: 800, Height: 0}),
	}
	var copied []string
	var deleted []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			copied = append(copied, strings.TrimPrefix(r.URL.Path, "/media/"))
			writeCopyObjectResponse(w)
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			if got, want := r.URL.Query().Get("prefix"), thumbnailPrefix(srcKey); got != want {
				t.Fatalf("listed prefix %q, want %q", got, want)
			}
			writeListObjectsResponse(w, srcThumbs)
		case r.Method == http.MethodPost && hasDeleteQuery(r):
			deleted = append(deleted, readDeleteObjectKeys(t, r.Body)...)
			writeDeleteObjectsResponse(w)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	client := newTestS3Client(server.URL)

	if err := client.RenameFile(context.Background(), srcKey, dstKey); err != nil {
		t.Fatalf("RenameFile returned error: %v", err)
	}

	assertSameStrings(t, copied, wantCopied)
	assertSameStrings(t, deleted, []string{srcKey, srcThumbs[0], srcThumbs[1]})
}

func TestRenameFileSucceedsWhenNoThumbnailObjectsExist(t *testing.T) {
	srcKey := "images/kickone/old-no-thumb.png"
	dstKey := "images/kickone/new-no-thumb.png"
	var copied []string
	var deleted []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut:
			copied = append(copied, strings.TrimPrefix(r.URL.Path, "/media/"))
			writeCopyObjectResponse(w)
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			if got, want := r.URL.Query().Get("prefix"), thumbnailPrefix(srcKey); got != want {
				t.Fatalf("listed prefix %q, want %q", got, want)
			}
			writeListObjectsResponse(w, nil)
		case r.Method == http.MethodPost && hasDeleteQuery(r):
			deleted = append(deleted, readDeleteObjectKeys(t, r.Body)...)
			writeDeleteObjectsResponse(w)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	client := newTestS3Client(server.URL)

	if err := client.RenameFile(context.Background(), srcKey, dstKey); err != nil {
		t.Fatalf("RenameFile returned error: %v", err)
	}

	assertSameStrings(t, copied, []string{dstKey})
	assertSameStrings(t, deleted, []string{srcKey})
}

func newTestS3Client(endpoint string) *Client {
	return &Client{
		S3: s3.New(s3.Options{
			BaseEndpoint: aws.String(endpoint),
			Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
			Region:       "us-east-1",
			UsePathStyle: true,
		}),
		Bucket: "media",
	}
}

func hasDeleteQuery(r *http.Request) bool {
	_, ok := r.URL.Query()["delete"]
	return ok
}

func writeListObjectsResponse(w http.ResponseWriter, keys []string) {
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`))
	_, _ = w.Write([]byte(`<Name>media</Name><IsTruncated>false</IsTruncated>`))
	for _, key := range keys {
		_, _ = w.Write([]byte(`<Contents><Key>` + key + `</Key><LastModified>2026-07-19T00:00:00Z</LastModified><Size>1</Size></Contents>`))
	}
	_, _ = w.Write([]byte(`</ListBucketResult>`))
}

func writeDeleteObjectsResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(`<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/" />`))
}

func writeCopyObjectResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(`<CopyObjectResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><ETag>"etag"</ETag></CopyObjectResult>`))
}

func readDeleteObjectKeys(t *testing.T, body io.Reader) []string {
	t.Helper()

	var req struct {
		Objects []struct {
			Key string `xml:"Key"`
		} `xml:"Object"`
	}
	if err := xml.NewDecoder(body).Decode(&req); err != nil {
		t.Fatalf("decode delete request: %v", err)
	}

	keys := make([]string, 0, len(req.Objects))
	for _, obj := range req.Objects {
		key, err := url.QueryUnescape(obj.Key)
		if err != nil {
			t.Fatalf("unescape key %q: %v", obj.Key, err)
		}
		keys = append(keys, key)
	}
	return keys
}

func assertSameStrings(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	counts := make(map[string]int, len(want))
	for _, value := range want {
		counts[value]++
	}
	for _, value := range got {
		counts[value]--
	}
	for value, count := range counts {
		if count != 0 {
			t.Fatalf("got %v, want %v; mismatch at %q", got, want, value)
		}
	}
}
