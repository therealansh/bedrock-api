package containers

import "context"

// ContainerConfig holds the parameters needed to create a container.
type ContainerConfig struct {
	Name    string            // Container name.
	Image   string            // Docker image.
	Env     []string          // Environment variables in KEY=VALUE format.
	Cmd     []string          // Command and arguments.
	Volumes map[string]string // Host path to container path bind mounts.
}

// ContainerInfo describes a container's current state.
type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	Status  string
	Running bool
}

// ContainerManager manages Docker container lifecycles.
// Implementations must be safe for concurrent use.
type ContainerManager interface {
	// Create creates and starts a new container. Returns the container ID.
	Create(ctx context.Context, cfg ContainerConfig) (string, error)
	// StoreLogs writes the container's stdout and stderr to the given file path.
	StoreLogs(ctx context.Context, containerID string, filePath string) error
	// List returns all containers managed by this instance.
	List(ctx context.Context) ([]ContainerInfo, error)
	// Stop stops a running container.
	Stop(ctx context.Context, containerID string) error
	// Remove removes a container.
	Remove(ctx context.Context, containerID string) error
}