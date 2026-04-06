package containers

import (
	"bytes"
	"context"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// mockDockerClient implements dockerManager client for unit testing.
type mockDockerClient struct {
	createFn     func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	startFn      func(ctx context.Context, containerID string, options container.StartOptions) error
	stopFn       func(ctx context.Context, containerID string, options container.StopOptions) error
	removeFn     func(ctx context.Context, containerID string, options container.RemoveOptions) error
	listFn       func(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	logsFn       func(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	inspectFn    func(ctx context.Context, containerID string) (container.InspectResponse, error)
	imageCheckFn func(ctx context.Context, image string) (image.InspectResponse, []byte, error)
	imagePullFn  func(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
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

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	if m.inspectFn != nil {
		return m.inspectFn(ctx, containerID)
	}
	return container.InspectResponse{}, nil
}

func (m *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageName string) (image.InspectResponse, []byte, error) {
	if m.imageCheckFn != nil {
		return m.imageCheckFn(ctx, imageName)
	}
	return image.InspectResponse{}, nil, nil
}

func (m *mockDockerClient) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
	if m.imagePullFn != nil {
		return m.imagePullFn(ctx, refStr, options)
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
