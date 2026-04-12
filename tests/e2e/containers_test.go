//go:build e2e

package e2e

import (
	"os"
	"testing"

	"github.com/amirhnajafiz/bedrock-api/internal/components/containers"
	"github.com/amirhnajafiz/bedrock-api/internal/components/containers/docker"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"github.com/docker/docker/api/types/image"
)

// TestDockerManager tests the docker container management functionality of the DockerD.
// It creates a container, checks if it's running, stops it, checks if it's stopped, stores the logs, and finally removes the container.
// Note: This test requires Docker to be installed and running on the host machine.
func TestDockerManager(t *testing.T) {
	// create a context
	ctx := t.Context()
	defer ctx.Done()

	// create a container manager
	cm, err := containers.NewDockerManager()
	if err != nil {
		t.Fatalf("failed to create container manager: %v", err)
	}

	// create an nginx container
	containerID, err := cm.Create(ctx, &models.ContainerConfig{
		Name:  "nginx-container",
		Image: "nginx:latest",
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	// start the nginx container
	err = cm.Start(ctx, containerID)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// list containers and check if the nginx container is running
	containersList, err := cm.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}

	found := false
	for _, c := range containersList {
		if c.ID == containerID {
			t.Logf("nginx container is running with ID: %s", containerID)
			found = true
			break
		}
	}
	if !found {
		t.Errorf("nginx container not found")
	}

	// stop the nginx container
	err = cm.Stop(ctx, containerID)
	if err != nil {
		t.Fatalf("failed to stop container: %v", err)
	}

	// list containers again and check if the nginx container is stopped
	containersList, err = cm.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}

	stopped := false
	for _, c := range containersList {
		if c.ID == containerID {
			if c.Exited {
				stopped = true
				break
			}
			t.Errorf("nginx container exists but is still running with ID: %s", containerID)
		}
	}
	if !stopped {
		t.Errorf("nginx container was not found in stopped state with ID: %s", containerID)
	}
	t.Logf("nginx container stopped successfully")

	// store the container logs
	err = cm.StoreLogs(ctx, containerID, "/tmp/nginx-logs.txt")
	if err != nil {
		t.Fatalf("failed to get container logs: %v", err)
	}

	// check if the log file is created
	if _, err := os.Stat("/tmp/nginx-logs.txt"); os.IsNotExist(err) {
		t.Errorf("log file not found")
	} else {
		t.Logf("log file created successfully")
	}

	// clean up the log file
	err = os.Remove("/tmp/nginx-logs.txt")
	if err != nil {
		t.Errorf("failed to remove log file: %v", err)
	}

	// remove the nginx container
	err = cm.Remove(ctx, containerID)
	if err != nil {
		t.Fatalf("failed to remove container: %v", err)
	}

	// list containers again and check if the nginx container is removed
	containersList, err = cm.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}

	for _, c := range containersList {
		if c.ID == containerID {
			t.Errorf("nginx container is still present with ID: %s", containerID)
		}
	}
	t.Logf("nginx container removed successfully")

	// create a docker client and remove the nginx image to clean up
	dclient, err := docker.NewDockerContainerClient()
	if err != nil {
		t.Fatalf("failed to create docker client: %v", err)
	}

	_, err = dclient.ImageRemove(ctx, "nginx:latest", image.RemoveOptions{})
	if err != nil {
		t.Errorf("failed to remove nginx image: %v", err)
	} else {
		t.Logf("nginx image removed successfully")
	}
}
