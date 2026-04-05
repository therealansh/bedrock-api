package cmd

import (
	"context"
	"fmt"
	"os"
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

		// TODO: must refactor this based on containers data
		sessions := make([]models.Session, 0)
		for _, c := range cts {
			status := enums.SessionStatusRunning
			if c.Status != "running" {
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
			fmt.Println(session.Id)
		}
	}
}
