package http

import (
	"fmt"
	"time"

	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"
)

// RequestCreateSession represents the expected payload for creating a new session.
type RequestCreateSession struct {
	Image   string `json:"image"`
	Command string `json:"command"`
	TTL     string `json:"ttl"`
}

// ToSpec validates the request fields and converts them into a Spec struct.
func (r RequestCreateSession) ToSpec() (*models.Spec, error) {
	if r.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	ttl, err := time.ParseDuration(r.TTL)
	if err != nil {
		return nil, fmt.Errorf("invalid ttl format: %w", err)
	}

	return &models.Spec{
		Image:   r.Image,
		Command: r.Command,
		TTL:     ttl,
	}, nil
}

// RequestUpdateSession represents the expected payload for updating a session's status.
type RequestUpdateSession struct {
	Status enums.SessionStatus `json:"status"`
}

// storeLogsFormFields maps the multipart form field names to log filenames.
// FileMD uploads exactly these three files per session.
var storeLogsFormFields = []struct {
	Field    string // multipart form field name
	Filename string // stored filename
}{
	{Field: "target_log", Filename: "target.log"},
	{Field: "tracer_log", Filename: "tracer.log"},
	{Field: "vfs_pdf", Filename: "vfs.pdf"},
}
