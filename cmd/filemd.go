package cmd

import (
	"github.com/amirhnajafiz/bedrock-api/internal/configs"

	"github.com/spf13/cobra"
)

// FileMD represents the File Management Daemon command.
type FileMD struct {
	Cfg *configs.FileMDConfig
}

// Command returns the cobra command for FileMD.
func (f FileMD) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "filemd",
		Short: "File Management Daemon",
		Long:  "File Management Daemon is a POSIX-compliant file management system that provides a unified interface for handling file operations.",
		Run: func(cmd *cobra.Command, args []string) {
			StartFileMD(f.Cfg)
		},
	}
}

func StartFileMD(cfg *configs.FileMDConfig) {}
