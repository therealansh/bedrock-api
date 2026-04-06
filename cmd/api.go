package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/configs"
	"github.com/amirhnajafiz/bedrock-api/internal/logger"
	"github.com/amirhnajafiz/bedrock-api/internal/ports/http"
	"github.com/amirhnajafiz/bedrock-api/internal/ports/zmq"
	"github.com/amirhnajafiz/bedrock-api/internal/workers"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// API represents the API command.
type API struct {
	Ctx context.Context
	Cfg *configs.APIConfig
}

// Command returns the cobra command for API.
func (a API) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "api",
		Short: "API Server",
		Long:  "API Server is a RESTful API server that provides endpoints for managing and interacting with the system.",
		Run: func(cmd *cobra.Command, args []string) {
			if a.Cfg.FullStackMode {
				// create a new error group to run both the API and the Docker Daemon in full stack mode
				erg, ctx := errgroup.WithContext(a.Ctx)

				// start the API server in a separate goroutine
				erg.Go(func() error {
					return StartAPI(ctx, a.Cfg)
				})

				// start the Docker Daemon in a separate goroutine
				erg.Go(func() error {
					return StartDockerd(ctx, &configs.DockerdConfig{
						Name:          "hostname",
						LogLevel:      a.Cfg.LogLevel,
						APISocketHost: a.Cfg.SocketHost,
						APISocketPort: a.Cfg.SocketPort,
						APITimeout:    10 * time.Second,
						PullInterval:  30 * time.Second,
					})
				})

				// wait for both servers to finish
				if err := erg.Wait(); err != nil {
					logger.New(a.Cfg.LogLevel).Error("full stack mode failed", zap.Error(err))
				}
			} else {
				if err := StartAPI(a.Ctx, a.Cfg); err != nil {
					panic(err)
				}
			}
		},
	}
}

func StartAPI(ctx context.Context, cfg *configs.APIConfig) error {
	// create a new logger instance
	logr := logger.New(cfg.LogLevel)

	// create an errgroup with the provided context
	erg, ctx := errgroup.WithContext(ctx)

	// start the workers
	dockerdHealthChannel := make(chan string)
	erg.Go(func() error {
		workers.WorkerDockerDHealthCheck(ctx, dockerdHealthChannel, logr.Named("dockerd-health"), cfg.DockerDHealthCheckInterval)
		return nil
	})
	erg.Go(func() error {
		workers.WorkerCheckExpiredSessions(ctx, logr.Named("session-worker"), cfg.SessionStatusCheckInterval)
		return nil
	})

	// build and start the ZMQ server in a separate goroutine
	zmqAddress := fmt.Sprintf("tcp://%s:%d", cfg.SocketHost, cfg.SocketPort)
	zmqServer := zmq.ZMQServer{
		DockerDHealthChannel: dockerdHealthChannel,
		Logr:                 logr.Named("zmq"),
	}.Build(
		zmqAddress,
		cfg.SocketHandlers,
		ctx,
	)
	erg.Go(zmqServer.Serve)

	// build and start the HTTP server in a separate goroutine
	httpServer := http.HTTPServer{
		Logr: logr.Named("http"),
	}.Build(
		fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort),
		zmqAddress,
	)
	erg.Go(httpServer.Serve)

	// wait for all servers to finish
	if err := erg.Wait(); err != nil {
		logr.Error("api failed", zap.Error(err))
		return err
	}

	return nil
}
