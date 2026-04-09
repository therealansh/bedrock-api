package enums

type SessionStatus string

// Session's status.
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
