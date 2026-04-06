package containers

import (
	"context"

	"github.com/docker/docker/client"
)

// ContainerManager manages Docker container lifecycles.
// Implementations must be safe for concurrent use.
type ContainerManager interface {
	// Start starts a new container. Returns the container ID.
	Start(ctx context.Context, cfg ContainerConfig) (string, error)
	// StoreLogs writes the container's stdout and stderr to the given file path.
	StoreLogs(ctx context.Context, containerID string, filePath string) error
	// List returns all containers managed by this instance.
	List(ctx context.Context) ([]ContainerInfo, error)
	// Get returns information about a specific container.
	Get(ctx context.Context, containerID string) (ContainerInfo, error)
	// Stop stops a running container.
	Stop(ctx context.Context, containerID string) error
	// Remove removes a container.
	Remove(ctx context.Context, containerID string) error
}

// NewDockerManager returns a ContainerManager backed by the Docker client.
func NewDockerManager() (ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &dockerManager{client: cli}, nil
}
