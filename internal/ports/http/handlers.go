package http

import (
	"log"
	"net/http"

	"github.com/amirhnajafiz/bedrock-api/pkg/zclient"

	"github.com/labstack/echo/v5"
)

func (h HTTPServer) health(c *echo.Context) error {
	// call the ZMQ server to check if it's alive
	rsv, err := zclient.SendEvent(h.SocketAddress, []byte("ping"), 10)
	if err != nil || string(rsv) != "pong" {
		log.Println(err)

		return c.String(http.StatusInternalServerError, "ZMQ server is not responding")
	}

	return c.String(http.StatusOK, "OK")
}

func (h HTTPServer) createSession(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}

func (h HTTPServer) updateSession(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}

func (h HTTPServer) getSessions(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}

func (h HTTPServer) getSessionLogs(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}
