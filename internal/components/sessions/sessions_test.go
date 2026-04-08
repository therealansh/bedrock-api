package sessions

import (
	"errors"
	"testing"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/storage/gocache"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"
	"github.com/amirhnajafiz/bedrock-api/pkg/xerrors"
)

// newTestSessionStore creates a SessionStore backed by an in-memory gocache instance.
func newTestSessionStore() SessionStore {
	return &sessionStore{backend: gocache.NewBackend(time.Minute)}
}

// The following test covers the basic save/get flow, ensuring that a session can be stored and retrieved correctly.
func TestSessionStore_SaveAndGet(t *testing.T) {
	s := newTestSessionStore()

	sessions := []models.Session{
		{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}},
		{Id: "s2", DockerDId: "d1", Spec: models.Spec{Image: "b"}},
		{Id: "s3", DockerDId: "d2", Spec: models.Spec{Image: "c"}},
	}

	for _, sess := range sessions {
		if err := s.SaveSession(&sess); err != nil {
			t.Fatalf("SaveSession: %v", err)
		}

		got, err := s.GetSession(sess.Id, sess.DockerDId)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}

		if string(got.Spec.Image) != string(sess.Spec.Image) {
			t.Errorf("GetSession: got %q, want %q", got, sess.Spec.Image)
		}
	}
}

// The following test covers the error handling of GetSession when the requested session does not exist, ensuring that the correct error is returned.
func TestSessionStore_Get_NotFound(t *testing.T) {
	s := newTestSessionStore()

	sessions := []models.Session{
		{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}},
		{Id: "s2", DockerDId: "d1", Spec: models.Spec{Image: "b"}},
		{Id: "s3", DockerDId: "d2", Spec: models.Spec{Image: "c"}},
	}

	for _, sess := range sessions {
		if err := s.SaveSession(&sess); err != nil {
			t.Fatalf("SaveSession: %v", err)
		}
	}

	_, err := s.GetSession("nope", "d1")
	if !errors.Is(err, xerrors.StorageErrNotFound) {
		t.Errorf("GetSession missing: got %v, want xerrors.StorageErrNotFound", err)
	}
}

// The following test covers the error handling of GetSession when the dockerdId is incorrect,
// ensuring that sessions are properly namespaced and that the correct error is returned when a session is not found under the wrong namespace.
func TestSessionStore_Get_WrongDockerdId(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}})

	_, err := s.GetSession("s1", "d2")
	if !errors.Is(err, xerrors.StorageErrNotFound) {
		t.Errorf("GetSession wrong dockerdId: got %v, want xerrors.StorageErrNotFound", err)
	}
}

// The following test covers id-only retrieval across daemon namespaces.
func TestSessionStore_GetById(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}})
	_ = s.SaveSession(&models.Session{Id: "s2", DockerDId: "d2", Spec: models.Spec{Image: "b"}})

	got, err := s.GetSessionById("s2")
	if err != nil {
		t.Fatalf("GetSessionById: %v", err)
	}

	if got.Id != "s2" || got.DockerDId != "d2" {
		t.Errorf("GetSessionById: got (%s,%s), want (s2,d2)", got.Id, got.DockerDId)
	}
}

// The following test covers id-only retrieval for unknown ids.
func TestSessionStore_GetById_NotFound(t *testing.T) {
	s := newTestSessionStore()

	_, err := s.GetSessionById("missing")
	if !errors.Is(err, xerrors.StorageErrNotFound) {
		t.Errorf("GetSessionById missing: got %v, want xerrors.StorageErrNotFound", err)
	}
}

// The following test covers the overwrite behavior of SaveSession,
// ensuring that saving a session with the same id and dockerdId updates the existing entry rather than creating a duplicate.
func TestSessionStore_Save_Overwrite(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}})
	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "b"}})

	got, err := s.GetSession("s1", "d1")
	if err != nil {
		t.Fatalf("GetSession after overwrite: %v", err)
	}

	if string(got.Spec.Image) != "b" {
		t.Errorf("GetSession after overwrite: got %q, want %q", got, "b")
	}
}

// The following test covers the delete functionality of DeleteSession, ensuring that a session can be removed and that subsequent
// retrieval attempts return the correct error.
func TestSessionStore_Delete(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}})

	if err := s.DeleteSession("s1", "d1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err := s.GetSession("s1", "d1")
	if !errors.Is(err, xerrors.StorageErrNotFound) {
		t.Errorf("GetSession after delete: got %v, want xerrors.StorageErrNotFound", err)
	}
}

// The following test covers the idempotency of DeleteSession, ensuring that attempting to delete a non-existent
// session does not result in an error and does not affect existing sessions.
func TestSessionStore_Delete_NoOp(t *testing.T) {
	s := newTestSessionStore()

	if err := s.DeleteSession("ghost", "d1"); err != nil {
		t.Errorf("DeleteSession missing: unexpected error: %v", err)
	}
}

// The following test covers the listing functionality of ListSessions, ensuring that all sessions are returned correctly.
func TestSessionStore_ListSessions(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}})
	_ = s.SaveSession(&models.Session{Id: "s2", DockerDId: "d1", Spec: models.Spec{Image: "b"}})
	_ = s.SaveSession(&models.Session{Id: "s3", DockerDId: "d2", Spec: models.Spec{Image: "c"}})

	all, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("ListSessions: got %d entries, want 3", len(all))
	}

	want := map[string]bool{"a": true, "b": true, "c": true}
	for _, v := range all {
		if !want[string(v.Spec.Image)] {
			t.Errorf("ListSessions: unexpected value %q", v)
		}
	}
}

// The following test covers the behavior of ListSessions when no sessions are stored, ensuring that it returns an empty slice without error.
func TestSessionStore_ListSessions_Empty(t *testing.T) {
	s := newTestSessionStore()

	all, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions empty: %v", err)
	}

	if len(all) != 0 {
		t.Errorf("ListSessions empty: got %d entries, want 0", len(all))
	}
}

// The following test covers the listing functionality of ListSessionsByDockerDId,
// ensuring that only sessions belonging to the specified Docker daemon are returned.
func TestSessionStore_ListSessionsByDockerDId(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}})
	_ = s.SaveSession(&models.Session{Id: "s2", DockerDId: "d1", Spec: models.Spec{Image: "b"}})
	_ = s.SaveSession(&models.Session{Id: "s3", DockerDId: "d2", Spec: models.Spec{Image: "c"}})

	d1Sessions, err := s.ListSessionsByDockerDId("d1")
	if err != nil {
		t.Fatalf("ListSessionsByDockerDId: %v", err)
	}

	if len(d1Sessions) != 2 {
		t.Errorf("ListSessionsByDockerDId d1: got %d entries, want 2", len(d1Sessions))
	}

	want := map[string]bool{"a": true, "b": true}
	for _, v := range d1Sessions {
		if !want[string(v.Spec.Image)] {
			t.Errorf("ListSessionsByDockerDId d1: unexpected value %q", v)
		}
	}
}

// The following test covers the behavior of ListSessionsByDockerDId when no sessions exist for the specified Docker daemon,
// ensuring that it returns an empty slice without error.
func TestSessionStore_ListSessionsByDockerDId_Empty(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}})

	d2Sessions, err := s.ListSessionsByDockerDId("d2")
	if err != nil {
		t.Fatalf("ListSessionsByDockerDId unknown daemon: %v", err)
	}

	if len(d2Sessions) != 0 {
		t.Errorf("ListSessionsByDockerDId unknown daemon: got %d entries, want 0", len(d2Sessions))
	}
}

// The following test covers the id-only retrieval functionality of GetSessionById across daemon namespaces,
// ensuring that a session can be retrieved by id without specifying the dockerdId and that the correct session
// is returned when multiple sessions with the same id exist under different dockerdIds.
func TestSessionStore_GetSessionById(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession(&models.Session{Id: "s1", DockerDId: "d1", Spec: models.Spec{Image: "a"}})

	session, err := s.GetSessionById("s1")
	if err != nil {
		t.Fatalf("GetSessionById: %v", err)
	}

	if session.Id != "s1" {
		t.Errorf("GetSessionById: got %q, want %q", session.Id, "s1")
	}

	session, err = s.GetSessionById("s2")
	if !errors.Is(err, xerrors.StorageErrNotFound) {
		t.Errorf("GetSessionById missing: got %v, want xerrors.StorageErrNotFound", err)
	}
}
