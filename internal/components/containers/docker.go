package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/stdcopy"
)

// Label applied to every container created by this manager so that List can
// filter them from unrelated containers on the same Docker host.
const (
	labelKey   = "bedrock.managed-by"
	labelValue = "bedrock-dockerd"
)

// dockerManager implements ContainerManager using the Docker Engine API.
type dockerManager struct {
	client ContainerClient
}

// Start pulls together the container configuration from cfg, creates the
// container on the Docker host, and starts it.
func (m *dockerManager) Start(ctx context.Context, cfg *ContainerConfig) (string, error) {
	// pull only when the image is not available locally.
	if _, _, err := m.client.ImageInspectWithRaw(ctx, cfg.Image); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return "", fmt.Errorf("failed to inspect image: %w", err)
		}

		// image not found locally, attempt to pull it
		pullReader, err := m.client.ImagePull(ctx, cfg.Image, image.PullOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to pull image: %w", err)
		}
		defer pullReader.Close()

		// read the pull output to completion to ensure the image is fully pulled
		decoder := json.NewDecoder(pullReader)
		for {
			var msg map[string]any
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF {
					break
				}
				return "", fmt.Errorf("failed reading pull stream: %w", err)
			}

			if e, ok := msg["error"]; ok {
				return "", fmt.Errorf("daemon pull error: %v", e)
			}
		}
	}

	// set up volume mounts
	var mounts []mount.Mount
	for hostPath, containerPath := range cfg.Volumes {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: hostPath,
			Target: containerPath,
		})
	}

	// set up host config with mounts and flags
	hostConfig := &container.HostConfig{
		AutoRemove:    false,
		Mounts:        mounts,
		RestartPolicy: container.RestartPolicy{Name: "no"},
	}
	if privileged, ok := cfg.Flags["privileged"].(bool); ok && privileged {
		hostConfig.Privileged = true
	}
	if pidMode, ok := cfg.Flags["pid"].(string); ok {
		hostConfig.PidMode = container.PidMode(pidMode)
	}

	// create and start the container
	resp, err := m.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: cfg.Image,
			Env:   cfg.Env,
			Cmd:   cfg.Cmd,
			Labels: map[string]string{
				labelKey: labelValue,
			},
		},
		hostConfig,
		nil,
		nil,
		cfg.Name,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// start the container
	if err := m.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = m.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})
		return "", fmt.Errorf("failed to start container: %w", err)
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
		return fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer f.Close()

	_, err = stdcopy.StdCopy(f, f, reader)
	if err != nil {
		return fmt.Errorf("failed to write logs: %w", err)
	}

	return nil
}

// List returns information about every container that carries the bedrock
// managed-by label, regardless of whether it is running or stopped.
func (m *dockerManager) List(ctx context.Context) ([]*ContainerInfo, error) {
	raw, err := m.client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", labelKey+"="+labelValue),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	infos := make([]*ContainerInfo, 0, len(raw))
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		// create a container info instance
		cinfo := &ContainerInfo{
			ID:       c.ID,
			Name:     name,
			Image:    c.Image,
			Status:   c.Status,
			Exited:   false,
			ExitCode: 0,
		}

		// call ContainerInspect to get the exit code if the container has finished
		inspect, err := m.client.ContainerInspect(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container: %w", err)
		}

		if inspect.State != nil && !inspect.State.Running {
			cinfo.Exited = true
			cinfo.ExitCode = int(inspect.State.ExitCode)
		}

		// convert the inspect created time string to a timestamp
		cinfo.CreatedAt, err = time.Parse(time.RFC3339, inspect.Created)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created time: %w", err)
		}

		infos = append(infos, cinfo)
	}

	return infos, nil
}

// Get returns information about a specific container, including its exit code if it has finished.
func (m *dockerManager) Get(ctx context.Context, containerID string) (*ContainerInfo, error) {
	inspect, err := m.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	name := ""
	if len(inspect.Name) > 0 {
		name = strings.TrimPrefix(inspect.Name, "/")
	}

	// convert the inspect created time string to a timestamp
	createdAt, err := time.Parse(time.RFC3339, inspect.Created)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created time: %w", err)
	}

	cinfo := &ContainerInfo{
		ID:        inspect.ID,
		Name:      name,
		Image:     inspect.Config.Image,
		Status:    inspect.State.Status,
		Exited:    false,
		ExitCode:  0,
		CreatedAt: createdAt,
	}

	if inspect.State != nil && !inspect.State.Running {
		cinfo.Exited = true
		cinfo.ExitCode = int(inspect.State.ExitCode)
	}

	return cinfo, nil
}

// Stop stops a running container.
func (m *dockerManager) Stop(ctx context.Context, containerID string) error {
	return m.client.ContainerStop(ctx, containerID, container.StopOptions{})
}

// Remove removes a container.
func (m *dockerManager) Remove(ctx context.Context, containerID string) error {
	return m.client.ContainerRemove(ctx, containerID, container.RemoveOptions{})
}

// Inspect returns detailed information about a container, including its exit code if it has finished.
func (m *dockerManager) Inspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return m.client.ContainerInspect(ctx, containerID)
}
