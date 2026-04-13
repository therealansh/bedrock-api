package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"go.uber.org/zap"
)

// labels used for identifying containers that are managed by the daemon.
const (
	daemonContainerKey        = "bedrock-daemon-container"
	daemonContainerVal        = "true"
	daemonContainerSessionId  = "bedrock-daemon-container.sid"
	daemonContainerType       = "bedrock-daemon-container.type"
	daemonContainerTypeTarget = "target"
	daemonContainerTypeTracer = "tracer"
)

// prepareEvents returns a list of events based on the current state of the containers.
func (d Daemon) prepareEvents() ([]models.Event, error) {
	// create a new background context for container operations
	ctx := context.Background()

	// get the list of target containers (assuming that tracer containers run without issues)
	containers, err := d.ContainerManager.List(ctx, map[string]string{
		daemonContainerKey:  daemonContainerVal,
		daemonContainerType: daemonContainerTypeTarget,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	d.Logr.Debug("active targets", zap.Int("count", len(containers)))

	// set events with containers data
	events := make([]models.Event, 0)
	for _, container := range containers {
		// inspect the container to get its current state
		inspect, err := d.ContainerManager.Get(ctx, container.ID)
		if err != nil {
			d.Logr.Warn("failed to inspect container", zap.String("container_id", container.ID), zap.Error(err))
			continue
		}

		// extract the session ID from the container labels
		sid, ok := inspect.Labels[daemonContainerSessionId]
		if !ok {
			d.Logr.Warn("container missing session ID label", zap.String("container_id", container.ID))
			continue
		}

		// determine the session status based on the container's running and exit status
		status := enums.EventTypeSessionRunning
		if inspect.Exited {
			if inspect.ExitCode == 0 {
				status = enums.EventTypeSessionEnd
			} else {
				status = enums.EventTypeSessionFailed
			}

			// collect logs and remove lock file for FileMD
			d.collectSessionLogs(sid)
		}

		// append the event to the list of events
		events = append(events, models.NewEvent().
			WithSessionId(sid).
			WithEventType(status).
			WithPayload(nil),
		)
	}

	return events, nil
}

// sync the daemon state with the API events.
func (d Daemon) syncEvents(events []models.Event) []error {
	// create a slice to collect any errors that occur during event processing
	errors := make([]error, 0)

	// process each event and take appropriate actions based on the event type
	for _, event := range events {
		switch event.GetEventType() {
		case enums.EventTypeSessionStart: // must start the target and tracer containers for this session
			// unmarshal the session spec from the event payload
			var spec models.Spec
			if err := json.Unmarshal(event.GetPayload(), &spec); err != nil {
				d.Logr.Warn("failed to unmarshal spec", zap.String("session_id", event.GetSessionId()), zap.Error(err))
				continue
			}

			// check if the containers for this session are already running to avoid duplicate starts
			if running, err := d.checkContainersForSession(event.GetSessionId()); err != nil {
				d.Logr.Warn("failed to check containers for session", zap.String("session_id", event.GetSessionId()), zap.Error(err))
				continue
			} else if running {
				d.Logr.Info("containers for session are already running, skipping start", zap.String("session_id", event.GetSessionId()))
				continue
			}

			// start the target and tracer containers for this session
			if err := d.startContainersForSession(
				event.GetSessionId(),
				spec,
			); err != nil {
				errors = append(errors, fmt.Errorf("failed to start containers for session %s: %w", event.GetSessionId(), err))

				// if starting the containers failed, we should attempt to stop any containers that may have been started to clean up the state
				if stopErr := d.stopContainersForSession(event.GetSessionId(), true); stopErr != nil {
					d.Logr.Warn("failed to cleanup containers after failed start", zap.String("session_id", event.GetSessionId()), zap.Error(stopErr))
				}
			}
		case enums.EventTypeSessionStopped: // must stop the target and tracer containers for this session
			if err := d.stopContainersForSession(event.GetSessionId(), false); err != nil {
				errors = append(errors, fmt.Errorf("failed to stop containers for session %s: %w", event.GetSessionId(), err))
			}
		case enums.EventTypeSessionCleanup: // must delete the target and tracer containers for this session
			if err := d.stopContainersForSession(event.GetSessionId(), true); err != nil {
				errors = append(errors, fmt.Errorf("failed to delete containers for session %s: %w", event.GetSessionId(), err))
			}
		}
	}

	return errors
}

// checkContainersForSession checks if the target and tracer containers for a given session are running.
func (d Daemon) checkContainersForSession(sessionId string) (bool, error) {
	// create a new background context for checking containers
	ctx := context.Background()

	// list the target containers
	containers, err := d.ContainerManager.List(ctx, map[string]string{
		daemonContainerKey:       daemonContainerVal,
		daemonContainerSessionId: sessionId,
	})
	if err != nil {
		return false, fmt.Errorf("failed to list containers: %w", err)
	}

	// check if both target and tracer containers exists for this session
	hasTarget := false
	hasTracer := false
	for _, container := range containers {
		switch container.Labels[daemonContainerType] {
		case daemonContainerTypeTarget:
			hasTarget = true
		case daemonContainerTypeTracer:
			hasTracer = true
		}
	}

	return hasTarget && hasTracer, nil
}

// starts the target and tracer containers for a given session.
func (d Daemon) startContainersForSession(sessionId string, sessionSpec models.Spec) error {
	// create a new background context for stopping containers
	ctx := context.Background()

	// define the container names for the target and tracer containers based on the session ID
	target := fmt.Sprintf("bedrock-target-%s", sessionId)
	tracer := fmt.Sprintf("bedrock-tracer-%s", sessionId)

	// create the output directory for the tracer
	if err := createTracerOutputDir(d.datadir, sessionId); err != nil {
		return fmt.Errorf("failed to create tracer output directory: %w", err)
	}

	// create lock file to signal volume is not ready
	if err := createLockFile(d.datadir, sessionId); err != nil {
		return fmt.Errorf("failed to create lock file: %w", err)
	}

	// create the tracer container
	tracerId, err := d.ContainerManager.Create(
		ctx,
		&models.ContainerConfig{
			Name:    tracer,
			Image:   d.tracerImage,
			Cmd:     defaultTracerCommand(target),
			Flags:   defaultContainerFlags(),
			Volumes: defaultTracerVolumes(d.datadir, sessionId),
			Labels: map[string]string{
				daemonContainerKey:       daemonContainerVal,
				daemonContainerType:      daemonContainerTypeTracer,
				daemonContainerSessionId: sessionId,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create container %s: %w", tracer, err)
	}

	d.Logr.Debug("container created", zap.String("container_id", tracerId), zap.String("name", tracer))

	// create the target container
	targetId, err := d.ContainerManager.Create(
		ctx,
		&models.ContainerConfig{
			Name:  target,
			Image: sessionSpec.Image,
			Cmd:   strings.Split(sessionSpec.Command, " "),
			Labels: map[string]string{
				daemonContainerKey:       daemonContainerVal,
				daemonContainerType:      daemonContainerTypeTarget,
				daemonContainerSessionId: sessionId,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create container %s: %w", target, err)
	}

	d.Logr.Debug("container created", zap.String("container_id", targetId), zap.String("name", target))

	// start the tracer container
	if err := d.ContainerManager.Start(ctx, tracerId); err != nil {
		return fmt.Errorf("failed to start container %s: %w", tracer, err)
	}

	d.Logr.Info("container started", zap.String("container_id", tracerId), zap.String("name", tracer))

	// start the target container
	if err := d.ContainerManager.Start(ctx, targetId); err != nil {
		return fmt.Errorf("failed to start container %s: %w", target, err)
	}

	d.Logr.Info("container started", zap.String("container_id", targetId), zap.String("name", target))

	return nil
}

// collectSessionLogs fetches stdout/stderr from the target and tracer containers
// and writes them to the session volume. It then removes the .lock file to signal
// that the volume is ready for FileMD to upload.
func (d Daemon) collectSessionLogs(sessionId string) {
	ctx := context.Background()

	targetName := fmt.Sprintf("bedrock-target-%s", sessionId)
	tracerName := fmt.Sprintf("bedrock-tracer-%s", sessionId)

	// list containers for this session to resolve IDs from names
	containers, err := d.ContainerManager.List(ctx, map[string]string{
		daemonContainerKey:       daemonContainerVal,
		daemonContainerSessionId: sessionId,
	})
	if err != nil {
		d.Logr.Warn("failed to list containers for log collection", zap.String("session_id", sessionId), zap.Error(err))
		if err := removeLockFile(d.datadir, sessionId); err != nil {
			d.Logr.Warn("failed to remove lock file", zap.String("session_id", sessionId), zap.Error(err))
		}
		return
	}

	// resolve container IDs by name
	var targetID, tracerID string
	for _, c := range containers {
		if c.Name == targetName {
			targetID = c.ID
		} else if c.Name == tracerName {
			tracerID = c.ID
		}
	}

	// collect target container logs
	if targetID != "" {
		targetLogPath := fmt.Sprintf("%s/%s/target.log", d.datadir, sessionId)
		if err := d.ContainerManager.StoreLogs(ctx, targetID, targetLogPath); err != nil {
			d.Logr.Warn("failed to store target logs", zap.String("session_id", sessionId), zap.Error(err))
		}
	} else {
		d.Logr.Warn("target container not found for log collection", zap.String("session_id", sessionId))
	}

	// collect tracer container logs
	if tracerID != "" {
		tracerLogPath := fmt.Sprintf("%s/%s/tracer.log", d.datadir, sessionId)
		if err := d.ContainerManager.StoreLogs(ctx, tracerID, tracerLogPath); err != nil {
			d.Logr.Warn("failed to store tracer logs", zap.String("session_id", sessionId), zap.Error(err))
		}
	} else {
		d.Logr.Warn("tracer container not found for log collection", zap.String("session_id", sessionId))
	}

	// remove lock file to signal volume is ready
	if err := removeLockFile(d.datadir, sessionId); err != nil {
		d.Logr.Warn("failed to remove lock file", zap.String("session_id", sessionId), zap.Error(err))
	}

	d.Logr.Info("collected session logs", zap.String("session_id", sessionId))
}

// stops the target and tracer containers for a given session.
func (d Daemon) stopContainersForSession(sessionId string, remove bool) error {
	// create a new background context for stopping containers
	ctx := context.Background()

	// list the target containers
	containers, err := d.ContainerManager.List(ctx, map[string]string{
		daemonContainerKey:       daemonContainerVal,
		daemonContainerSessionId: sessionId,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// stop both target and tracer containers for this session
	for _, container := range containers {
		if err := d.ContainerManager.Stop(ctx, container.ID); err != nil {
			d.Logr.Warn("failed to stop container", zap.String("container_id", container.ID), zap.Error(err))
		} else {
			d.Logr.Info("container stopped", zap.String("container_id", container.ID), zap.String("name", container.Name))
		}

		if remove {
			if err := d.ContainerManager.Remove(ctx, container.ID); err != nil {
				d.Logr.Warn("failed to delete container", zap.String("container_id", container.ID), zap.Error(err))
			} else {
				d.Logr.Info("container deleted", zap.String("container_id", container.ID), zap.String("name", container.Name))
			}
		}
	}

	return nil
}
