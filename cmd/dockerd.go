package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/components/containers"
	zmqclient "github.com/amirhnajafiz/bedrock-api/internal/components/zmq_client"
	"github.com/amirhnajafiz/bedrock-api/internal/configs"
	"github.com/amirhnajafiz/bedrock-api/internal/logger"
	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// Dockerd represents the Docker Daemon command.
type Dockerd struct {
	Ctx context.Context
	Cfg *configs.DockerdConfig
}

// Command returns the cobra command for Dockerd.
func (d Dockerd) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "dockerd",
		Short: "Docker Daemon",
		Long:  "Docker Daemon is a containerization platform that allows you to build, ship, and run containers.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := StartDockerd(d.Ctx, d.Cfg); err != nil {
				panic(err)
			}
		},
	}
}

func StartDockerd(ctx context.Context, cfg *configs.DockerdConfig) error {
	// create a new logger instance
	logr := logger.New(cfg.LogLevel)

	// setting the dockerd name
	name := cfg.Name
	if name == "hostname" {
		name, _ = os.Hostname()
	}
	if len(name) == 0 {
		name = uuid.NewString()
	}

	// build the ZMQ client
	zclient := zmqclient.NewZMQClient(fmt.Sprintf("tcp://%s:%d", cfg.APISocketHost, cfg.APISocketPort))

	// create Docker client and container manager
	cm, err := containers.NewDockerManager()
	if err != nil {
		logr.Error("failed to create Docker Manager", zap.Error(err))
		return err
	}

	// dockerd main loop
	for {
		// check if the context is done before each iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// interval between each API call
		time.Sleep(cfg.PullInterval)

		// get the list of containers
		cts, err := cm.List(context.Background())
		if err != nil {
			logr.Warn("failed to monitor containers", zap.Error(err))
			continue
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
		packet := models.NewPacket().WithSender(name).WithSessions(sessions...)

		// send the packet to ZMQ server
		resp, err := zclient.SendWithTimeout(packet.ToBytes(), int(cfg.APITimeout.Seconds()))
		if err != nil {
			logr.Warn("failed to call API", zap.Error(err))
			continue
		}

		// get the response from ZMQ server
		respPacket, err := models.PacketFromBytes(resp)
		if err != nil {
			logr.Warn("failed to parse packet", zap.Error(err))
			continue
		}

		// TODO: make changes to reach to API state
		for _, session := range respPacket.Sessions {
			switch session.Status {
			case enums.SessionStatusStopped:
			case enums.SessionStatusFailed:
			case enums.SessionStatusFinished:
				if err := stopContainersForSession(cm, session); err != nil {
					logr.Warn("failed to stop container", zap.String("id", session.Id), zap.Error(err))
				}
			case enums.SessionStatusPending:
				if err := startContainersForSession(cm, session); err != nil {
					logr.Warn("failed to start container", zap.String("id", session.Id), zap.Error(err))
				}
			}
		}
	}
}

func startContainersForSession(cm containers.ContainerManager, session models.Session) error {
	target := fmt.Sprintf("bedrock-target-%s", session.Id)
	tracer := fmt.Sprintf("bedrock-tracer-%s", session.Id)

	// create tracing output directory for the session
	outputDir := fmt.Sprintf("/tmp/bedrock-outputs/%s", session.Id)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// start the tracer container
	if _, err := cm.Start(
		context.Background(),
		containers.ContainerConfig{
			Name:  tracer,
			Image: "ghcr.io/amirhnajafiz/bedrock-tracer:v0.0.6-beta",
			Cmd: []string{
				"bdtrace",
				"--container",
				target,
				"-o",
				"/logs",
			},
			Flags: map[string]any{
				"pid":        "host",
				"privileged": true,
			},
			Volumes: map[string]string{
				"/sys":                               "/sys:rw",
				"/lib/modules":                       "/lib/modules:ro",
				"/var/run/docker.sock":               "/var/run/docker.sock",
				"/tmp/bedrock-outputs/" + session.Id: "/logs",
			},
		},
	); err != nil {
		return err
	}

	// start the target container
	if _, err := cm.Start(
		context.Background(),
		containers.ContainerConfig{
			Name:  target,
			Image: session.Spec.Image,
			Cmd:   strings.Split(session.Spec.Command, " "),
		},
	); err != nil {
		return err
	}

	return nil
}

func stopContainersForSession(cm containers.ContainerManager, session models.Session) error {
	target := fmt.Sprintf("bedrock-target-%s", session.Id)
	tracer := fmt.Sprintf("bedrock-tracer-%s", session.Id)

	// stop the target container
	if err := cm.Stop(context.Background(), target); err != nil {
		return err
	}

	// stop the tracer container
	if err := cm.Stop(context.Background(), tracer); err != nil {
		return err
	}

	return nil
}
