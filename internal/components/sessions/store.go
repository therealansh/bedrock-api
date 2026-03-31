package sessions

import (
	"github.com/amirhnajafiz/bedrock-api/internal/storage"
)

// sessionPrefix namespaces all session keys inside the shared KVStorage backend.
// Keys follow the pattern: sessions/<dockerdId>/<sessionId>
// This allows O(1) prefix scans to retrieve all sessions for a given daemon.
const (
	sessionPrefix = "sessions/"
)

// sessionStore wraps any storage.KVStorage backend and exposes session-specific
// operations. Using an interface rather than a concrete type means that any
// future backend (Redis, BadgerDB, …) can be swapped in without touching this file.
type sessionStore struct {
	backend storage.KVStorage
}

// sessionKey builds the namespaced key for a session.
func sessionKey(dockerdId, id string) string {
	return sessionPrefix + dockerdId + "/" + id
}

// SaveSession persists raw session bytes under id, namespaced by dockerdId.
func (s *sessionStore) SaveSession(id, dockerdId string, data []byte) error {
	return s.backend.Set(sessionKey(dockerdId, id), data)
}

// GetSession retrieves the raw bytes for id within the given dockerdId namespace.
// Returns storage.ErrNotFound when the session does not exist.
func (s *sessionStore) GetSession(id, dockerdId string) ([]byte, error) {
	return s.backend.Get(sessionKey(dockerdId, id))
}

// ListSessions returns the raw bytes of every stored session across all daemons.
func (s *sessionStore) ListSessions() ([][]byte, error) {
	return s.backend.List(sessionPrefix)
}

// ListSessionsByDockerDId returns the raw bytes of every session belonging to
// the given Docker daemon instance.
func (s *sessionStore) ListSessionsByDockerDId(dockerdId string) ([][]byte, error) {
	return s.backend.List(sessionPrefix + dockerdId + "/")
}

// DeleteSession removes the session for id within the given dockerdId namespace.
// It is a no-op when the entry is unknown.
func (s *sessionStore) DeleteSession(id, dockerdId string) error {
	return s.backend.Delete(sessionKey(dockerdId, id))
}
