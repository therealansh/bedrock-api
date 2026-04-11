package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amirhnajafiz/bedrock-api/internal/components/containers"
	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"
	"go.uber.org/zap"
)

// prepares the pull request packet with the current system status, including the list of containers and their statuses.
func (d Daemon) preparePullRequest() (*models.Packet, error) {
	// get the list of containers
	cts, err := d.ContainerManager.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// set events with containers data
	events := make([]models.Event, 0)
	for _, c := range cts {
		// skip containers that are not part of any session
		if ctype, ok := c.Labels["container.type"]; !ok || ctype != "target" {
			continue
		}

		// extract the session ID from the container labels
		sid, ok := c.Labels["container.sid"]
		if !ok {
			continue
		}

		// determine the session status based on the container's running and exit status
		status := enums.EventTypeSessionRunning
		if c.Exited {
			if c.ExitCode == 0 {
				status = enums.EventTypeSessionEnd
			} else {
				status = enums.EventTypeSessionFailed
			}
		}

		// append the event to the list of events
		events = append(events, models.NewEvent().
			WithSessionId(sid).
			WithEventType(status).
			WithPayload(nil),
		)
	}

	// build a packet with events
	packet := models.NewPacket().WithSender(d.name).WithEvents(events...)
	return &packet, nil
}

// sync the local container state with the API state.
func (d Daemon) syncWithAPI(events []models.Event) []error {
	errors := make([]error, 0)

	// make changes to reach to API state
	for _, event := range events {
		switch event.GetEventType() {
		case enums.EventTypeSessionStart:
			var spec models.Spec
			if err := json.Unmarshal(event.GetPayload(), &spec); err != nil {
				d.Logr.Warn("failed to unmarshal spec", zap.String("session_id", event.GetSessionId()), zap.Error(err))
				continue
			}

			// start the target and tracer containers for this session
			if err := d.startContainersForSession(
				event.GetSessionId(),
				spec,
			); err != nil {
				errors = append(errors, fmt.Errorf("failed to start containers for session %s: %w", event.GetSessionId(), err))
			}
		case enums.EventTypeSessionStopped:
			if err := d.stopContainersForSession(event.GetSessionId()); err != nil {
				errors = append(errors, fmt.Errorf("failed to stop containers for session %s: %w", event.GetSessionId(), err))
			}
		case enums.EventTypeSessionCleanup:
			if err := d.deleteContainersForSession(event.GetSessionId()); err != nil {
				errors = append(errors, fmt.Errorf("failed to delete containers for session %s: %w", event.GetSessionId(), err))
			}
		}
	}

	return errors
}

// starts the target and tracer containers for a given session.
func (d Daemon) startContainersForSession(sessionId string, sessionSpec models.Spec) error {
	// create a new background context for stopping containers
	ctx := context.Background()

	target := fmt.Sprintf("bedrock-target-%s", sessionId)
	tracer := fmt.Sprintf("bedrock-tracer-%s", sessionId)

	// check if the target container is running before creating
	if _, err := d.ContainerManager.Get(ctx, target); err == nil {
		return nil
	}

	// create the output directory for the tracer
	if err := createTracerOutputDir(d.datadir, sessionId); err != nil {
		return fmt.Errorf("failed to create tracer output directory: %w", err)
	}

	// start the tracer container
	if _, err := d.ContainerManager.Start(
		ctx,
		&containers.ContainerConfig{
			Name:    tracer,
			Image:   d.tracerImage,
			Cmd:     defaultTracerCommand(target),
			Flags:   defaultContainerFlags(),
			Volumes: defaultTracerVolumes(d.datadir, sessionId),
			Labels: map[string]string{
				"container.type": "tracer",
				"container.sid":  sessionId,
			},
		},
	); err != nil {
		return fmt.Errorf("failed to start container %s: %w", tracer, err)
	}

	// start the target container
	if _, err := d.ContainerManager.Start(
		ctx,
		&containers.ContainerConfig{
			Name:  target,
			Image: sessionSpec.Image,
			Cmd:   strings.Split(sessionSpec.Command, " "),
			Labels: map[string]string{
				"container.type": "target",
				"container.sid":  sessionId,
			},
		},
	); err != nil {
		return fmt.Errorf("failed to start container %s: %w", target, err)
	}

	return nil
}

// stops the target and tracer containers for a given session.
func (d Daemon) stopContainersForSession(sessionId string) error {
	// create a new background context for stopping containers
	ctx := context.Background()

	target := fmt.Sprintf("bedrock-target-%s", sessionId)
	tracer := fmt.Sprintf("bedrock-tracer-%s", sessionId)

	// stop the target container
	if err := d.ContainerManager.Stop(ctx, target); err != nil {
		return fmt.Errorf("failed to stop container %s: %w", target, err)
	}

	// stop the tracer container
	if err := d.ContainerManager.Stop(ctx, tracer); err != nil {
		return fmt.Errorf("failed to stop container %s: %w", tracer, err)
	}

	return nil
}

func (d Daemon) deleteContainersForSession(sessionId string) error {
	// create a new background context for stopping containers
	ctx := context.Background()

	target := fmt.Sprintf("bedrock-target-%s", sessionId)
	tracer := fmt.Sprintf("bedrock-tracer-%s", sessionId)

	// check if the target container is running before trying to stop it
	targetInfo, err := d.ContainerManager.Get(ctx, target)
	if err != nil {
		return fmt.Errorf("failed to get container info for %s: %w", target, err)
	}

	if !targetInfo.Exited {
		// stop the target container
		if err := d.ContainerManager.Stop(ctx, target); err != nil {
			return fmt.Errorf("failed to stop container %s: %w", target, err)
		}

		// stop the tracer container
		if err := d.ContainerManager.Stop(ctx, tracer); err != nil {
			return fmt.Errorf("failed to stop container %s: %w", tracer, err)
		}
	}

	// remove both containers after stopping
	if err := d.ContainerManager.Remove(ctx, target); err != nil {
		return fmt.Errorf("failed to remove container %s: %w", target, err)
	}
	if err := d.ContainerManager.Remove(ctx, tracer); err != nil {
		return fmt.Errorf("failed to remove container %s: %w", tracer, err)
	}

	return nil
}
