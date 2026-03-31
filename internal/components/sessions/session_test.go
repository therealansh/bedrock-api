package sessions

import (
	"errors"
	"testing"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/storage"
	"github.com/amirhnajafiz/bedrock-api/internal/storage/gocache"
)

func newTestSessionStore() SessionStore {
	return &sessionStore{backend: gocache.NewBackend(time.Minute)}
}

func TestSessionStore_SaveAndGet(t *testing.T) {
	s := newTestSessionStore()

	if err := s.SaveSession("s1", "d1", []byte(`{"user":"alice"}`)); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	got, err := s.GetSession("s1", "d1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if string(got) != `{"user":"alice"}` {
		t.Errorf("GetSession: got %q", got)
	}
}

func TestSessionStore_Get_NotFound(t *testing.T) {
	s := newTestSessionStore()

	_, err := s.GetSession("nope", "d1")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetSession missing: got %v, want storage.ErrNotFound", err)
	}
}

func TestSessionStore_Get_WrongDockerdId(t *testing.T) {
	s := newTestSessionStore()

	_ = s.SaveSession("s1", "d1", []byte("v"))

	_, err := s.GetSession("s1", "d2")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetSession wrong dockerdId: got %v, want storage.ErrNotFound", err)
	}
}

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

func TestSessionStore_Delete_NoOp(t *testing.T) {
	s := newTestSessionStore()

	if err := s.DeleteSession("ghost", "d1"); err != nil {
		t.Errorf("DeleteSession missing: unexpected error: %v", err)
	}
}

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
