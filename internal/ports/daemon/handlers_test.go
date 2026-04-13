package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/amirhnajafiz/bedrock-api/pkg/models"
	"go.uber.org/zap"
)

// stubContainerManager records calls to ContainerManager methods.
type stubContainerManager struct {
	createIDs      map[string]string
	listResult     []*models.ContainerInfo
	getResult      map[string]*models.ContainerInfo
	storeLogsCalls []storeLogsCall
}

type storeLogsCall struct {
	ContainerID string
	FilePath    string
}

func newStubContainerManager() *stubContainerManager {
	return &stubContainerManager{
		createIDs: make(map[string]string),
		getResult: make(map[string]*models.ContainerInfo),
	}
}

func (s *stubContainerManager) Create(_ context.Context, cfg *models.ContainerConfig) (string, error) {
	id := "id-" + cfg.Name
	s.createIDs[cfg.Name] = id
	return id, nil
}

func (s *stubContainerManager) Start(_ context.Context, _ string) error  { return nil }
func (s *stubContainerManager) Stop(_ context.Context, _ string) error   { return nil }
func (s *stubContainerManager) Remove(_ context.Context, _ string) error { return nil }

func (s *stubContainerManager) StoreLogs(_ context.Context, containerID string, filePath string) error {
	s.storeLogsCalls = append(s.storeLogsCalls, storeLogsCall{containerID, filePath})
	return os.WriteFile(filePath, []byte("log-data-"+containerID), 0644)
}

func (s *stubContainerManager) List(_ context.Context, _ map[string]string) ([]*models.ContainerInfo, error) {
	return s.listResult, nil
}

func (s *stubContainerManager) Get(_ context.Context, id string) (*models.ContainerInfo, error) {
	return s.getResult[id], nil
}

func TestStartContainersForSession_CreatesLockFile(t *testing.T) {
	datadir := t.TempDir()
	cm := newStubContainerManager()

	d := Daemon{
		ContainerManager: cm,
		Logr:             zap.NewNop(),
		datadir:          datadir,
		tracerImage:      "tracer:latest",
	}

	err := d.startContainersForSession("sess-1", models.Spec{
		Image:   "ubuntu:latest",
		Command: "echo hello",
	})
	if err != nil {
		t.Fatalf("startContainersForSession: %v", err)
	}

	lockPath := filepath.Join(datadir, "sess-1", ".lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("expected .lock file to be created, but it does not exist")
	}
}

func TestCollectSessionLogs_StoresLogsAndRemovesLock(t *testing.T) {
	datadir := t.TempDir()
	sessionID := "sess-1"

	sessionDir := filepath.Join(datadir, sessionID)
	os.MkdirAll(sessionDir, 0o755)
	os.WriteFile(filepath.Join(sessionDir, ".lock"), nil, 0o644)

	cm := newStubContainerManager()
	cm.listResult = []*models.ContainerInfo{
		{ID: "id-target", Name: "bedrock-target-sess-1", Labels: map[string]string{
			daemonContainerKey:       daemonContainerVal,
			daemonContainerType:      daemonContainerTypeTarget,
			daemonContainerSessionId: sessionID,
		}},
		{ID: "id-tracer", Name: "bedrock-tracer-sess-1", Labels: map[string]string{
			daemonContainerKey:       daemonContainerVal,
			daemonContainerType:      daemonContainerTypeTracer,
			daemonContainerSessionId: sessionID,
		}},
	}

	d := Daemon{
		ContainerManager: cm,
		Logr:             zap.NewNop(),
		datadir:          datadir,
	}

	d.collectSessionLogs(sessionID)

	if len(cm.storeLogsCalls) != 2 {
		t.Fatalf("expected 2 StoreLogs calls, got %d", len(cm.storeLogsCalls))
	}

	targetLog := filepath.Join(sessionDir, "target.log")
	if _, err := os.Stat(targetLog); os.IsNotExist(err) {
		t.Error("expected target.log to be created")
	}

	tracerLog := filepath.Join(sessionDir, "tracer.log")
	if _, err := os.Stat(tracerLog); os.IsNotExist(err) {
		t.Error("expected tracer.log to be created")
	}

	lockPath := filepath.Join(sessionDir, ".lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected .lock file to be removed after log collection")
	}
}

func TestCollectSessionLogs_RemovesLockEvenOnFailure(t *testing.T) {
	datadir := t.TempDir()
	sessionID := "sess-2"

	sessionDir := filepath.Join(datadir, sessionID)
	os.MkdirAll(sessionDir, 0o755)
	os.WriteFile(filepath.Join(sessionDir, ".lock"), nil, 0o644)

	cm := newStubContainerManager()
	cm.listResult = []*models.ContainerInfo{}

	d := Daemon{
		ContainerManager: cm,
		Logr:             zap.NewNop(),
		datadir:          datadir,
	}

	d.collectSessionLogs(sessionID)

	lockPath := filepath.Join(sessionDir, ".lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected .lock file to be removed even when no containers found")
	}
}
