package sessions

import (
	"encoding/json"

	"github.com/amirhnajafiz/bedrock-api/internal/storage"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"
)

// sessionPrefix namespaces all session keys inside the shared KVStorage backend.
// Keys follow the pattern: sessions/<dockerdId>/<sessionId>
// This allows O(1) prefix scans to retrieve all sessions for a given daemon.
const (
	sessionPrefix         = "sessions/"
	sessionIdDockerDIndex = "sessionIdDockerDIndex/"
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

// sessionIdDockerDIndexKey builds the key for the sessionId to dockerdId index entry.
func sessionIdDockerDIndexKey(id string) string {
	return sessionIdDockerDIndex + id
}

// SaveSession persists raw session bytes under id, namespaced by dockerdId.
func (s *sessionStore) SaveSession(data *models.Session) error {
	// save the session index
	if err := s.backend.Set(sessionIdDockerDIndexKey(data.Id), []byte(data.DockerDId)); err != nil {
		return err
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return s.backend.Set(sessionKey(data.DockerDId, data.Id), bytes)
}

// GetSession retrieves the raw bytes for id within the given dockerdId namespace.
// Returns storage.ErrNotFound when the session does not exist.
func (s *sessionStore) GetSession(id, dockerdId string) (*models.Session, error) {
	bytes, err := s.backend.Get(sessionKey(dockerdId, id))
	if err != nil {
		return nil, err
	}

	var session *models.Session
	if err := json.Unmarshal(bytes, &session); err != nil {
		return nil, err
	}

	return session, nil
}

// GetSessionById retrieves the session by scanning all daemon namespaces.
func (s *sessionStore) GetSessionById(id string) (*models.Session, error) {
	// look up for the dockerdId namespace in the sessionId index
	dockerdIdBytes, err := s.backend.Get(sessionIdDockerDIndexKey(id))
	if err != nil {
		return nil, err
	}
	dockerdId := string(dockerdIdBytes)

	// retrieve the session using the dockerdId namespace
	return s.GetSession(id, dockerdId)
}

// ListSessions returns the raw bytes of every stored session across all daemons.
func (s *sessionStore) ListSessions() ([]*models.Session, error) {
	bytes, err := s.backend.List(sessionPrefix)
	if err != nil {
		return nil, err
	}

	var sessions []*models.Session
	for _, b := range bytes {
		var session *models.Session
		if err := json.Unmarshal(b, &session); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// ListSessionsByDockerDId returns the raw bytes of every session belonging to
// the given Docker daemon instance.
func (s *sessionStore) ListSessionsByDockerDId(dockerdId string) ([]*models.Session, error) {
	bytes, err := s.backend.List(sessionPrefix + dockerdId + "/")
	if err != nil {
		return nil, err
	}

	var sessions []*models.Session
	for _, b := range bytes {
		var session *models.Session
		if err := json.Unmarshal(b, &session); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeleteSession removes the session for id within the given dockerdId namespace.
// It is a no-op when the entry is unknown.
func (s *sessionStore) DeleteSession(id, dockerdId string) error {
	// delete the session index
	if err := s.backend.Delete(sessionIdDockerDIndexKey(id)); err != nil {
		return err
	}

	return s.backend.Delete(sessionKey(dockerdId, id))
}
