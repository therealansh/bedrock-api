package containers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// mockDockerClient implements dockerClient for unit testing.
type mockDockerClient struct {
	createFn func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	startFn  func(ctx context.Context, containerID string, options container.StartOptions) error
	stopFn   func(ctx context.Context, containerID string, options container.StopOptions) error
	removeFn func(ctx context.Context, containerID string, options container.RemoveOptions) error
	listFn   func(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	logsFn   func(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "mock-id"}, nil
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.startFn != nil {
		return m.startFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.stopFn != nil {
		return m.stopFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.removeFn != nil {
		return m.removeFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	if m.listFn != nil {
		return m.listFn(ctx, options)
	}
	return nil, nil
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, ctr string, options container.LogsOptions) (io.ReadCloser, error) {
	if m.logsFn != nil {
		return m.logsFn(ctx, ctr, options)
	}
	return io.NopCloser(&bytes.Buffer{}), nil
}

// newMockLogReader returns an io.ReadCloser whose content is encoded in the
// Docker multiplexed stream format so that stdcopy.StdCopy can decode it.
func newMockLogReader(content string) io.ReadCloser {
	var buf bytes.Buffer
	w := stdcopy.NewStdWriter(&buf, stdcopy.Stdout)
	_, _ = w.Write([]byte(content))
	return io.NopCloser(&buf)
}

// --- Create ---

func TestCreate(t *testing.T) {
	var capturedConfig *container.Config
	var capturedHost *container.HostConfig
	var capturedName string

	mock := &mockDockerClient{
		createFn: func(_ context.Context, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, name string) (container.CreateResponse, error) {
			capturedConfig = config
			capturedHost = hostConfig
			capturedName = name
			return container.CreateResponse{ID: "abc123"}, nil
		},
	}

	mgr := &dockerManager{client: mock}

	id, err := mgr.Create(context.Background(), ContainerConfig{
		Name:  "my-container",
		Image: "alpine:latest",
		Env:   []string{"FOO=bar"},
		Cmd:   []string{"echo", "hello"},
		Volumes: map[string]string{
			"/host/data": "/container/data",
		},
	})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if id != "abc123" {
		t.Errorf("Create: got id %q, want %q", id, "abc123")
	}

	if capturedName != "my-container" {
		t.Errorf("Create: container name = %q, want %q", capturedName, "my-container")
	}
	if capturedConfig.Image != "alpine:latest" {
		t.Errorf("Create: image = %q, want %q", capturedConfig.Image, "alpine:latest")
	}
	if len(capturedConfig.Env) != 1 || capturedConfig.Env[0] != "FOO=bar" {
		t.Errorf("Create: env = %v, want [FOO=bar]", capturedConfig.Env)
	}
	if len(capturedConfig.Cmd) != 2 || capturedConfig.Cmd[0] != "echo" {
		t.Errorf("Create: cmd = %v, want [echo hello]", capturedConfig.Cmd)
	}
	if capturedConfig.Labels[labelManagedBy] != labelValue {
		t.Errorf("Create: label %q = %q, want %q", labelManagedBy, capturedConfig.Labels[labelManagedBy], labelValue)
	}
	if len(capturedHost.Mounts) != 1 {
		t.Fatalf("Create: got %d mounts, want 1", len(capturedHost.Mounts))
	}
	if capturedHost.Mounts[0].Source != "/host/data" || capturedHost.Mounts[0].Target != "/container/data" {
		t.Errorf("Create: mount = %s:%s, want /host/data:/container/data",
			capturedHost.Mounts[0].Source, capturedHost.Mounts[0].Target)
	}
}

func TestCreate_NoVolumes(t *testing.T) {
	mock := &mockDockerClient{
		createFn: func(_ context.Context, _ *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
			if len(hostConfig.Mounts) != 0 {
				t.Errorf("Create no volumes: got %d mounts, want 0", len(hostConfig.Mounts))
			}
			return container.CreateResponse{ID: "id1"}, nil
		},
	}

	mgr := &dockerManager{client: mock}

	_, err := mgr.Create(context.Background(), ContainerConfig{
		Name:  "simple",
		Image: "alpine",
	})
	if err != nil {
		t.Fatalf("Create no volumes: unexpected error: %v", err)
	}
}

func TestCreate_CreateError(t *testing.T) {
	mock := &mockDockerClient{
		createFn: func(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *ocispec.Platform, string) (container.CreateResponse, error) {
			return container.CreateResponse{}, errors.New("image not found")
		},
	}

	mgr := &dockerManager{client: mock}

	_, err := mgr.Create(context.Background(), ContainerConfig{Image: "missing:latest"})
	if err == nil {
		t.Fatal("Create with bad image: expected error, got nil")
	}
	if err.Error() != "image not found" {
		t.Errorf("Create with bad image: got %q, want %q", err.Error(), "image not found")
	}
}

func TestCreate_StartErrorCleansUp(t *testing.T) {
	removeCalled := false
	removedID := ""

	mock := &mockDockerClient{
		createFn: func(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *ocispec.Platform, string) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "created-id"}, nil
		},
		startFn: func(_ context.Context, _ string, _ container.StartOptions) error {
			return errors.New("port conflict")
		},
		removeFn: func(_ context.Context, containerID string, _ container.RemoveOptions) error {
			removeCalled = true
			removedID = containerID
			return nil
		},
	}

	mgr := &dockerManager{client: mock}

	_, err := mgr.Create(context.Background(), ContainerConfig{Image: "alpine"})
	if err == nil {
		t.Fatal("Create with start failure: expected error, got nil")
	}

	if !removeCalled {
		t.Error("Create with start failure: expected cleanup Remove call")
	}
	if removedID != "created-id" {
		t.Errorf("Create with start failure: removed %q, want %q", removedID, "created-id")
	}
}

// --- StoreLogs ---

func TestStoreLogs(t *testing.T) {
	mock := &mockDockerClient{
		logsFn: func(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
			return newMockLogReader("hello from container"), nil
		},
	}

	mgr := &dockerManager{client: mock}
	dir := t.TempDir()
	filePath := filepath.Join(dir, "container.log")

	if err := mgr.StoreLogs(context.Background(), "cid", filePath); err != nil {
		t.Fatalf("StoreLogs: unexpected error: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("StoreLogs: failed to read log file: %v", err)
	}

	if string(data) != "hello from container" {
		t.Errorf("StoreLogs: got %q, want %q", string(data), "hello from container")
	}
}

func TestStoreLogs_LogsError(t *testing.T) {
	mock := &mockDockerClient{
		logsFn: func(context.Context, string, container.LogsOptions) (io.ReadCloser, error) {
			return nil, errors.New("container not found")
		},
	}

	mgr := &dockerManager{client: mock}

	err := mgr.StoreLogs(context.Background(), "bad-id", "/tmp/nope.log")
	if err == nil {
		t.Fatal("StoreLogs with bad container: expected error, got nil")
	}
}

func TestStoreLogs_FileError(t *testing.T) {
	mock := &mockDockerClient{
		logsFn: func(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
			return newMockLogReader("data"), nil
		},
	}

	mgr := &dockerManager{client: mock}

	err := mgr.StoreLogs(context.Background(), "cid", "/nonexistent/dir/file.log")
	if err == nil {
		t.Fatal("StoreLogs with bad path: expected error, got nil")
	}
}

func TestStoreLogs_RequestsStdoutAndStderr(t *testing.T) {
	var capturedOpts container.LogsOptions

	mock := &mockDockerClient{
		logsFn: func(_ context.Context, _ string, opts container.LogsOptions) (io.ReadCloser, error) {
			capturedOpts = opts
			return newMockLogReader(""), nil
		},
	}

	mgr := &dockerManager{client: mock}
	dir := t.TempDir()

	_ = mgr.StoreLogs(context.Background(), "cid", filepath.Join(dir, "out.log"))

	if !capturedOpts.ShowStdout {
		t.Error("StoreLogs: ShowStdout should be true")
	}
	if !capturedOpts.ShowStderr {
		t.Error("StoreLogs: ShowStderr should be true")
	}
}

// --- List ---

func TestList(t *testing.T) {
	mock := &mockDockerClient{
		listFn: func(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
			return []container.Summary{
				{ID: "id1", Names: []string{"/tracer-1"}, Image: "tracer:latest", State: "running", Status: "Up 5 minutes"},
				{ID: "id2", Names: []string{"/target-1"}, Image: "target:latest", State: "exited", Status: "Exited (0) 2 minutes ago"},
			}, nil
		},
	}

	mgr := &dockerManager{client: mock}

	infos, err := mgr.List(context.Background())
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("List: got %d containers, want 2", len(infos))
	}

	if infos[0].ID != "id1" || infos[0].Name != "tracer-1" || infos[0].Image != "tracer:latest" || !infos[0].Running {
		t.Errorf("List[0]: got %+v", infos[0])
	}

	if infos[1].ID != "id2" || infos[1].Name != "target-1" || infos[1].Running {
		t.Errorf("List[1]: got %+v", infos[1])
	}
}

func TestList_Empty(t *testing.T) {
	mock := &mockDockerClient{
		listFn: func(context.Context, container.ListOptions) ([]container.Summary, error) {
			return nil, nil
		},
	}

	mgr := &dockerManager{client: mock}

	infos, err := mgr.List(context.Background())
	if err != nil {
		t.Fatalf("List empty: unexpected error: %v", err)
	}

	if len(infos) != 0 {
		t.Errorf("List empty: got %d containers, want 0", len(infos))
	}
}

func TestList_VerifiesLabelFilter(t *testing.T) {
	var capturedOpts container.ListOptions

	mock := &mockDockerClient{
		listFn: func(_ context.Context, opts container.ListOptions) ([]container.Summary, error) {
			capturedOpts = opts
			return nil, nil
		},
	}

	mgr := &dockerManager{client: mock}
	_, _ = mgr.List(context.Background())

	if !capturedOpts.All {
		t.Error("List: expected All=true")
	}

	got := capturedOpts.Filters.Get("label")
	want := labelManagedBy + "=" + labelValue
	found := false
	for _, v := range got {
		if v == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List: label filter = %v, want %q", got, want)
	}
}

func TestList_Error(t *testing.T) {
	mock := &mockDockerClient{
		listFn: func(context.Context, container.ListOptions) ([]container.Summary, error) {
			return nil, errors.New("daemon unreachable")
		},
	}

	mgr := &dockerManager{client: mock}

	_, err := mgr.List(context.Background())
	if err == nil {
		t.Fatal("List with error: expected error, got nil")
	}
}

func TestList_NameTrimming(t *testing.T) {
	mock := &mockDockerClient{
		listFn: func(context.Context, container.ListOptions) ([]container.Summary, error) {
			return []container.Summary{
				{ID: "id1", Names: nil, State: "running"},
			}, nil
		},
	}

	mgr := &dockerManager{client: mock}

	infos, err := mgr.List(context.Background())
	if err != nil {
		t.Fatalf("List no-name: unexpected error: %v", err)
	}

	if infos[0].Name != "" {
		t.Errorf("List no-name: got name %q, want empty", infos[0].Name)
	}
}

// --- Stop ---

func TestStop(t *testing.T) {
	stoppedID := ""

	mock := &mockDockerClient{
		stopFn: func(_ context.Context, containerID string, _ container.StopOptions) error {
			stoppedID = containerID
			return nil
		},
	}

	mgr := &dockerManager{client: mock}

	if err := mgr.Stop(context.Background(), "cid"); err != nil {
		t.Fatalf("Stop: unexpected error: %v", err)
	}

	if stoppedID != "cid" {
		t.Errorf("Stop: stopped %q, want %q", stoppedID, "cid")
	}
}

func TestStop_Error(t *testing.T) {
	mock := &mockDockerClient{
		stopFn: func(context.Context, string, container.StopOptions) error {
			return errors.New("no such container")
		},
	}

	mgr := &dockerManager{client: mock}

	err := mgr.Stop(context.Background(), "ghost")
	if err == nil {
		t.Fatal("Stop missing container: expected error, got nil")
	}
}

// --- Remove ---

func TestRemove(t *testing.T) {
	removedID := ""

	mock := &mockDockerClient{
		removeFn: func(_ context.Context, containerID string, _ container.RemoveOptions) error {
			removedID = containerID
			return nil
		},
	}

	mgr := &dockerManager{client: mock}

	if err := mgr.Remove(context.Background(), "cid"); err != nil {
		t.Fatalf("Remove: unexpected error: %v", err)
	}

	if removedID != "cid" {
		t.Errorf("Remove: removed %q, want %q", removedID, "cid")
	}
}

func TestRemove_Error(t *testing.T) {
	mock := &mockDockerClient{
		removeFn: func(context.Context, string, container.RemoveOptions) error {
			return errors.New("container is running")
		},
	}

	mgr := &dockerManager{client: mock}

	err := mgr.Remove(context.Background(), "running-cid")
	if err == nil {
		t.Fatal("Remove running container: expected error, got nil")
	}
}
