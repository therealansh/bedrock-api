package enums

type SessionStatus string

// Session's status. This enum is used to track the lifecycle of a session.
// - pending: the session is created but not sent to the docker daemon yet, or docker daemon didn't acknowledge the session creation yet.
// - running: the session is running and the docker daemon acknowledged the session creation.
// - failed: the session failed caused by an error during execution received from the docker daemon.
// - stopped: the session is stopped by the user or the system, but docker daemon didn't acknowledge the session stop yet.
// - finished: the session is finished and the docker daemon acknowledged the session stop.
const (
	SessionStatusPending  SessionStatus = "pending"
	SessionStatusRunning  SessionStatus = "running"
	SessionStatusFailed   SessionStatus = "failed"
	SessionStatusStopped  SessionStatus = "stopped"
	SessionStatusFinished SessionStatus = "finished"
)

// String returns the string representation of the SessionStatus.
func (s SessionStatus) String() string {
	return string(s)
}
