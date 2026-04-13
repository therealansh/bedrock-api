package filemd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// mockScanner implements VolumeScanner for testing.
type mockScanner struct {
	sessions []string
	err      error
}

func (m *mockScanner) ReadyVolumes() ([]string, error) {
	return m.sessions, m.err
}

// recordingUploader records uploads for verification.
type recordingUploader struct {
	mu      sync.Mutex
	uploads map[string][]LogUpload
	err     error
}

func newRecordingUploader() *recordingUploader {
	return &recordingUploader{uploads: make(map[string][]LogUpload)}
}

func (r *recordingUploader) Upload(sessionID string, files []LogUpload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.uploads[sessionID] = files
	return nil
}

func (r *recordingUploader) get(sessionID string) []LogUpload {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.uploads[sessionID]
}

func TestDaemon_ProcessSession(t *testing.T) {
	base := t.TempDir()

	// Set up a ready volume with log files.
	sessionDir := filepath.Join(base, "sess-1")
	os.MkdirAll(sessionDir, 0o755)
	os.WriteFile(filepath.Join(sessionDir, "target.log"), []byte("target-data"), 0o644)
	os.WriteFile(filepath.Join(sessionDir, "tracer.log"), []byte("tracer-data"), 0o644)
	os.WriteFile(filepath.Join(sessionDir, "vfs.pdf"), []byte("pdf-data"), 0o644)

	uploader := newRecordingUploader()

	d := &Daemon{
		Scanner: &mockScanner{sessions: []string{"sess-1"}},
		Uploader: uploader,
		VolumePath:   base,
		PollInterval: time.Millisecond,
		Logger:       noopLogger(),
	}

	d.processOnce()

	uploads := uploader.get("sess-1")
	if len(uploads) != 3 {
		t.Fatalf("expected 3 uploads, got %d", len(uploads))
	}

	want := map[string]string{
		"target_log": "target-data",
		"tracer_log": "tracer-data",
		"vfs_pdf":    "pdf-data",
	}
	for _, u := range uploads {
		if want[u.Field] != string(u.Content) {
			t.Errorf("upload field %s: got %q, want %q", u.Field, u.Content, want[u.Field])
		}
	}

	// Verify volume was cleaned up.
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("volume directory should have been removed after upload")
	}
}

func TestDaemon_ProcessSession_PartialFiles(t *testing.T) {
	base := t.TempDir()

	sessionDir := filepath.Join(base, "sess-2")
	os.MkdirAll(sessionDir, 0o755)
	os.WriteFile(filepath.Join(sessionDir, "target.log"), []byte("only-target"), 0o644)

	uploader := newRecordingUploader()

	d := &Daemon{
		Scanner:      &mockScanner{sessions: []string{"sess-2"}},
		Uploader:     uploader,
		VolumePath:   base,
		PollInterval: time.Millisecond,
		Logger:       noopLogger(),
	}

	d.processOnce()

	uploads := uploader.get("sess-2")
	if len(uploads) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(uploads))
	}

	if uploads[0].Field != "target_log" || string(uploads[0].Content) != "only-target" {
		t.Errorf("unexpected upload: %+v", uploads[0])
	}
}

func TestDaemon_ProcessSession_NoFiles(t *testing.T) {
	base := t.TempDir()

	sessionDir := filepath.Join(base, "sess-3")
	os.MkdirAll(sessionDir, 0o755)

	uploader := newRecordingUploader()

	d := &Daemon{
		Scanner:      &mockScanner{sessions: []string{"sess-3"}},
		Uploader:     uploader,
		VolumePath:   base,
		PollInterval: time.Millisecond,
		Logger:       noopLogger(),
	}

	d.processOnce()

	uploads := uploader.get("sess-3")
	if len(uploads) != 0 {
		t.Errorf("expected 0 uploads for empty volume, got %d", len(uploads))
	}
}

// failingUploader fails a configurable number of times before succeeding.
type failingUploader struct {
	mu        sync.Mutex
	failCount int
	callCount int
	uploads   map[string][]LogUpload
}

func newFailingUploader(failCount int) *failingUploader {
	return &failingUploader{
		failCount: failCount,
		uploads:   make(map[string][]LogUpload),
	}
}

func (f *failingUploader) Upload(sessionID string, files []LogUpload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	if f.callCount <= f.failCount {
		return fmt.Errorf("upload error (attempt %d)", f.callCount)
	}
	f.uploads[sessionID] = files
	return nil
}

func (f *failingUploader) getCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callCount
}

func (f *failingUploader) get(sessionID string) []LogUpload {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.uploads[sessionID]
}

func TestDaemon_ProcessSession_RetrySuccess(t *testing.T) {
	base := t.TempDir()

	sessionDir := filepath.Join(base, "sess-retry")
	os.MkdirAll(sessionDir, 0o755)
	os.WriteFile(filepath.Join(sessionDir, "target.log"), []byte("target-data"), 0o644)

	// Fail twice, succeed on third attempt.
	uploader := newFailingUploader(2)

	d := &Daemon{
		Scanner:      &mockScanner{sessions: []string{"sess-retry"}},
		Uploader:     uploader,
		VolumePath:   base,
		PollInterval: time.Millisecond,
		Logger:       noopLogger(),
	}

	d.processOnce()

	if got := uploader.getCallCount(); got != 3 {
		t.Errorf("expected 3 upload attempts, got %d", got)
	}

	uploads := uploader.get("sess-retry")
	if len(uploads) != 1 {
		t.Fatalf("expected 1 upload after retry, got %d", len(uploads))
	}

	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("volume directory should have been removed after successful retry")
	}
}

func TestDaemon_ProcessSession_RetryExhaustion(t *testing.T) {
	base := t.TempDir()

	sessionDir := filepath.Join(base, "sess-fail")
	os.MkdirAll(sessionDir, 0o755)
	os.WriteFile(filepath.Join(sessionDir, "target.log"), []byte("target-data"), 0o644)

	// Fail all 3 attempts.
	uploader := newFailingUploader(3)

	d := &Daemon{
		Scanner:      &mockScanner{sessions: []string{"sess-fail"}},
		Uploader:     uploader,
		VolumePath:   base,
		PollInterval: time.Millisecond,
		Logger:       noopLogger(),
	}

	d.processOnce()

	if got := uploader.getCallCount(); got != 3 {
		t.Errorf("expected 3 upload attempts, got %d", got)
	}

	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Error("volume directory should NOT have been removed after retry exhaustion")
	}
}

func TestDaemon_RunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		Scanner:      &mockScanner{},
		Uploader:     newRecordingUploader(),
		VolumePath:   t.TempDir(),
		PollInterval: time.Second,
		Logger:       noopLogger(),
	}

	done := make(chan error, 1)
	go func() {
		done <- d.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run: got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}
