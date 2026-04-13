package filemd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFSVolumeScanner_ReadyVolumes(t *testing.T) {
	base := t.TempDir()

	// Create a locked volume (should be skipped).
	locked := filepath.Join(base, "sess-locked")
	os.MkdirAll(locked, 0o755)
	os.WriteFile(filepath.Join(locked, LockFileName), nil, 0o644)
	os.WriteFile(filepath.Join(locked, "target.log"), []byte("data"), 0o644)

	// Create an unlocked volume with log files (should be detected).
	ready := filepath.Join(base, "sess-ready")
	os.MkdirAll(ready, 0o755)
	os.WriteFile(filepath.Join(ready, "target.log"), []byte("data"), 0o644)
	os.WriteFile(filepath.Join(ready, "tracer.log"), []byte("data"), 0o644)

	// Create an unlocked volume with no log files (should be skipped).
	empty := filepath.Join(base, "sess-empty")
	os.MkdirAll(empty, 0o755)

	// Create a regular file in base (not a directory, should be skipped).
	os.WriteFile(filepath.Join(base, "stray-file"), []byte("x"), 0o644)

	scanner := &FSVolumeScanner{
		BasePath:      base,
		ExpectedFiles: []string{"target.log", "tracer.log", "vfs.pdf"},
	}

	sessions, err := scanner.ReadyVolumes()
	if err != nil {
		t.Fatalf("ReadyVolumes: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("ReadyVolumes: got %d sessions, want 1; sessions=%v", len(sessions), sessions)
	}

	if sessions[0] != "sess-ready" {
		t.Errorf("ReadyVolumes: got %q, want %q", sessions[0], "sess-ready")
	}
}

func TestFSVolumeScanner_NonexistentBase(t *testing.T) {
	scanner := &FSVolumeScanner{
		BasePath:      filepath.Join(t.TempDir(), "does-not-exist"),
		ExpectedFiles: []string{"target.log"},
	}

	sessions, err := scanner.ReadyVolumes()
	if err != nil {
		t.Fatalf("ReadyVolumes on missing dir: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("ReadyVolumes on missing dir: got %d, want 0", len(sessions))
	}
}

func TestReadLogFile(t *testing.T) {
	base := t.TempDir()
	sessionDir := filepath.Join(base, "sess-1")
	os.MkdirAll(sessionDir, 0o755)
	os.WriteFile(filepath.Join(sessionDir, "target.log"), []byte("hello"), 0o644)

	data, err := ReadLogFile(base, "sess-1", "target.log")
	if err != nil {
		t.Fatalf("ReadLogFile: %v", err)
	}

	if string(data) != "hello" {
		t.Errorf("ReadLogFile: got %q, want %q", data, "hello")
	}
}

func TestReadLogFile_NotFound(t *testing.T) {
	base := t.TempDir()

	_, err := ReadLogFile(base, "no-session", "target.log")
	if err == nil {
		t.Error("ReadLogFile: expected error for missing file")
	}
}

func TestRemoveVolume(t *testing.T) {
	base := t.TempDir()
	sessionDir := filepath.Join(base, "sess-1")
	os.MkdirAll(sessionDir, 0o755)
	os.WriteFile(filepath.Join(sessionDir, "target.log"), []byte("data"), 0o644)

	if err := RemoveVolume(base, "sess-1"); err != nil {
		t.Fatalf("RemoveVolume: %v", err)
	}

	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Error("RemoveVolume: directory still exists")
	}
}
