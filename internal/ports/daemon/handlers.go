package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/amirhnajafiz/bedrock-api/internal/components/containers"
	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"
)

// prepares the pull request packet with the current system status, including the list of containers and their statuses.
func (d Daemon) preparePullRequest() (*models.Packet, error) {
	// get the list of containers
	cts, err := d.ContainerManager.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// set sessions with containers data
	sessions := make([]models.Session, 0)
	for _, c := range cts {
		status := enums.SessionStatusRunning
		if c.Exited {
			if c.ExitCode == 0 {
				status = enums.SessionStatusFinished
			} else {
				status = enums.SessionStatusFailed
			}
		}

		sessions = append(sessions, models.Session{
			Id:     c.ID,
			Status: status,
		})
	}

	// build a packet with the container sessions
	packet := models.NewPacket().WithSender(d.name).WithSessions(sessions...)
	return &packet, nil
}

// sync the local container state with the API state.
func (d Daemon) syncWithAPI(sessions []models.Session) []error {
	errors := make([]error, 0)

	// make changes to reach to API state
	for _, session := range sessions {
		switch session.Status {
		case enums.SessionStatusStopped:
		case enums.SessionStatusFailed:
		case enums.SessionStatusFinished:
			// must stop the target and tracer containers for stopped, failed, and finished sessions
			if err := d.stopContainersForSession(session); err != nil {
				errors = append(errors, fmt.Errorf("failed to stop containers for session %s: %w", session.Id, err))
			}
		case enums.SessionStatusPending:
			// start the target and tracer containers for pending sessions
			if err := d.startContainersForSession(session); err != nil {
				errors = append(errors, fmt.Errorf("failed to start containers for session %s: %w", session.Id, err))
			}
		}
	}

	return errors
}

// starts the target and tracer containers for a given session.
func (d Daemon) startContainersForSession(session models.Session) error {
	target := fmt.Sprintf("bedrock-target-%s", session.Id)
	tracer := fmt.Sprintf("bedrock-tracer-%s", session.Id)

	// create the output directory for the tracer
	if err := createTracerOutputDir(d.datadir, session.Id); err != nil {
		return fmt.Errorf("failed to create tracer output directory: %w", err)
	}

	// start the tracer container
	if _, err := d.ContainerManager.Start(
		context.Background(),
		&containers.ContainerConfig{
			Name:    tracer,
			Image:   d.tracerImage,
			Cmd:     defaultTracerCommand(target),
			Flags:   defaultContainerFlags(),
			Volumes: defaultTracerVolumes(d.datadir, session.Id),
		},
	); err != nil {
		return err
	}

	// start the target container
	if _, err := d.ContainerManager.Start(
		context.Background(),
		&containers.ContainerConfig{
			Name:  target,
			Image: session.Spec.Image,
			Cmd:   strings.Split(session.Spec.Command, " "),
		},
	); err != nil {
		return err
	}

	return nil
}

// stops the target and tracer containers for a given session.
func (d Daemon) stopContainersForSession(session models.Session) error {
	// create a new background context for stopping containers
	ctx := context.Background()

	target := fmt.Sprintf("bedrock-target-%s", session.Id)
	tracer := fmt.Sprintf("bedrock-tracer-%s", session.Id)

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
