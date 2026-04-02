package filemd

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// LogUpload pairs a multipart form field name with the file content to upload.
type LogUpload struct {
	Field    string
	Filename string
	Content  []byte
}

// Uploader sends log files to the API's POST /api/sessions/:id/logs endpoint.
type Uploader interface {
	Upload(sessionID string, files []LogUpload) error
}

// HTTPUploader implements Uploader using standard net/http.
type HTTPUploader struct {
	BaseURL string // e.g. "http://127.0.0.1:8080"
	Client  *http.Client
}

// NewHTTPUploader creates an Uploader targeting the given API base URL.
func NewHTTPUploader(baseURL string) Uploader {
	return &HTTPUploader{
		BaseURL: baseURL,
		Client:  &http.Client{},
	}
}

func (u *HTTPUploader) Upload(sessionID string, files []LogUpload) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for _, f := range files {
		part, err := writer.CreateFormFile(f.Field, f.Filename)
		if err != nil {
			return fmt.Errorf("create form file %s: %w", f.Field, err)
		}
		if _, err := part.Write(f.Content); err != nil {
			return fmt.Errorf("write form file %s: %w", f.Field, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/api/sessions/%s/logs", u.BaseURL, sessionID)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := u.Client.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed: status %d", resp.StatusCode)
	}

	return nil
}
