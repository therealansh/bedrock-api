package filemd

import (
	"os"
	"path/filepath"
)

// LockFileName is the sentinel file that DockerD places inside a volume while
// containers are still running. Its absence signals that the volume is ready
// for log collection.
const LockFileName = ".lock"

// VolumeScanner abstracts filesystem scanning so the watcher can be tested
// without real directories.
type VolumeScanner interface {
	// ReadyVolumes returns session IDs whose volumes are unlocked and contain
	// at least one log file.
	ReadyVolumes() ([]string, error)
}

// FSVolumeScanner scans a base directory for session volume subdirectories.
// Each subdirectory is named after a session ID. A volume is considered ready
// when no .lock file is present and at least one expected log file exists.
type FSVolumeScanner struct {
	BasePath     string
	ExpectedFiles []string
}

// ReadyVolumes walks BasePath looking for directories that are unlocked.
func (s *FSVolumeScanner) ReadyVolumes() ([]string, error) {
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var ready []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		volPath := filepath.Join(s.BasePath, sessionID)

		// Skip if still locked.
		if _, err := os.Stat(filepath.Join(volPath, LockFileName)); err == nil {
			continue
		}

		// Check that at least one expected file exists.
		for _, f := range s.ExpectedFiles {
			if _, err := os.Stat(filepath.Join(volPath, f)); err == nil {
				ready = append(ready, sessionID)
				break
			}
		}
	}

	return ready, nil
}

// ReadLogFile reads the contents of a log file from a volume directory.
func ReadLogFile(basePath, sessionID, filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join(basePath, sessionID, filename))
}

// RemoveVolume deletes a session's volume directory and all its contents.
func RemoveVolume(basePath, sessionID string) error {
	return os.RemoveAll(filepath.Join(basePath, sessionID))
}
