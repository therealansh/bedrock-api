package logs

import (
	"errors"
	"path"

	"github.com/amirhnajafiz/bedrock-api/internal/storage"
	"github.com/amirhnajafiz/bedrock-api/pkg/xerrors"
)

// logPrefix namespaces all log keys inside the shared KVStorage backend.
// Keys follow the pattern: logs/<sessionId>/<filename>
const logPrefix = "logs/"

// logStore wraps any storage.KVStorage backend and exposes log-specific operations.
type logStore struct {
	backend storage.KVStorage
}

// logKey builds the namespaced key for a log file.
func logKey(sessionID, filename string) string {
	return path.Join(logPrefix+sessionID, filename)
}

// sessionPrefix returns the key prefix for all logs of a session.
func sessionLogPrefix(sessionID string) string {
	return logPrefix + sessionID + "/"
}

func (s *logStore) SaveLog(sessionID, filename string, data []byte) error {
	return s.backend.Set(logKey(sessionID, filename), data)
}

func (s *logStore) GetLog(sessionID, filename string) ([]byte, error) {
	return s.backend.Get(logKey(sessionID, filename))
}

func (s *logStore) ListLogs(sessionID string) ([]LogEntry, error) {
	// Retrieve each well-known file individually so we preserve filenames.
	var entries []LogEntry
	for _, name := range AllLogFiles {
		data, err := s.backend.Get(logKey(sessionID, name))
		if err != nil {
			if errors.Is(err, xerrors.StorageErrNotFound) {
				continue
			}
			return nil, err
		}
		entries = append(entries, LogEntry{Filename: name, Content: data})
	}
	return entries, nil
}

func (s *logStore) DeleteLogs(sessionID string) error {
	for _, name := range AllLogFiles {
		if err := s.backend.Delete(logKey(sessionID, name)); err != nil {
			return err
		}
	}
	return nil
}
