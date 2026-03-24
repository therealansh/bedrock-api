package cmd

import (
	"fmt"

	"github.com/amirhnajafiz/bedrock-api/internal/configs"
	"github.com/amirhnajafiz/bedrock-api/internal/ports/http"
	"github.com/amirhnajafiz/bedrock-api/internal/ports/zmq"

	"github.com/spf13/cobra"
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
	// start the ZMQ server
	zmqServer := zmq.ZMQServer{
		Address: fmt.Sprintf("tcp://%s:%d", cfg.SocketHost, cfg.SocketPort),
	}
	go func() {
		if err := zmqServer.Serve(); err != nil {
			panic(err)
		}
	}()

	// start the HTTP server
	httpServer := http.HTTPServer{
		Address:       fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort),
		SocketAddress: zmqServer.Address,
	}
	if err := httpServer.Serve(); err != nil {
		panic(err)
	}
}
