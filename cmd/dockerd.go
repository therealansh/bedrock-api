package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/amirhnajafiz/bedrock-api/internal/components/containers"
	"github.com/amirhnajafiz/bedrock-api/internal/configs"
	"github.com/amirhnajafiz/bedrock-api/internal/logger"
	"github.com/amirhnajafiz/bedrock-api/internal/ports/daemon"

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

	// create Docker client and container manager
	cm, err := containers.NewDockerManager()
	if err != nil {
		logr.Error("failed to create Docker Manager", zap.Error(err))
		return err
	}

	// create and build a daemon instance
	daemonInstance := daemon.Daemon{
		ContainerManager: cm,
		Logr:             logr.Named("daemon"),
		PullInterval:     cfg.PullInterval,
	}.Build(name, cfg.DataDir, cfg.BDTraceImage, fmt.Sprintf("tcp://%s:%d", cfg.APISocketHost, cfg.APISocketPort))
	if err := daemonInstance.Serve(ctx); err != nil {
		logr.Error("failed to start daemon", zap.Error(err))
		return err
	}

	return nil
}
