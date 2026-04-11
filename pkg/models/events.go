package models

import (
	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
)

// Event represents an event that can be sent over ZMQ.
type Event struct {
	Headers map[string]string `json:"headers"`
	Payload []byte            `json:"payload"`
}

// NewEvent creates and returns a new Event instance.
func NewEvent() *Event {
	return &Event{
		Headers: make(map[string]string),
	}
}

// WithEventType sets the event type in the event headers.
func (e Event) WithEventType(et enums.EventType) Event {
	e.Headers["event_type"] = et.String()
	return e
}

// GetEventType retrieves the event type from the event headers.
func (e Event) GetEventType() enums.EventType {
	return enums.EventType(e.Headers["event_type"])
}

// WithSessionId sets the session ID in the event headers.
func (e Event) WithSessionId(sessionId string) Event {
	e.Headers["session_id"] = sessionId
	return e
}

// GetSessionId retrieves the session ID from the event headers.
func (e Event) GetSessionId() string {
	return e.Headers["session_id"]
}

// WithPayload sets the payload of the event and returns the event for chaining.
func (e Event) WithPayload(data []byte) Event {
	e.Payload = data
	return e
}

// GetPayload retrieves the payload of the event.
func (e Event) GetPayload() []byte {
	return e.Payload
}
