package containers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// simulatorClient is an in-memory ContainerClient implementation intended for
// tests and local simulation where containers are assumed to run successfully.
type simulatorClient struct {
	mu sync.RWMutex

	nextID int64

	images     map[string]struct{}
	containers map[string]*simContainer
}

type simContainer struct {
	id      string
	name    string
	image   string
	labels  map[string]string
	created time.Time

	running  bool
	exitCode int

	stdout string
	stderr string
}

// newSimulatorClient builds an in-memory simulator that satisfies ContainerClient.
// Any image names passed in initialImages are treated as already available.
func newSimulatorClient(initialImages ...string) ContainerClient {
	images := make(map[string]struct{}, len(initialImages))
	for _, img := range initialImages {
		images[img] = struct{}{}
	}

	return &simulatorClient{
		nextID:     1,
		images:     images,
		containers: make(map[string]*simContainer),
	}
}

func (s *simulatorClient) ContainerCreate(_ context.Context, config *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, _ *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("sim-%d", s.nextID)
	s.nextID++

	labels := make(map[string]string)
	if config != nil && config.Labels != nil {
		maps.Copy(labels, config.Labels)
	}

	imageName := ""
	if config != nil {
		imageName = config.Image
	}

	s.containers[id] = &simContainer{
		id:       id,
		name:     containerName,
		image:    imageName,
		labels:   labels,
		created:  time.Now().UTC(),
		running:  false,
		exitCode: 0,
		stdout:   "",
		stderr:   "",
	}

	return container.CreateResponse{ID: id}, nil
}

func (s *simulatorClient) ContainerStart(_ context.Context, containerID string, _ container.StartOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.containers[containerID]
	if !ok {
		return cerrdefs.ErrNotFound
	}

	c.running = true
	c.exitCode = 0
	return nil
}

func (s *simulatorClient) ContainerStop(_ context.Context, containerID string, _ container.StopOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.containers[containerID]
	if !ok {
		return cerrdefs.ErrNotFound
	}

	c.running = false
	c.exitCode = 0
	return nil
}

func (s *simulatorClient) ContainerRemove(_ context.Context, containerID string, _ container.RemoveOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.containers[containerID]; !ok {
		return cerrdefs.ErrNotFound
	}

	delete(s.containers, containerID)
	return nil
}

func (s *simulatorClient) ContainerList(_ context.Context, options container.ListOptions) ([]container.Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.containers))
	for id := range s.containers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]container.Summary, 0, len(ids))
	for _, id := range ids {
		c := s.containers[id]

		if !options.All && !c.running {
			continue
		}

		if options.Filters.Len() > 0 && !options.Filters.MatchKVList("label", c.labels) {
			continue
		}

		state := "exited"
		status := "Exited (0)"
		if c.running {
			state = "running"
			status = "Up"
		}

		name := c.name
		if name != "" && !strings.HasPrefix(name, "/") {
			name = "/" + name
		}

		out = append(out, container.Summary{
			ID:      c.id,
			Names:   []string{name},
			Image:   c.image,
			State:   state,
			Status:  status,
			Created: c.created.Unix(),
		})
	}

	return out, nil
}

func (s *simulatorClient) ContainerLogs(_ context.Context, containerID string, _ container.LogsOptions) (io.ReadCloser, error) {
	s.mu.RLock()
	c, ok := s.containers[containerID]
	if !ok {
		s.mu.RUnlock()
		return nil, cerrdefs.ErrNotFound
	}

	stdout := c.stdout
	stderr := c.stderr
	s.mu.RUnlock()

	var buf bytes.Buffer
	if stdout != "" {
		_, _ = stdcopy.NewStdWriter(&buf, stdcopy.Stdout).Write([]byte(stdout))
	}
	if stderr != "" {
		_, _ = stdcopy.NewStdWriter(&buf, stdcopy.Stderr).Write([]byte(stderr))
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func (s *simulatorClient) ContainerInspect(_ context.Context, containerID string) (container.InspectResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.containers[containerID]
	if !ok {
		return container.InspectResponse{}, cerrdefs.ErrNotFound
	}

	state := &container.State{
		Running:  c.running,
		ExitCode: c.exitCode,
	}
	if c.running {
		state.Status = "running"
	} else {
		state.Status = "exited"
	}

	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:      c.id,
			Name:    "/" + c.name,
			Created: c.created.Format(time.RFC3339),
			State:   state,
		},
		Config: &container.Config{
			Image: c.image,
		},
	}, nil
}

func (s *simulatorClient) ImageInspectWithRaw(_ context.Context, imageName string) (image.InspectResponse, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.images[imageName]; !ok {
		return image.InspectResponse{}, nil, cerrdefs.ErrNotFound
	}

	return image.InspectResponse{}, nil, nil
}

func (s *simulatorClient) ImagePull(_ context.Context, refStr string, _ image.PullOptions) (io.ReadCloser, error) {
	s.mu.Lock()
	s.images[refStr] = struct{}{}
	s.mu.Unlock()

	payload := map[string]string{"status": "Downloaded newer image"}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(strings.NewReader(string(b) + "\n")), nil
}

func (s *simulatorClient) ImageRemove(_ context.Context, imageName string, _ image.RemoveOptions) ([]image.DeleteResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.images[imageName]; !ok {
		return nil, cerrdefs.ErrNotFound
	}

	delete(s.images, imageName)
	return []image.DeleteResponse{{Deleted: imageName}}, nil
}
