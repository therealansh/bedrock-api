package http

import (
	"fmt"

	"github.com/amirhnajafiz/bedrock-api/internal/components/sessions"
	zmqclient "github.com/amirhnajafiz/bedrock-api/internal/components/zmq_client"
	"github.com/amirhnajafiz/bedrock-api/internal/scheduler"
	"github.com/amirhnajafiz/bedrock-api/internal/state_machine"
	"github.com/amirhnajafiz/bedrock-api/internal/storage"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"
)

// HTTPServer represents the HTTP server that handles incoming requests and interacts with the ZMQ server, session store, and scheduler.
type HTTPServer struct {
	// public shared modules
	Logr *zap.Logger

	// private modules
	address      string
	scheduler    scheduler.Scheduler
	sessionStore sessions.SessionStore
	zclient      *zmqclient.ZMQClient
	stateMachine *statemachine.StateMachine
}

// NewHTTPServer creates and returns a new instance of HTTPServer.
func (h HTTPServer) Build(address, socketAddress string) *HTTPServer {
	h.address = address

	h.scheduler = scheduler.NewRoundRobin()
	h.sessionStore = sessions.NewSessionStore(storage.NewGoCache())
	h.zclient = zmqclient.NewZMQClient(socketAddress)
	h.stateMachine = statemachine.NewStateMachine()

	return &h
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
	api.POST("/sessions/:id/logs", h.storeSessionLogs)

	// log the server start information
	h.Logr.Info("server started", zap.String("address", h.address))

	// log the registered routes
	for _, route := range e.Router().Routes() {
		h.Logr.Info("registered route", zap.String("method", route.Method), zap.String("path", route.Path))
	}

	// start the server
	if err := e.Start(h.address); err != nil {
		return fmt.Errorf("failed to start echo server: %v", err)
	}

	return nil
}
