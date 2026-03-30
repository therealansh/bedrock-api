package cmd

import (
	"fmt"

	"github.com/amirhnajafiz/bedrock-api/internal/configs"
	"github.com/amirhnajafiz/bedrock-api/internal/logger"
	"github.com/amirhnajafiz/bedrock-api/internal/ports/http"
	"github.com/amirhnajafiz/bedrock-api/internal/ports/zmq"
	"github.com/amirhnajafiz/bedrock-api/internal/scheduler"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// API represents the API command.
type API struct {
	Cfg *configs.APIConfig
}

// Command returns the cobra command for API.
func (a API) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "api",
		Short: "API Server",
		Long:  "API Server is a RESTful API server that provides endpoints for managing and interacting with the system.",
		Run: func(cmd *cobra.Command, args []string) {
			StartAPI(a.Cfg)
		},
	}
}

func StartAPI(cfg *configs.APIConfig) {
	// create a new logger instance
	logr := logger.New(cfg.LogLevel)

	// start the ZMQ server
	zmqServer := zmq.ZMQServer{
		Address:   fmt.Sprintf("tcp://%s:%d", cfg.SocketHost, cfg.SocketPort),
		Logr:      logr.Named("zmq"),
		Scheduler: scheduler.NewRoundRobin(),
	}
	go func() {
		if err := zmqServer.Serve(); err != nil {
			logr.Panic("zmq failed", zap.Error(err))
		}
	}()

	// start the HTTP server
	httpServer := http.HTTPServer{
		Address:       fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort),
		SocketAddress: zmqServer.Address,
		Logr:          logr.Named("http"),
	}
	if err := httpServer.Serve(); err != nil {
		logr.Error("http failed", zap.Error(err))
	}
}
