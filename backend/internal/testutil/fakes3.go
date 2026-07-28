package testutil

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FakeS3 là một S3 in-memory đủ dùng cho các thao tác mà internal/minio gọi:
// ListObjectsV2, Put/Get/Head/Delete object, DeleteObjects và CopyObject.
// Có nó thì test được handler thật với client minio thật, không cần mock
// từng method của tầng storage.
type FakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte

	// failOn ép một thao tác trả 500, để test nhánh lỗi của handler.
	// Khoá dạng "PUT", "GET", "LIST", "DELETE", "COPY".
	failOn map[string]bool
}

func newFakeS3() *FakeS3 {
	return &FakeS3{objects: map[string][]byte{}, failOn: map[string]bool{}}
}

func (f *FakeS3) Put(key string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = body
}

func (f *FakeS3) Has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func (f *FakeS3) Keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.objects))
	for k := range f.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (f *FakeS3) Fail(op string) { f.failOn[op] = true }

type listBucketResult struct {
	XMLName        xml.Name          `xml:"ListBucketResult"`
	Name           string            `xml:"Name"`
	Prefix         string            `xml:"Prefix"`
	Delimiter      string            `xml:"Delimiter,omitempty"`
	KeyCount       int               `xml:"KeyCount"`
	MaxKeys        int               `xml:"MaxKeys"`
	IsTruncated    bool              `xml:"IsTruncated"`
	Contents       []listObject      `xml:"Contents"`
	CommonPrefixes []commonPrefixXML `xml:"CommonPrefixes"`
}

type listObject struct {
	Key          string `xml:"Key"`
	Size         int64  `xml:"Size"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
}

type commonPrefixXML struct {
	Prefix string `xml:"Prefix"`
}

type deleteRequest struct {
	XMLName xml.Name `xml:"Delete"`
	Objects []struct {
		Key string `xml:"Key"`
	} `xml:"Object"`
}

type deleteResult struct {
	XMLName xml.Name `xml:"DeleteResult"`
	Deleted []struct {
		Key string `xml:"Key"`
	} `xml:"Deleted"`
}

// splitPath tách "/bucket/some/key" thành ("bucket", "some/key").
func splitPath(p string) (bucket, key string) {
	trimmed := strings.TrimPrefix(p, "/")
	if i := strings.Index(trimmed, "/"); i >= 0 {
		return trimmed[:i], trimmed[i+1:]
	}
	return trimmed, ""
}

func (f *FakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, key := splitPath(r.URL.Path)
	q := r.URL.Query()

	switch {
	case r.Method == http.MethodGet && q.Get("list-type") == "2":
		f.handleList(w, q)

	case r.Method == http.MethodPost && q.Has("delete"):
		f.handleDeleteObjects(w, r)

	case r.Method == http.MethodPut && r.Header.Get("X-Amz-Copy-Source") != "":
		f.handleCopy(w, r, key)

	case r.Method == http.MethodPut:
		f.handlePut(w, r, key)

	case r.Method == http.MethodHead && key == "":
		// HeadBucket — Fail("HEADBUCKET") giả lập bucket chưa tồn tại
		// để chạm nhánh tạo bucket.
		if f.failOn["HEADBUCKET"] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)

	case r.Method == http.MethodHead:
		f.handleHead(w, key)

	case r.Method == http.MethodGet:
		f.handleGet(w, key)

	case r.Method == http.MethodDelete:
		f.mu.Lock()
		delete(f.objects, key)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (f *FakeS3) handleList(w http.ResponseWriter, q url.Values) {
	if f.failOn["LIST"] {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	prefix := q.Get("prefix")
	delimiter := q.Get("delimiter")

	f.mu.Lock()
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	res := listBucketResult{Name: "media", Prefix: prefix, Delimiter: delimiter, MaxKeys: 1000}
	seenPrefix := map[string]bool{}
	for _, k := range keys {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		if delimiter != "" {
			if i := strings.Index(rest, delimiter); i >= 0 {
				cp := prefix + rest[:i+len(delimiter)]
				if !seenPrefix[cp] {
					seenPrefix[cp] = true
					res.CommonPrefixes = append(res.CommonPrefixes, commonPrefixXML{Prefix: cp})
				}
				continue
			}
		}
		res.Contents = append(res.Contents, listObject{
			Key:          k,
			Size:         int64(len(f.objects[k])),
			LastModified: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
			ETag:         `"fake-etag"`,
		})
	}
	f.mu.Unlock()

	res.KeyCount = len(res.Contents) + len(res.CommonPrefixes)
	writeXML(w, res)
}

func (f *FakeS3) handleDeleteObjects(w http.ResponseWriter, r *http.Request) {
	if f.failOn["DELETE"] {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, _ := io.ReadAll(r.Body)
	var req deleteRequest
	_ = xml.Unmarshal(body, &req)

	var res deleteResult
	f.mu.Lock()
	for _, o := range req.Objects {
		delete(f.objects, o.Key)
		res.Deleted = append(res.Deleted, struct {
			Key string `xml:"Key"`
		}{Key: o.Key})
	}
	f.mu.Unlock()

	writeXML(w, res)
}

func (f *FakeS3) handleCopy(w http.ResponseWriter, r *http.Request, dstKey string) {
	if f.failOn["COPY"] {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	src := r.Header.Get("X-Amz-Copy-Source")
	src = strings.TrimPrefix(src, "/")
	if i := strings.Index(src, "/"); i >= 0 {
		src = src[i+1:]
	}
	if decoded, err := url.QueryUnescape(src); err == nil {
		src = decoded
	}

	f.mu.Lock()
	body, ok := f.objects[src]
	if ok {
		f.objects[dstKey] = body
	}
	f.mu.Unlock()

	if !ok {
		writeS3Error(w, http.StatusNotFound, "NoSuchKey")
		return
	}
	writeXML(w, struct {
		XMLName xml.Name `xml:"CopyObjectResult"`
		ETag    string   `xml:"ETag"`
	}{ETag: `"fake-etag"`})
}

func (f *FakeS3) handlePut(w http.ResponseWriter, r *http.Request, key string) {
	if f.failOn["PUT"] {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.objects[key] = body
	f.mu.Unlock()
	w.Header().Set("ETag", `"fake-etag"`)
	w.WriteHeader(http.StatusOK)
}

func (f *FakeS3) handleHead(w http.ResponseWriter, key string) {
	f.mu.Lock()
	body, ok := f.objects[key]
	f.mu.Unlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("ETag", `"fake-etag"`)
	w.WriteHeader(http.StatusOK)
}

func (f *FakeS3) handleGet(w http.ResponseWriter, key string) {
	if f.failOn["GET"] {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	f.mu.Lock()
	body, ok := f.objects[key]
	f.mu.Unlock()

	if !ok {
		writeS3Error(w, http.StatusNotFound, "NoSuchKey")
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func writeXML(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

func writeS3Error(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<?xml version="1.0"?><Error><Code>` + code + `</Code></Error>`))
}

func StartFakeS3(t interface{ Cleanup(func()) }) (*FakeS3, string) {
	fake := newFakeS3()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	return fake, strings.TrimPrefix(server.URL, "http://")
}
