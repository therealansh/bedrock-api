package http

import (
	"net/http"

	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

// health checks the server's health by sending an empty packet to ZMQ server.
func (h HTTPServer) health(c *echo.Context) error {
	// call the ZMQ server to check if it's alive
	_, err := h.zclient.Send(models.NewPacket().ToBytes())
	if err != nil {
		h.Logr.Warn("zmq server connection error", zap.Error(err))
		return c.String(http.StatusInternalServerError, "zmq not healthy")
	}

	return c.String(http.StatusOK, "OK")
}

// createSession creates a new session based on the request payload and returns the session ID.
func (h HTTPServer) createSession(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}

// updateSession updates an existing session with the specified ID based on the request payload.
func (h HTTPServer) updateSession(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}

// getSessions retrieves a list of all sessions and returns them in the response.
func (h HTTPServer) getSessions(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}

// getSessionLogs retrieves the logs for a specific session based on the session ID provided in the request parameters.
func (h HTTPServer) getSessionLogs(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}

// storeSessionLogs stores the logs for a specific session based on the session ID provided in the request parameters.
func (h HTTPServer) storeSessionLogs(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}
