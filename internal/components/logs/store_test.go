package logs

import (
	"errors"
	"testing"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/storage/gocache"
	"github.com/amirhnajafiz/bedrock-api/pkg/xerrors"
)

func newTestLogStore() LogStore {
	return &logStore{backend: gocache.NewBackend(time.Minute)}
}

func TestLogStore_SaveAndGet(t *testing.T) {
	s := newTestLogStore()

	content := []byte("target container output")
	if err := s.SaveLog("sess-1", TargetLogFile, content); err != nil {
		t.Fatalf("SaveLog: %v", err)
	}

	got, err := s.GetLog("sess-1", TargetLogFile)
	if err != nil {
		t.Fatalf("GetLog: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("GetLog: got %q, want %q", got, content)
	}
}

func TestLogStore_GetNotFound(t *testing.T) {
	s := newTestLogStore()

	_, err := s.GetLog("nonexistent", TargetLogFile)
	if !errors.Is(err, xerrors.StorageErrNotFound) {
		t.Errorf("GetLog missing: got %v, want StorageErrNotFound", err)
	}
}

func TestLogStore_SaveOverwrite(t *testing.T) {
	s := newTestLogStore()

	_ = s.SaveLog("sess-1", TargetLogFile, []byte("old"))
	_ = s.SaveLog("sess-1", TargetLogFile, []byte("new"))

	got, err := s.GetLog("sess-1", TargetLogFile)
	if err != nil {
		t.Fatalf("GetLog: %v", err)
	}

	if string(got) != "new" {
		t.Errorf("GetLog after overwrite: got %q, want %q", got, "new")
	}
}

func TestLogStore_ListLogs(t *testing.T) {
	s := newTestLogStore()

	_ = s.SaveLog("sess-1", TargetLogFile, []byte("target"))
	_ = s.SaveLog("sess-1", TracerLogFile, []byte("tracer"))
	_ = s.SaveLog("sess-1", VFSPDFFile, []byte("pdf"))

	entries, err := s.ListLogs("sess-1")
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("ListLogs: got %d entries, want 3", len(entries))
	}

	want := map[string]string{
		TargetLogFile: "target",
		TracerLogFile: "tracer",
		VFSPDFFile:    "pdf",
	}
	for _, e := range entries {
		if want[e.Filename] != string(e.Content) {
			t.Errorf("ListLogs %s: got %q, want %q", e.Filename, e.Content, want[e.Filename])
		}
	}
}

func TestLogStore_ListLogs_Partial(t *testing.T) {
	s := newTestLogStore()

	_ = s.SaveLog("sess-1", TargetLogFile, []byte("target only"))

	entries, err := s.ListLogs("sess-1")
	if err != nil {
		t.Fatalf("ListLogs: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("ListLogs partial: got %d entries, want 1", len(entries))
	}

	if entries[0].Filename != TargetLogFile {
		t.Errorf("ListLogs partial: got filename %q, want %q", entries[0].Filename, TargetLogFile)
	}
}

func TestLogStore_ListLogs_Empty(t *testing.T) {
	s := newTestLogStore()

	entries, err := s.ListLogs("nonexistent")
	if err != nil {
		t.Fatalf("ListLogs empty: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("ListLogs empty: got %d entries, want 0", len(entries))
	}
}

func TestLogStore_DeleteLogs(t *testing.T) {
	s := newTestLogStore()

	_ = s.SaveLog("sess-1", TargetLogFile, []byte("target"))
	_ = s.SaveLog("sess-1", TracerLogFile, []byte("tracer"))
	_ = s.SaveLog("sess-1", VFSPDFFile, []byte("pdf"))

	if err := s.DeleteLogs("sess-1"); err != nil {
		t.Fatalf("DeleteLogs: %v", err)
	}

	entries, err := s.ListLogs("sess-1")
	if err != nil {
		t.Fatalf("ListLogs after delete: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("ListLogs after delete: got %d entries, want 0", len(entries))
	}
}

func TestLogStore_IsolationBetweenSessions(t *testing.T) {
	s := newTestLogStore()

	_ = s.SaveLog("sess-1", TargetLogFile, []byte("s1-target"))
	_ = s.SaveLog("sess-2", TargetLogFile, []byte("s2-target"))

	got1, _ := s.GetLog("sess-1", TargetLogFile)
	got2, _ := s.GetLog("sess-2", TargetLogFile)

	if string(got1) != "s1-target" {
		t.Errorf("session isolation: sess-1 got %q", got1)
	}
	if string(got2) != "s2-target" {
		t.Errorf("session isolation: sess-2 got %q", got2)
	}
}
