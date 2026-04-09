package containers

import (
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerClient abstracts the Docker Engine SDK methods used by the manager.
type ContainerClient interface {
	// Container management
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, containerID string) (containerJSON container.InspectResponse, err error)
	// Image management
	ImageInspectWithRaw(ctx context.Context, imageName string) (image.InspectResponse, []byte, error)
	ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	ImageRemove(ctx context.Context, imageName string, options image.RemoveOptions) ([]image.DeleteResponse, error)
}

// ContainerConfig holds the parameters needed to create a container.
type ContainerConfig struct {
	// Container name.
	Name string
	// Container image and tag, e.g. "ubuntu:latest".
	Image string
	// Environment variables in "KEY=VALUE" format.
	Env []string
	// Command to run in the container. If empty, the image's default CMD is used.
	Cmd []string
	// Volumes to mount, mapping host paths to container paths.
	Volumes map[string]string
	// Flags to control container behavior (e.g. privileged, network mode).
	Flags map[string]any
	// Labels to attach to the container.
	Labels map[string]string
}

// ContainerInfo describes a container's current state.
type ContainerInfo struct {
	// Unique identifier of the container.
	ID string
	// Human-readable name of the container.
	Name string
	// Image the container was created from, e.g. "ubuntu:latest".
	Image string
	// Current status of the container, e.g. "running", "exited".
	Status string
	// Exited indicates whether the container has finished execution.
	Exited bool
	// Exit code if the container has finished.
	ExitCode int
	// Creation timestamp of the container.
	CreatedAt time.Time
	// Labels associated with the container.
	Labels map[string]string
}
