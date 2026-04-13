package cmd

import (
	"context"
	"fmt"

	"github.com/amirhnajafiz/bedrock-api/internal/components/filemd"
	"github.com/amirhnajafiz/bedrock-api/internal/components/logs"
	"github.com/amirhnajafiz/bedrock-api/internal/configs"
	"github.com/amirhnajafiz/bedrock-api/internal/logger"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// FileMD represents the File Management Daemon command.
type FileMD struct {
	Ctx context.Context
	Cfg *configs.FileMDConfig
}

// Command returns the cobra command for FileMD.
func (f FileMD) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "filemd",
		Short: "File Management Daemon",
		Long:  "File Management Daemon is a POSIX-compliant file management system that provides a unified interface for handling file operations.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("\nconfigs:\n%s\n\n", f.Cfg.String())

			if err := StartFileMD(f.Ctx, f.Cfg); err != nil {
				panic(err)
			}
		},
	}
}

func StartFileMD(ctx context.Context, cfg *configs.FileMDConfig) error {
	logr := logger.New(cfg.LogLevel)

	apiBaseURL := fmt.Sprintf("http://%s:%d", cfg.APIHTTPHost, cfg.APIHTTPPort)

	scanner := &filemd.FSVolumeScanner{
		BasePath:      cfg.VolumePath,
		ExpectedFiles: logs.AllLogFiles,
	}

	uploader := filemd.NewHTTPUploader(apiBaseURL)

	daemon := &filemd.Daemon{
		Scanner:      scanner,
		Uploader:     uploader,
		VolumePath:   cfg.VolumePath,
		PollInterval: cfg.PollInterval,
		Logger:       logr.Named("filemd"),
	}

	logr.Info("starting filemd",
		zap.String("volume_path", cfg.VolumePath),
		zap.Duration("poll_interval", cfg.PollInterval),
		zap.String("api_base_url", apiBaseURL),
	)

	return daemon.Run(ctx)
}
