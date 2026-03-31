package sessions

// SessionStore provides domain-specific access to session data.
// Sessions are stored as opaque byte slices so the storage layer remains
// agnostic of the serialisation format chosen by higher-level code.
type SessionStore interface {
	// SaveSession persists raw session bytes under the given id, namespaced by
	// the owning Docker daemon's id. Calling SaveSession with the same id and
	// dockerdId overwrites the entry.
	SaveSession(id, dockerdId string, data []byte) error
	// GetSession retrieves the raw bytes for id within the given dockerdId namespace.
	// Returns ErrNotFound when absent.
	GetSession(id, dockerdId string) ([]byte, error)
	// ListSessions returns the raw bytes of every stored session across all daemons.
	ListSessions() ([][]byte, error)
	// ListSessionsByDockerDId returns the raw bytes of every session belonging to
	// the given Docker daemon instance. Returns an empty slice when none exist.
	ListSessionsByDockerDId(dockerdId string) ([][]byte, error)
	// DeleteSession removes the session for id within the given dockerdId namespace.
	// It is a no-op when the entry is unknown.
	DeleteSession(id, dockerdId string) error
}
