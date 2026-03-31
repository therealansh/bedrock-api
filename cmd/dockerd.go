package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/components/containers"
	"github.com/amirhnajafiz/bedrock-api/internal/configs"
	"github.com/amirhnajafiz/bedrock-api/internal/logger"
	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"
	"github.com/amirhnajafiz/bedrock-api/pkg/zclient"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// Dockerd represents the Docker Daemon command.
type Dockerd struct {
	Cfg *configs.DockerdConfig
}

// Command returns the cobra command for Dockerd.
func (d Dockerd) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "dockerd",
		Short: "Docker Daemon",
		Long:  "Docker Daemon is a containerization platform that allows you to build, ship, and run containers.",
		Run: func(cmd *cobra.Command, args []string) {
			StartDockerd(d.Cfg)
		},
	}
}

func StartDockerd(cfg *configs.DockerdConfig) {
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

	// API ZMQ server address
	address := fmt.Sprintf("tcp://%s:%d", cfg.APISocketHost, cfg.APISocketPort)

	// register this docker daemon with API
	registered := false
	_, err := zclient.SendEvent(address, models.NewPacket().WithRegisterDaemon(name).ToBytes(), 20)
	if err != nil {
		logr.Warn("register daemon failed", zap.Error(err))
		for range cfg.APIConnectionRetrys {
			_, e := zclient.SendEvent(address, models.NewPacket().WithRegisterDaemon(name).ToBytes(), 20)
			if e == nil {
				registered = true
				break
			}

			time.Sleep(2 * time.Second)
		}
	} else {
		registered = true
	}

	// check registration before main loop
	if !registered {
		logr.Error("API registration failed", zap.Int("retrys", cfg.APIConnectionRetrys))
		return
	}

	// containers manager instance
	var cm containers.ContainerManager

	// dockerd main loop
	for {
		time.Sleep(30 * time.Second)

		// TODO: must return status of contaienrs
		cts, err := cm.ListContainers()
		if err != nil {
			logr.Warn("failed to monitor containers", zap.Error(err))
			continue
		}

		// TODO: must refactor this based on containers data
		sessions := make([]models.Session, 0)
		for _, c := range cts {
			sessions = append(sessions, models.Session{
				Id:     c,
				Status: enums.SessionStatusRunning,
			})
		}

		// build a packet
		packet := models.NewPacket().WithSender(name).WithSessions(sessions...)

		// send packet to ZMQ server
		resp, err := zclient.SendEvent(address, packet.ToBytes(), 30)
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
