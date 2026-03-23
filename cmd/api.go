package cmd

import (
	"github.com/amirhnajafiz/bedrock-api/internal/configs"
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

func StartAPI(cfg *configs.APIConfig) {}
