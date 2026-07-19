package minioclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeleteFolderDeletesThumbnailObjectsUnderFolder(t *testing.T) {
	prefix := "images/kickone/"
	thumbPrefix := "_thumbs/" + prefix
	folderKeys := []string{
		"images/kickone/.keep",
		"images/kickone/photo.png",
	}
	thumbKeys := []string{
		"_thumbs/images/kickone/photo.png_150x150.jpg",
		"_thumbs/images/kickone/nested/avatar.png_150x150.jpg",
	}
	var listed []string
	var deleted []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			p := r.URL.Query().Get("prefix")
			listed = append(listed, p)
			switch p {
			case prefix:
				writeListObjectsResponse(w, folderKeys)
			case thumbPrefix:
				writeListObjectsResponse(w, thumbKeys)
			default:
				t.Fatalf("unexpected listed prefix %q", p)
			}
		case r.Method == http.MethodPost && hasDeleteQuery(r):
			deleted = append(deleted, readDeleteObjectKeys(t, r.Body)...)
			writeDeleteObjectsResponse(w)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	client := newTestS3Client(server.URL)

	if err := client.DeleteFolder(context.Background(), prefix); err != nil {
		t.Fatalf("DeleteFolder returned error: %v", err)
	}

	assertSameStrings(t, listed, []string{prefix, thumbPrefix})
	assertSameStrings(t, deleted, []string{
		folderKeys[0],
		folderKeys[1],
		thumbKeys[0],
		thumbKeys[1],
	})
}
