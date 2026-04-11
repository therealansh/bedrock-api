package enums

// EventType represents the type of an event that can be sent over ZMQ.
type EventType string

// Event types. This enum is used for API and DockerD communication to manage the sessions.
// - session_start: API sends this event to DockerD to start a session.
// - session_running: DockerD sends this event to API to indicate that the session is running.
// - session_end: DockerD sends this event to API to indicate that the session has ended.
// - session_stopped: API sends this event to DockerD to stop a session.
// - session_failed: DockerD sends this event to API to indicate that the session failed.
// - session_cleanup: API sends this event to DockerD to clean up the session.
const (
	EventTypeSessionStart   EventType = "session_start"
	EventTypeSessionRunning EventType = "session_running"
	EventTypeSessionEnd     EventType = "session_end"
	EventTypeSessionStopped EventType = "session_stopped"
	EventTypeSessionFailed  EventType = "session_failed"
	EventTypeSessionCleanup EventType = "session_cleanup"
)

// String returns the string representation of the EventType.
func (e EventType) String() string {
	return string(e)
}
