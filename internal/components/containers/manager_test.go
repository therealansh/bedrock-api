package containers

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestDockerManager_Start(t *testing.T) {
	t.Run("creates and starts container from config", func(t *testing.T) {
		var (
			capturedConfig *container.Config
			capturedHost   *container.HostConfig
			capturedName   string
			startedID      string
			pullCalled     bool
		)

		mock := &mockDockerClient{
			imageCheckFn: func(_ context.Context, imageName string) (image.InspectResponse, []byte, error) {
				if imageName != "alpine:latest" {
					t.Fatalf("inspected image = %q, want %q", imageName, "alpine:latest")
				}
				return image.InspectResponse{}, nil, nil
			},
			imagePullFn: func(_ context.Context, _ string, _ image.PullOptions) (io.ReadCloser, error) {
				pullCalled = true
				return io.NopCloser(strings.NewReader("")), nil
			},
			createFn: func(_ context.Context, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, name string) (container.CreateResponse, error) {
				capturedConfig = config
				capturedHost = hostConfig
				capturedName = name
				return container.CreateResponse{ID: "abc123"}, nil
			},
			startFn: func(_ context.Context, containerID string, _ container.StartOptions) error {
				startedID = containerID
				return nil
			},
		}

		mgr := &dockerManager{client: mock}

		id, err := mgr.Start(context.Background(), &ContainerConfig{
			Name:  "my-container",
			Image: "alpine:latest",
			Env:   []string{"FOO=bar"},
			Cmd:   []string{"echo", "hello"},
			Volumes: map[string]string{
				"/host/data": "/container/data",
			},
			Flags: map[string]any{
				"privileged": true,
				"pid":        "host",
			},
		})
		if err != nil {
			t.Fatalf("Start returned unexpected error: %v", err)
		}

		if id != "abc123" {
			t.Fatalf("container id = %q, want %q", id, "abc123")
		}
		if startedID != "abc123" {
			t.Fatalf("started id = %q, want %q", startedID, "abc123")
		}
		if pullCalled {
			t.Fatal("image pull should not be called when image already exists locally")
		}

		if capturedName != "my-container" {
			t.Errorf("container name = %q, want %q", capturedName, "my-container")
		}
		if capturedConfig.Image != "alpine:latest" {
			t.Errorf("image = %q, want %q", capturedConfig.Image, "alpine:latest")
		}
		if len(capturedConfig.Env) != 1 || capturedConfig.Env[0] != "FOO=bar" {
			t.Errorf("env = %v, want [FOO=bar]", capturedConfig.Env)
		}
		if len(capturedConfig.Cmd) != 2 || capturedConfig.Cmd[0] != "echo" || capturedConfig.Cmd[1] != "hello" {
			t.Errorf("cmd = %v, want [echo hello]", capturedConfig.Cmd)
		}
		if capturedConfig.Labels[labelKey] != labelValue {
			t.Errorf("label %q = %q, want %q", labelKey, capturedConfig.Labels[labelKey], labelValue)
		}

		if len(capturedHost.Mounts) != 1 {
			t.Fatalf("mount count = %d, want 1", len(capturedHost.Mounts))
		}
		if capturedHost.Mounts[0].Source != "/host/data" || capturedHost.Mounts[0].Target != "/container/data" {
			t.Errorf("mount = %s:%s, want /host/data:/container/data", capturedHost.Mounts[0].Source, capturedHost.Mounts[0].Target)
		}
		if !capturedHost.Privileged {
			t.Error("privileged should be true when requested")
		}
		if string(capturedHost.PidMode) != "host" {
			t.Errorf("pid mode = %q, want %q", string(capturedHost.PidMode), "host")
		}
	})

	t.Run("pulls image when it does not exist locally", func(t *testing.T) {
		pulledRef := ""

		mock := &mockDockerClient{
			imageCheckFn: func(_ context.Context, _ string) (image.InspectResponse, []byte, error) {
				return image.InspectResponse{}, nil, cerrdefs.ErrNotFound
			},
			imagePullFn: func(_ context.Context, ref string, _ image.PullOptions) (io.ReadCloser, error) {
				pulledRef = ref

				// docker pull streams are newline-delimited JSON objects.
				return io.NopCloser(strings.NewReader("{\"status\":\"Pulling from library/busybox\"}\n{\"status\":\"Digest: sha256:abc\"}\n{\"status\":\"Downloaded newer image\"}\n")), nil
			},
			createFn: func(_ context.Context, _ *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
				return container.CreateResponse{ID: "id1"}, nil
			},
		}

		mgr := &dockerManager{client: mock}

		_, err := mgr.Start(context.Background(), &ContainerConfig{Name: "simple", Image: "busybox:latest"})
		if err != nil {
			t.Fatalf("Start returned unexpected error: %v", err)
		}

		if pulledRef != "busybox:latest" {
			t.Fatalf("pulled ref = %q, want %q", pulledRef, "busybox:latest")
		}
	})

	t.Run("without volumes uses empty mounts", func(t *testing.T) {
		mock := &mockDockerClient{
			createFn: func(_ context.Context, _ *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, _ string) (container.CreateResponse, error) {
				if len(hostConfig.Mounts) != 0 {
					t.Errorf("mount count = %d, want 0", len(hostConfig.Mounts))
				}
				return container.CreateResponse{ID: "id1"}, nil
			},
		}

		mgr := &dockerManager{client: mock}

		_, err := mgr.Start(context.Background(), &ContainerConfig{Name: "simple", Image: "alpine"})
		if err != nil {
			t.Fatalf("Start returned unexpected error: %v", err)
		}
	})

	t.Run("create failure is wrapped", func(t *testing.T) {
		errCreate := errors.New("image not found")
		mock := &mockDockerClient{
			createFn: func(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *ocispec.Platform, string) (container.CreateResponse, error) {
				return container.CreateResponse{}, errCreate
			},
		}

		mgr := &dockerManager{client: mock}

		_, err := mgr.Start(context.Background(), &ContainerConfig{Image: "missing:latest"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, errCreate) {
			t.Fatalf("expected wrapped create error, got %v", err)
		}
		if !strings.Contains(err.Error(), "failed to create container") {
			t.Fatalf("expected contextual create error message, got %q", err.Error())
		}
	})

	t.Run("start failure triggers cleanup remove", func(t *testing.T) {
		removeCalled := false
		removedID := ""
		errStart := errors.New("port conflict")

		mock := &mockDockerClient{
			createFn: func(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *ocispec.Platform, string) (container.CreateResponse, error) {
				return container.CreateResponse{ID: "created-id"}, nil
			},
			startFn: func(_ context.Context, _ string, _ container.StartOptions) error {
				return errStart
			},
			removeFn: func(_ context.Context, containerID string, _ container.RemoveOptions) error {
				removeCalled = true
				removedID = containerID
				return nil
			},
		}

		mgr := &dockerManager{client: mock}

		_, err := mgr.Start(context.Background(), &ContainerConfig{Image: "alpine"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, errStart) {
			t.Fatalf("expected wrapped start error, got %v", err)
		}
		if !removeCalled {
			t.Fatal("expected cleanup remove call")
		}
		if removedID != "created-id" {
			t.Fatalf("removed id = %q, want %q", removedID, "created-id")
		}
	})
}

func TestDockerManager_StoreLogs(t *testing.T) {
	t.Run("writes demuxed logs to destination file", func(t *testing.T) {
		var capturedOpts container.LogsOptions

		mock := &mockDockerClient{
			logsFn: func(_ context.Context, _ string, opts container.LogsOptions) (io.ReadCloser, error) {
				capturedOpts = opts
				return newMockLogReader("hello from container"), nil
			},
		}

		mgr := &dockerManager{client: mock}
		dir := t.TempDir()
		logPath := filepath.Join(dir, "container.log")

		err := mgr.StoreLogs(context.Background(), "cid", logPath)
		if err != nil {
			t.Fatalf("StoreLogs returned unexpected error: %v", err)
		}

		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("failed to read log file: %v", err)
		}
		if string(data) != "hello from container" {
			t.Fatalf("log content = %q, want %q", string(data), "hello from container")
		}
		if !capturedOpts.ShowStdout || !capturedOpts.ShowStderr {
			t.Fatalf("expected ShowStdout and ShowStderr to be true, got %+v", capturedOpts)
		}
	})

	t.Run("logs retrieval failure is wrapped", func(t *testing.T) {
		errLogs := errors.New("container not found")
		mock := &mockDockerClient{
			logsFn: func(context.Context, string, container.LogsOptions) (io.ReadCloser, error) {
				return nil, errLogs
			},
		}

		mgr := &dockerManager{client: mock}

		err := mgr.StoreLogs(context.Background(), "bad-id", "/tmp/nope.log")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, errLogs) {
			t.Fatalf("expected wrapped logs error, got %v", err)
		}
	})

	t.Run("file creation failure is returned", func(t *testing.T) {
		mock := &mockDockerClient{
			logsFn: func(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
				return newMockLogReader("data"), nil
			},
		}

		mgr := &dockerManager{client: mock}

		err := mgr.StoreLogs(context.Background(), "cid", "/nonexistent/dir/file.log")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to create log file") {
			t.Fatalf("expected file create context in error, got %q", err.Error())
		}
	})
}

func TestDockerManager_List(t *testing.T) {
	created1 := "2026-01-01T00:00:00Z"
	created2 := "2026-01-01T01:00:00Z"
	expectedCreated1, _ := time.Parse(time.RFC3339, created1)
	expectedCreated2, _ := time.Parse(time.RFC3339, created2)

	t.Run("returns normalized container info", func(t *testing.T) {
		mock := &mockDockerClient{
			listFn: func(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{
					{ID: "id1", Names: []string{"/tracer-1"}, Image: "tracer:latest", State: "running", Status: "Up 5 minutes"},
					{ID: "id2", Names: []string{"/target-1"}, Image: "target:latest", State: "exited", Status: "Exited (0) 2 minutes ago"},
				}, nil
			},
			inspectFn: func(_ context.Context, containerID string) (container.InspectResponse, error) {
				switch containerID {
				case "id1":
					return container.InspectResponse{
						ContainerJSONBase: &container.ContainerJSONBase{
							State:   &container.State{Running: true},
							Created: created1,
						},
					}, nil
				case "id2":
					return container.InspectResponse{
						ContainerJSONBase: &container.ContainerJSONBase{
							State:   &container.State{Running: false, ExitCode: 0},
							Created: created2,
						},
					}, nil
				default:
					return container.InspectResponse{}, errors.New("not found")
				}
			},
		}

		mgr := &dockerManager{client: mock}

		infos, err := mgr.List(context.Background())
		if err != nil {
			t.Fatalf("List returned unexpected error: %v", err)
		}
		if len(infos) != 2 {
			t.Fatalf("container count = %d, want 2", len(infos))
		}

		if infos[0].ID != "id1" || infos[0].Name != "tracer-1" || infos[0].Image != "tracer:latest" || infos[0].Exited {
			t.Errorf("infos[0] = %+v", infos[0])
		}
		if !infos[0].CreatedAt.Equal(expectedCreated1) {
			t.Errorf("infos[0].CreatedAt = %v, want %v", infos[0].CreatedAt, expectedCreated1)
		}

		if infos[1].ID != "id2" || infos[1].Name != "target-1" || !infos[1].Exited || infos[1].ExitCode != 0 {
			t.Errorf("infos[1] = %+v", infos[1])
		}
		if !infos[1].CreatedAt.Equal(expectedCreated2) {
			t.Errorf("infos[1].CreatedAt = %v, want %v", infos[1].CreatedAt, expectedCreated2)
		}
	})

	t.Run("returns empty list when no containers", func(t *testing.T) {
		mock := &mockDockerClient{
			listFn: func(context.Context, container.ListOptions) ([]container.Summary, error) {
				return nil, nil
			},
		}

		mgr := &dockerManager{client: mock}

		infos, err := mgr.List(context.Background())
		if err != nil {
			t.Fatalf("List returned unexpected error: %v", err)
		}
		if len(infos) != 0 {
			t.Fatalf("container count = %d, want 0", len(infos))
		}
	})

	t.Run("applies managed label filter", func(t *testing.T) {
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
			t.Fatal("expected All=true")
		}

		filterValues := capturedOpts.Filters.Get("label")
		expectedLabel := labelKey + "=" + labelValue
		found := false
		for _, v := range filterValues {
			if v == expectedLabel {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("label filters = %v, want %q", filterValues, expectedLabel)
		}
	})

	t.Run("list failure is wrapped", func(t *testing.T) {
		errList := errors.New("daemon unreachable")
		mock := &mockDockerClient{
			listFn: func(context.Context, container.ListOptions) ([]container.Summary, error) {
				return nil, errList
			},
		}

		mgr := &dockerManager{client: mock}

		_, err := mgr.List(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, errList) {
			t.Fatalf("expected wrapped list error, got %v", err)
		}
	})

	t.Run("inspect failure is wrapped", func(t *testing.T) {
		errInspect := errors.New("inspect failed")
		mock := &mockDockerClient{
			listFn: func(context.Context, container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{{ID: "id1"}}, nil
			},
			inspectFn: func(context.Context, string) (container.InspectResponse, error) {
				return container.InspectResponse{}, errInspect
			},
		}

		mgr := &dockerManager{client: mock}

		_, err := mgr.List(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, errInspect) {
			t.Fatalf("expected wrapped inspect error, got %v", err)
		}
	})

	t.Run("created time parsing failure is returned", func(t *testing.T) {
		mock := &mockDockerClient{
			listFn: func(context.Context, container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{{ID: "id1"}}, nil
			},
			inspectFn: func(context.Context, string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						Created: "not-a-time",
						State:   &container.State{Running: true},
					},
				}, nil
			},
		}

		mgr := &dockerManager{client: mock}

		_, err := mgr.List(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse created time") {
			t.Fatalf("expected parse-time context in error, got %q", err.Error())
		}
	})

	t.Run("missing names are normalized to empty string", func(t *testing.T) {
		mock := &mockDockerClient{
			listFn: func(context.Context, container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{{ID: "id1", Names: nil}}, nil
			},
			inspectFn: func(_ context.Context, _ string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						State:   &container.State{Running: true},
						Created: created1,
					},
				}, nil
			},
		}

		mgr := &dockerManager{client: mock}
		infos, err := mgr.List(context.Background())
		if err != nil {
			t.Fatalf("List returned unexpected error: %v", err)
		}

		if infos[0].Name != "" {
			t.Fatalf("name = %q, want empty string", infos[0].Name)
		}
	})
}

func TestDockerManager_Stop(t *testing.T) {
	t.Run("delegates to docker client", func(t *testing.T) {
		stoppedID := ""
		mock := &mockDockerClient{
			stopFn: func(_ context.Context, containerID string, _ container.StopOptions) error {
				stoppedID = containerID
				return nil
			},
		}

		mgr := &dockerManager{client: mock}

		err := mgr.Stop(context.Background(), "cid")
		if err != nil {
			t.Fatalf("Stop returned unexpected error: %v", err)
		}
		if stoppedID != "cid" {
			t.Fatalf("stopped id = %q, want %q", stoppedID, "cid")
		}
	})

	t.Run("returns client error", func(t *testing.T) {
		errStop := errors.New("no such container")
		mock := &mockDockerClient{
			stopFn: func(context.Context, string, container.StopOptions) error {
				return errStop
			},
		}

		mgr := &dockerManager{client: mock}

		err := mgr.Stop(context.Background(), "ghost")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, errStop) {
			t.Fatalf("expected stop error from client, got %v", err)
		}
	})
}

func TestDockerManager_Remove(t *testing.T) {
	t.Run("delegates to docker client", func(t *testing.T) {
		removedID := ""
		mock := &mockDockerClient{
			removeFn: func(_ context.Context, containerID string, _ container.RemoveOptions) error {
				removedID = containerID
				return nil
			},
		}

		mgr := &dockerManager{client: mock}

		err := mgr.Remove(context.Background(), "cid")
		if err != nil {
			t.Fatalf("Remove returned unexpected error: %v", err)
		}
		if removedID != "cid" {
			t.Fatalf("removed id = %q, want %q", removedID, "cid")
		}
	})

	t.Run("returns client error", func(t *testing.T) {
		errRemove := errors.New("container is running")
		mock := &mockDockerClient{
			removeFn: func(context.Context, string, container.RemoveOptions) error {
				return errRemove
			},
		}

		mgr := &dockerManager{client: mock}

		err := mgr.Remove(context.Background(), "running-cid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, errRemove) {
			t.Fatalf("expected remove error from client, got %v", err)
		}
	})
}
