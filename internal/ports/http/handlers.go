package http

import (
	"net/http"

	"github.com/amirhnajafiz/bedrock-api/pkg/zclient"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

// health checks the server's health by sending a ping message to the
// ZMQ server and expecting a pong response. It returns a 200 OK status if the
// ZMQ server is alive and responsive.
func (h HTTPServer) health(c *echo.Context) error {
	// call the ZMQ server to check if it's alive
	rsv, err := zclient.SendEvent(h.SocketAddress, []byte("ping"), 10)
	if err != nil {
		h.Logr.Warn("zmq server connection error", zap.Error(err))
	} else if string(rsv) != "pong" {
		h.Logr.Warn("zmq server unexpected response", zap.String("response", string(rsv)))
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
