package filemd

import (
	"context"
	"time"

	"github.com/amirhnajafiz/bedrock-api/internal/components/logs"
	"go.uber.org/zap"
)

const (
	maxUploadRetries  = 3
	initialRetryDelay = 1 * time.Second
)

// uploadFields maps each well-known log file to the multipart form field name
// expected by the API endpoint.
var uploadFields = []struct {
	Filename string
	Field    string
}{
	{Filename: logs.TargetLogFile, Field: "target_log"},
	{Filename: logs.TracerLogFile, Field: "tracer_log"},
	{Filename: logs.VFSPDFFile, Field: "vfs_pdf"},
}

// Daemon is the main FileMD orchestrator. It periodically scans for completed
// volumes, reads log files, uploads them to the API, and removes local artifacts.
type Daemon struct {
	Scanner      VolumeScanner
	Uploader     Uploader
	VolumePath   string
	PollInterval time.Duration
	Logger       *zap.Logger
}

// Run starts the FileMD polling loop. It blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.Logger.Info("filemd shutting down")
			return ctx.Err()
		case <-ticker.C:
			d.processOnce()
		}
	}
}

// processOnce performs a single scan-upload-cleanup cycle.
func (d *Daemon) processOnce() {
	sessions, err := d.Scanner.ReadyVolumes()
	if err != nil {
		d.Logger.Warn("failed to scan volumes", zap.Error(err))
		return
	}

	for _, sessionID := range sessions {
		if err := d.processSession(sessionID); err != nil {
			d.Logger.Warn("failed to process session",
				zap.String("session_id", sessionID),
				zap.Error(err),
			)
			continue
		}

		// Clean up local artifacts after successful upload.
		if err := RemoveVolume(d.VolumePath, sessionID); err != nil {
			d.Logger.Warn("failed to remove volume",
				zap.String("session_id", sessionID),
				zap.Error(err),
			)
		}

		d.Logger.Info("processed session logs", zap.String("session_id", sessionID))
	}
}

// processSession reads log files from the volume and uploads them to the API.
func (d *Daemon) processSession(sessionID string) error {
	var uploads []LogUpload

	for _, mapping := range uploadFields {
		data, err := ReadLogFile(d.VolumePath, sessionID, mapping.Filename)
		if err != nil {
			// Log file might not exist (e.g. tracer didn't produce output).
			// We still upload whatever is available.
			d.Logger.Debug("log file not found, skipping",
				zap.String("session_id", sessionID),
				zap.String("filename", mapping.Filename),
			)
			continue
		}

		uploads = append(uploads, LogUpload{
			Field:    mapping.Field,
			Filename: mapping.Filename,
			Content:  data,
		})
	}

	if len(uploads) == 0 {
		return nil
	}

	return d.retryUpload(sessionID, uploads)
}

// retryUpload attempts to upload session logs with exponential backoff.
// Returns the last error if all attempts fail.
func (d *Daemon) retryUpload(sessionID string, uploads []LogUpload) error {
	var lastErr error
	delay := initialRetryDelay

	for attempt := 1; attempt <= maxUploadRetries; attempt++ {
		lastErr = d.Uploader.Upload(sessionID, uploads)
		if lastErr == nil {
			return nil
		}

		d.Logger.Warn("upload attempt failed",
			zap.String("session_id", sessionID),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", maxUploadRetries),
			zap.Error(lastErr),
		)

		if attempt < maxUploadRetries {
			time.Sleep(delay)
			delay *= 2
		}
	}

	return lastErr
}
