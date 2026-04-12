package cmd

import (
	"context"
	"errors"
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
			fmt.Printf("\nconfigs:\n%s\n\n", a.Cfg.String())

			if a.Cfg.FullStackMode {
				// create a new error group to run both the API and the Docker Daemon in full stack mode
				erg, ctx := errgroup.WithContext(a.Ctx)

				// start the API server in a separate goroutine
				erg.Go(func() error {
					return StartAPI(ctx, a.Cfg)
				})

				// start the Docker Daemon in a separate goroutine
				erg.Go(func() error {
					cfg := configs.DefaultDockerdConfig()
					cfg.LogLevel = a.Cfg.LogLevel
					cfg.APISocketHost = a.Cfg.SocketHost
					cfg.APISocketPort = a.Cfg.SocketPort
					cfg.BedrockTracerImage = a.Cfg.BedrockTracerImage

					return StartDockerd(ctx, cfg)
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
	erg, ectx := errgroup.WithContext(ctx)

	// parse durations
	dockerdHealthCheckInterval, _ := time.ParseDuration(cfg.DockerDHealthCheckInterval)
	sessionStatusCheckInterval, _ := time.ParseDuration(cfg.SessionStatusCheckInterval)

	// start the workers
	dockerdHealthChannel := make(chan string)
	erg.Go(func() error {
		workers.WorkerDockerDHealthCheck(ectx, dockerdHealthChannel, logr.Named("dockerd-health"), dockerdHealthCheckInterval)
		logr.Info("docker daemon health check worker stopped")

		return nil
	})
	erg.Go(func() error {
		workers.WorkerCheckExpiredSessions(ectx, logr.Named("session-worker"), sessionStatusCheckInterval)
		logr.Info("session status check worker stopped")

		return nil
	})
	erg.Go(func() error {
		workers.WorkerRemoveFinishedSessions(ectx, logr.Named("cleanup-worker"), sessionStatusCheckInterval)
		logr.Info("cleanup worker stopped")

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
	)
	erg.Go(func() error {
		err := zmqServer.Serve(ectx)
		logr.Info("zmq server stopped", zap.Error(err))

		return err
	})

	// build and start the HTTP server in a separate goroutine
	httpServer := http.HTTPServer{
		Logr: logr.Named("http"),
	}.Build(
		fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort),
		zmqAddress,
	)
	erg.Go(func() error {
		err := httpServer.Serve()
		logr.Info("http server stopped", zap.Error(err))

		return err
	})

	// wait for all servers to finish
	if err := erg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logr.Error("api failed", zap.Error(err))
		return err
	}

	return nil
}
