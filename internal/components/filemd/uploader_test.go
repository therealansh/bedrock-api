package filemd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPUploader_Upload(t *testing.T) {
	var receivedContentType string
	var receivedBody []byte
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		receivedPath = r.URL.Path
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	uploader := &HTTPUploader{
		BaseURL: server.URL,
		Client:  server.Client(),
	}

	files := []LogUpload{
		{Field: "target_log", Filename: "target.log", Content: []byte("target")},
		{Field: "tracer_log", Filename: "tracer.log", Content: []byte("tracer")},
	}

	err := uploader.Upload("sess-1", files)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if receivedPath != "/api/sessions/sess-1/logs" {
		t.Errorf("Upload path: got %q, want %q", receivedPath, "/api/sessions/sess-1/logs")
	}

	if receivedContentType == "" {
		t.Error("Upload: missing Content-Type header")
	}

	if len(receivedBody) == 0 {
		t.Error("Upload: empty body")
	}
}

func TestHTTPUploader_Upload_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	uploader := &HTTPUploader{
		BaseURL: server.URL,
		Client:  server.Client(),
	}

	err := uploader.Upload("sess-1", []LogUpload{
		{Field: "target_log", Filename: "target.log", Content: []byte("data")},
	})

	if err == nil {
		t.Error("Upload with server error: expected error, got nil")
	}
}

func TestHTTPUploader_Upload_ConnectionError(t *testing.T) {
	uploader := &HTTPUploader{
		BaseURL: "http://127.0.0.1:0", // nothing listening
		Client:  &http.Client{},
	}

	err := uploader.Upload("sess-1", []LogUpload{
		{Field: "target_log", Filename: "target.log", Content: []byte("data")},
	})

	if err == nil {
		t.Error("Upload with connection error: expected error, got nil")
	}
}
