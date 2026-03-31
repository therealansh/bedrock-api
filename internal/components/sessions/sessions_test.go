package sessions

import (
	"errors"
	"testing"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/storage"
	"github.com/amirhnajafiz/bedrock-api/internal/storage/gocache"
)

// newTestSessionStore creates a SessionStore backed by an in-memory gocache instance.
func newTestSessionStore() SessionStore {
	return &sessionStore{backend: gocache.NewBackend(time.Minute)}
}

// The following test covers the basic save/get flow, ensuring that a session can be stored and retrieved correctly.
func TestSessionStore_SaveAndGet(t *testing.T) {
	s := newTestSessionStore()

	sessions := []struct {
		id        string
		dockerdId string
		data      []byte
	}{
		{"s1", "d1", []byte(`{"user":"alice"}`)},
		{"s2", "d1", []byte(`{"user":"bob"}`)},
		{"s3", "d2", []byte(`{"user":"carol"}`)},
		{"s3", "d2", []byte(`{"user":"carol_updated"}`)}, // same id and dockerdId as previous, should overwrite
	}

	for _, sess := range sessions {
		if err := s.SaveSession(sess.id, sess.dockerdId, sess.data); err != nil {
			t.Fatalf("SaveSession: %v", err)
		}

		got, err := s.GetSession(sess.id, sess.dockerdId)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}

		if string(got) != string(sess.data) {
			t.Errorf("GetSession: got %q, want %q", got, sess.data)
		}
	}
}

// The following test covers the error handling of GetSession when the requested session does not exist, ensuring that the correct error is returned.
func TestSessionStore_Get_NotFound(t *testing.T) {
	s := newTestSessionStore()

	sessions := []struct {
		id        string
		dockerdId string
		data      []byte
	}{
		{"s1", "d1", []byte(`{"user":"alice"}`)},
		{"s2", "d1", []byte(`{"user":"bob"}`)},
		{"s3", "d2", []byte(`{"user":"carol"}`)},
	}

	for _, sess := range sessions {
		if err := s.SaveSession(sess.id, sess.dockerdId, sess.data); err != nil {
			t.Fatalf("SaveSession: %v", err)
		}
	}

	_, err := s.GetSession("nope", "d1")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetSession missing: got %v, want storage.ErrNotFound", err)
	}
}

// The following test covers the error handling of GetSession when the dockerdId is incorrect,
// ensuring that sessions are properly namespaced and that the correct error is returned when a session is not found under the wrong namespace.
func TestSessionStore_Get_WrongDockerdId(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession("s1", "d1", []byte("v"))

	_, err := s.GetSession("s1", "d2")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetSession wrong dockerdId: got %v, want storage.ErrNotFound", err)
	}
}

// The following test covers the overwrite behavior of SaveSession,
// ensuring that saving a session with the same id and dockerdId updates the existing entry rather than creating a duplicate.
func TestSessionStore_Save_Overwrite(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession("s1", "d1", []byte("v1"))
	_ = s.SaveSession("s1", "d1", []byte("v2"))

	got, err := s.GetSession("s1", "d1")
	if err != nil {
		t.Fatalf("GetSession after overwrite: %v", err)
	}

	if string(got) != "v2" {
		t.Errorf("GetSession after overwrite: got %q, want %q", got, "v2")
	}
}

// The following test covers the delete functionality of DeleteSession, ensuring that a session can be removed and that subsequent
// retrieval attempts return the correct error.
func TestSessionStore_Delete(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession("s1", "d1", []byte("v"))

	if err := s.DeleteSession("s1", "d1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err := s.GetSession("s1", "d1")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetSession after delete: got %v, want storage.ErrNotFound", err)
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

	_ = s.SaveSession("s1", "d1", []byte("a"))
	_ = s.SaveSession("s2", "d1", []byte("b"))
	_ = s.SaveSession("s3", "d2", []byte("c"))

	all, err := s.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("ListSessions: got %d entries, want 3", len(all))
	}

	want := map[string]bool{"a": true, "b": true, "c": true}
	for _, v := range all {
		if !want[string(v)] {
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

	_ = s.SaveSession("s1", "d1", []byte("a"))
	_ = s.SaveSession("s2", "d1", []byte("b"))
	_ = s.SaveSession("s3", "d2", []byte("c"))

	d1Sessions, err := s.ListSessionsByDockerDId("d1")
	if err != nil {
		t.Fatalf("ListSessionsByDockerDId: %v", err)
	}

	if len(d1Sessions) != 2 {
		t.Errorf("ListSessionsByDockerDId d1: got %d entries, want 2", len(d1Sessions))
	}

	want := map[string]bool{"a": true, "b": true}
	for _, v := range d1Sessions {
		if !want[string(v)] {
			t.Errorf("ListSessionsByDockerDId d1: unexpected value %q", v)
		}
	}
}

// The following test covers the behavior of ListSessionsByDockerDId when no sessions exist for the specified Docker daemon,
// ensuring that it returns an empty slice without error.
func TestSessionStore_ListSessionsByDockerDId_Empty(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession("s1", "d1", []byte("a"))

	d2Sessions, err := s.ListSessionsByDockerDId("d2")
	if err != nil {
		t.Fatalf("ListSessionsByDockerDId unknown daemon: %v", err)
	}

	if len(d2Sessions) != 0 {
		t.Errorf("ListSessionsByDockerDId unknown daemon: got %d entries, want 0", len(d2Sessions))
	}
}
