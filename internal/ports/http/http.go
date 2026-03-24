package http

import (
	"fmt"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type HTTPServer struct {
	Address       string
	SocketAddress string
}

func (h HTTPServer) Serve() error {
	// create a new echo instance
	e := echo.New()

	// set the health handler
	e.GET("/health", h.health)

	// set the middlewares
	e.Use(middleware.RequestLogger())
	e.Use(middleware.CORS("*"))

	// create api group
	api := e.Group("/api")

	// set the session handlers
	api.POST("/sessions", h.createSession)
	api.PUT("/sessions/:id", h.updateSession)
	api.GET("/sessions", h.getSessions)
	api.GET("/sessions/:id/logs", h.getSessionLogs)

	// start the server
	if err := e.Start(h.Address); err != nil {
		return fmt.Errorf("failed to start echo server: %v", err)
	}

	return nil
}
