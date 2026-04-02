package logs

import "github.com/amirhnajafiz/bedrock-api/internal/storage"

// Well-known log file names produced by each tracing session.
const (
	TargetLogFile = "target.log"
	TracerLogFile = "tracer.log"
	VFSPDFFile   = "vfs.pdf"
)

// AllLogFiles lists every file that FileMD is expected to upload per session.
var AllLogFiles = []string{TargetLogFile, TracerLogFile, VFSPDFFile}

// LogEntry represents a single log file associated with a session.
type LogEntry struct {
	Filename string `json:"filename"`
	Content  []byte `json:"content"`
}

// LogStore provides domain-specific access to session log files.
type LogStore interface {
	// SaveLog persists a single log file under the given session ID.
	SaveLog(sessionID, filename string, data []byte) error

	// GetLog retrieves a single log file for the given session ID.
	GetLog(sessionID, filename string) ([]byte, error)

	// ListLogs returns all log entries stored for the given session ID.
	ListLogs(sessionID string) ([]LogEntry, error)

	// DeleteLogs removes all log files for the given session ID.
	DeleteLogs(sessionID string) error
}

// NewLogStore returns a LogStore backed by the provided KVStorage.
func NewLogStore(backend storage.KVStorage) LogStore {
	return &logStore{backend: backend}
}
