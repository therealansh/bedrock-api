package http

import (
	"fmt"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"
)

type HTTPServer struct {
	Address       string
	SocketAddress string

	Logr *zap.Logger
}

func (h HTTPServer) Serve() error {
	// create a new echo instance
	e := echo.New()

	// set the health handler
	e.GET("/health", h.health)

	// set the middlewares
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:     true,
		LogStatus:  true,
		LogMethod:  true,
		LogLatency: true,
		Skipper:    middleware.DefaultSkipper,
		LogValuesFunc: func(c *echo.Context, values middleware.RequestLoggerValues) error {
			h.Logr.Info("request",
				zap.String("uri", values.URI),
				zap.String("method", values.Method),
				zap.Int("status", values.Status),
				zap.Duration("latency", values.Latency),
				zap.Error(values.Error),
			)
			return nil
		},
	}))
	e.Use(middleware.CORS("*"))

	// create api group
	api := e.Group("/api")

	// set the session handlers
	api.POST("/sessions", h.createSession)
	api.PUT("/sessions/:id", h.updateSession)
	api.GET("/sessions", h.getSessions)
	api.GET("/sessions/:id/logs", h.getSessionLogs)

	// log the server start information
	h.Logr.Info("server started", zap.String("address", h.Address), zap.String("socket_address", h.SocketAddress))

	// log the registered routes
	for _, route := range e.Router().Routes() {
		h.Logr.Info("registered route", zap.String("method", route.Method), zap.String("path", route.Path))
	}

	// start the server
	if err := e.Start(h.Address); err != nil {
		return fmt.Errorf("failed to start echo server: %v", err)
	}

	return nil
}
