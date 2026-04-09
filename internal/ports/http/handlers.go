package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
	"github.com/amirhnajafiz/bedrock-api/pkg/models"

	"github.com/google/uuid"
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
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(c.Request().Body).Decode(&raw); err != nil {
		return c.String(http.StatusBadRequest, "invalid request body")
	}

	var spec models.Spec
	if err := json.Unmarshal(raw["image"], &spec.Image); err != nil {
		return c.String(http.StatusBadRequest, "invalid image")
	}
	if err := json.Unmarshal(raw["command"], &spec.Command); err != nil {
		return c.String(http.StatusBadRequest, "invalid command")
	}

	var ttlStr string
	if err := json.Unmarshal(raw["ttl"], &ttlStr); err != nil {
		return c.String(http.StatusBadRequest, "invalid ttl")
	}
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid ttl format (e.g. 1s, 1m, 1h)")
	}
	spec.TTL = ttl

	// assign DockerD to this session
	dockerd, err := h.scheduler.Pick()
	if err != nil {
		return c.String(http.StatusServiceUnavailable, "no docker daemons available")
	}

	// create and save session to KV store
	session := models.Session{
		Id:        uuid.New().String(),
		DockerDId: dockerd,
		CreatedAt: time.Now(),
		Status:    enums.SessionStatusPending,
		Spec:      spec,
	}
	h.sessionStore.SaveSession(&session)

	return c.JSON(http.StatusCreated, session)
}

// updateSession updates an existing session with the specified ID based on the request payload.
func (h HTTPServer) updateSession(c *echo.Context) error {
	// read path params
	id := c.Param("id")

	// payload only contains the new desired state
	var payload struct {
		Status enums.SessionStatus `json:"status"`
	}
	if err := c.Bind(&payload); err != nil {
		return c.String(http.StatusBadRequest, "invalid request body")
	}

	// fetch existing session by id (store will find the owning dockerd)
	session, err := h.sessionStore.GetSessionById(id)
	if err != nil {
		h.Logr.Warn("failed to get session", zap.Error(err), zap.String("session id", id))
		return c.String(http.StatusNotFound, "session not found")
	}

	// apply state machine transition
	newState := h.stateMachine.Transition(session.Status, payload.Status)
	if newState == session.Status {
		// transition not allowed or no-op
		return c.String(http.StatusBadRequest, "invalid state transition")
	}

	session.Status = newState

	if err := h.sessionStore.SaveSession(session); err != nil {
		h.Logr.Warn("failed to save session", zap.Error(err), zap.String("session id", id), zap.String("dockerd id", session.DockerDId))
		return c.String(http.StatusInternalServerError, "failed to save session")
	}

	return c.JSON(http.StatusOK, session)
}

// getSessions retrieves a list of all sessions and returns them in the response.
func (h HTTPServer) getSessions(c *echo.Context) error {
	sessions, err := h.sessionStore.ListSessions()
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to list sessions")
	}
	return c.JSON(http.StatusFound, sessions)
}

// getSessionLogs retrieves the logs for a specific session based on the session ID provided in the request parameters.
func (h HTTPServer) getSessionLogs(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}

// storeSessionLogs stores the logs for a specific session based on the session ID provided in the request parameters.
func (h HTTPServer) storeSessionLogs(c *echo.Context) error {
	return c.String(http.StatusNotImplemented, "Not implemented")
}
