package minioclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestGetOrCreateThumbnailDeletesCachedThumbWhenOriginalMissing(t *testing.T) {
	t.Parallel()

	opts := ThumbOptions{Width: 150, Height: 150, Fit: true}
	originalKey := "images/kickone/missing.png"
	thumbKey := ThumbnailKey(originalKey, opts)
	deleted := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>not found</Message></Error>`))
		case http.MethodDelete:
			deleted <- r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(server.Close)

	client := &Client{
		S3: s3.New(s3.Options{
			BaseEndpoint: aws.String(server.URL),
			Credentials:  credentials.NewStaticCredentialsProvider("test", "test", ""),
			Region:       "us-east-1",
			UsePathStyle: true,
		}),
		Bucket: "media",
	}

	_, err := client.GetOrCreateThumbnail(context.Background(), originalKey, opts)
	if err == nil {
		t.Fatal("expected missing original error")
	}
	if !IsOriginalNotFound(err) {
		t.Fatalf("expected original not found error, got %v", err)
	}

	wantPath := "/media/" + thumbKey
	select {
	case gotPath := <-deleted:
		if gotPath != wantPath {
			t.Fatalf("deleted %q, want %q", gotPath, wantPath)
		}
	default:
		t.Fatalf("expected thumbnail %q to be deleted", thumbKey)
	}
}

func TestGetOrCreateThumbnailDoesNotTreatOtherOriginalErrorsAsMissing(t *testing.T) {
	t.Parallel()

	opts := ThumbOptions{Width: 150, Height: 150, Fit: true}
	originalKey := "images/kickone/error.png"
	requests := 0
	deleted := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			requests++
			w.Header().Set("Content-Type", "application/xml")
			if requests == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>not found</Message></Error>`))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<Error><Code>InternalError</Code><Message>try again</Message></Error>`))
		case http.MethodDelete:
			deleted <- r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(server.Close)

	client := newTestS3Client(server.URL)

	_, err := client.GetOrCreateThumbnail(context.Background(), originalKey, opts)
	if err == nil {
		t.Fatal("expected original fetch error")
	}
	if IsOriginalNotFound(err) {
		t.Fatalf("expected non-missing original error, got %v", err)
	}

	select {
	case gotPath := <-deleted:
		t.Fatalf("unexpected thumbnail delete %q", gotPath)
	default:
	}
}
