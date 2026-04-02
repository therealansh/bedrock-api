package statemachine

import (
	"slices"

	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
)

// StateMachine defines the allowed state transitions for session statuses.
type StateMachine struct {
	states map[enums.SessionStatus][]enums.SessionStatus
}

// NewStateMachine initializes a StateMachine with predefined valid transitions between session statuses.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		states: map[enums.SessionStatus][]enums.SessionStatus{
			enums.SessionStatusPending:  {enums.SessionStatusRunning, enums.SessionStatusStopped, enums.SessionStatusFailed, enums.SessionStatusFinished},
			enums.SessionStatusRunning:  {enums.SessionStatusStopped, enums.SessionStatusFailed, enums.SessionStatusFinished},
			enums.SessionStatusStopped:  {},
			enums.SessionStatusFailed:   {},
			enums.SessionStatusFinished: {},
		},
	}
}

// Transition checks if moving from the 'from' status to the 'to' status is valid according to the state machine.
// If the transition is valid, it returns the 'to' status; otherwise, it returns the original 'from' status.
func (s *StateMachine) Transition(from enums.SessionStatus, to enums.SessionStatus) enums.SessionStatus {
	allowed, ok := s.states[from]
	if !ok {
		return from
	}

	if slices.Contains(allowed, to) {
		return to
	}

	return from
}
