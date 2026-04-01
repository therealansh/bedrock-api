package containers

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Label applied to every container created by this manager so that List can
// filter them from unrelated containers on the same Docker host.
const (
	labelManagedBy = "bedrock.managed-by"
	labelValue     = "bedrock-dockerd"
)

// dockerClient abstracts the Docker Engine SDK methods used by the manager.
// The concrete *client.Client satisfies this interface.
type dockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
}

// dockerManager implements ContainerManager using the Docker Engine API.
type dockerManager struct {
	client dockerClient
}

// NewDockerManager returns a ContainerManager backed by the given Docker client.
func NewDockerManager(cli *client.Client) ContainerManager {
	return &dockerManager{client: cli}
}

// Create pulls together the container configuration from cfg, creates the
// container on the Docker host, and starts it. If start fails the container
// is removed on a best-effort basis so we don't leak resources.
func (m *dockerManager) Create(ctx context.Context, cfg ContainerConfig) (string, error) {
	var mounts []mount.Mount
	for hostPath, containerPath := range cfg.Volumes {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: hostPath,
			Target: containerPath,
		})
	}

	resp, err := m.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: cfg.Image,
			Env:   cfg.Env,
			Cmd:   cfg.Cmd,
			Labels: map[string]string{
				labelManagedBy: labelValue,
			},
		},
		&container.HostConfig{
			Mounts: mounts,
		},
		nil, nil, cfg.Name,
	)
	if err != nil {
		return "", err
	}

	if err := m.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = m.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})
		return "", err
	}

	return resp.ID, nil
}

// StoreLogs fetches the stdout and stderr streams of a container and writes
// them to filePath. The Docker multiplexed log format is decoded before writing.
func (m *dockerManager) StoreLogs(ctx context.Context, containerID string, filePath string) error {
	reader, err := m.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return err
	}
	defer reader.Close()

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = stdcopy.StdCopy(f, f, reader)
	return err
}

// List returns information about every container that carries the bedrock
// managed-by label, regardless of whether it is running or stopped.
func (m *dockerManager) List(ctx context.Context) ([]ContainerInfo, error) {
	raw, err := m.client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", labelManagedBy+"="+labelValue),
		),
	})
	if err != nil {
		return nil, err
	}

	infos := make([]ContainerInfo, 0, len(raw))
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		infos = append(infos, ContainerInfo{
			ID:      c.ID,
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			Running: c.State == "running",
		})
	}

	return infos, nil
}

// Stop stops a running container.
func (m *dockerManager) Stop(ctx context.Context, containerID string) error {
	return m.client.ContainerStop(ctx, containerID, container.StopOptions{})
}

// Remove removes a container.
func (m *dockerManager) Remove(ctx context.Context, containerID string) error {
	return m.client.ContainerRemove(ctx, containerID, container.RemoveOptions{})
}
