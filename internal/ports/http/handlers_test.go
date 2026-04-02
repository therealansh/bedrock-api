package http

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/components/logs"
	"github.com/amirhnajafiz/bedrock-api/internal/storage/gocache"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"go.uber.org/zap"
)

func newTestServer() HTTPServer {
	return HTTPServer{
		Logr:     zap.NewNop(),
		logStore: logs.NewLogStore(gocache.NewBackend(time.Minute)),
	}
}

func TestStoreSessionLogs(t *testing.T) {
	h := newTestServer()

	rec := echotest.ContextConfig{
		MultipartForm: &echotest.MultipartForm{
			Files: []echotest.MultipartFormFile{
				{Fieldname: "target_log", Filename: "target.log", Content: []byte("target-data")},
				{Fieldname: "tracer_log", Filename: "tracer.log", Content: []byte("tracer-data")},
				{Fieldname: "vfs_pdf", Filename: "vfs.pdf", Content: []byte("pdf-data")},
			},
		},
		PathValues: echo.PathValues{
			{Name: "id", Value: "sess-1"},
		},
	}.ServeWithHandler(t, h.storeSessionLogs)

	if rec.Code != http.StatusCreated {
		t.Errorf("POST logs: got status %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// Verify files were stored.
	entries, err := h.logStore.ListLogs("sess-1")
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("ListLogs: got %d entries, want 3", len(entries))
	}
}

func TestStoreSessionLogs_PartialUpload(t *testing.T) {
	h := newTestServer()

	rec := echotest.ContextConfig{
		MultipartForm: &echotest.MultipartForm{
			Files: []echotest.MultipartFormFile{
				{Fieldname: "target_log", Filename: "target.log", Content: []byte("only-target")},
			},
		},
		PathValues: echo.PathValues{
			{Name: "id", Value: "sess-2"},
		},
	}.ServeWithHandler(t, h.storeSessionLogs)

	if rec.Code != http.StatusCreated {
		t.Errorf("POST partial logs: got status %d, want %d", rec.Code, http.StatusCreated)
	}

	entries, err := h.logStore.ListLogs("sess-2")
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("ListLogs partial: got %d entries, want 1", len(entries))
	}
}

func TestStoreSessionLogs_NoFiles(t *testing.T) {
	h := newTestServer()

	rec := echotest.ContextConfig{
		MultipartForm: &echotest.MultipartForm{},
		PathValues: echo.PathValues{
			{Name: "id", Value: "sess-3"},
		},
	}.ServeWithHandler(t, h.storeSessionLogs)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST no files: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStoreSessionLogs_MissingSessionID(t *testing.T) {
	h := newTestServer()

	rec := echotest.ContextConfig{
		MultipartForm: &echotest.MultipartForm{
			Files: []echotest.MultipartFormFile{
				{Fieldname: "target_log", Filename: "target.log", Content: []byte("data")},
			},
		},
	}.ServeWithHandler(t, h.storeSessionLogs)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST missing id: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetSessionLogs(t *testing.T) {
	h := newTestServer()

	// Pre-populate logs.
	_ = h.logStore.SaveLog("sess-1", "target.log", []byte("target-data"))
	_ = h.logStore.SaveLog("sess-1", "tracer.log", []byte("tracer-data"))
	_ = h.logStore.SaveLog("sess-1", "vfs.pdf", []byte("pdf-data"))

	rec := echotest.ContextConfig{
		PathValues: echo.PathValues{
			{Name: "id", Value: "sess-1"},
		},
	}.ServeWithHandler(t, h.getSessionLogs)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET logs: got status %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp SessionLogsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GET logs: failed to unmarshal response: %v", err)
	}

	if resp.SessionID != "sess-1" {
		t.Errorf("GET logs: session_id = %q, want %q", resp.SessionID, "sess-1")
	}

	if len(resp.Files) != 3 {
		t.Fatalf("GET logs: got %d files, want 3", len(resp.Files))
	}

	want := map[string]string{
		"target.log": "target-data",
		"tracer.log": "tracer-data",
		"vfs.pdf":    "pdf-data",
	}
	for _, f := range resp.Files {
		if want[f.Filename] != string(f.Content) {
			t.Errorf("GET logs file %s: got %q, want %q", f.Filename, f.Content, want[f.Filename])
		}
	}
}

func TestGetSessionLogs_NotFound(t *testing.T) {
	h := newTestServer()

	rec := echotest.ContextConfig{
		PathValues: echo.PathValues{
			{Name: "id", Value: "nonexistent"},
		},
	}.ServeWithHandler(t, h.getSessionLogs)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET missing logs: got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetSessionLogs_MissingSessionID(t *testing.T) {
	h := newTestServer()

	rec := echotest.ContextConfig{}.ServeWithHandler(t, h.getSessionLogs)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("GET missing id: got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStoreAndGetSessionLogs_Integration(t *testing.T) {
	h := newTestServer()

	// Upload logs via POST handler.
	echotest.ContextConfig{
		MultipartForm: &echotest.MultipartForm{
			Files: []echotest.MultipartFormFile{
				{Fieldname: "target_log", Filename: "target.log", Content: []byte("integrated-target")},
				{Fieldname: "tracer_log", Filename: "tracer.log", Content: []byte("integrated-tracer")},
				{Fieldname: "vfs_pdf", Filename: "vfs.pdf", Content: []byte("integrated-pdf")},
			},
		},
		PathValues: echo.PathValues{
			{Name: "id", Value: "sess-int"},
		},
	}.ServeWithHandler(t, h.storeSessionLogs)

	// Retrieve logs via GET handler.
	rec := echotest.ContextConfig{
		PathValues: echo.PathValues{
			{Name: "id", Value: "sess-int"},
		},
	}.ServeWithHandler(t, h.getSessionLogs)

	if rec.Code != http.StatusOK {
		t.Fatalf("integration GET: got status %d, want %d", rec.Code, http.StatusOK)
	}

	var resp SessionLogsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("integration GET: unmarshal: %v", err)
	}

	want := map[string]string{
		"target.log": "integrated-target",
		"tracer.log": "integrated-tracer",
		"vfs.pdf":    "integrated-pdf",
	}
	for _, f := range resp.Files {
		if want[f.Filename] != string(f.Content) {
			t.Errorf("integration file %s: got %q, want %q", f.Filename, f.Content, want[f.Filename])
		}
	}
}
