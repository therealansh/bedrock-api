package statemachine_test

import (
	"testing"

	statemachine "github.com/amirhnajafiz/bedrock-api/internal/state_machine"
	"github.com/amirhnajafiz/bedrock-api/pkg/enums"
)

// TestStateMachine tests the state machine logic for session status transitions.
func TestStateMachine(t *testing.T) {
	// create a new state machine instance
	sm := statemachine.NewStateMachine()

	// define test cases with different session status transitions
	cases := [][]enums.SessionStatus{
		{enums.SessionStatusPending, enums.SessionStatusRunning, enums.SessionStatusRunning},
		{enums.SessionStatusPending, enums.SessionStatusFailed, enums.SessionStatusFailed},
		{enums.SessionStatusPending, enums.SessionStatusStopped, enums.SessionStatusStopped},
		{enums.SessionStatusPending, enums.SessionStatusFinished, enums.SessionStatusFinished},
		{enums.SessionStatusRunning, enums.SessionStatusFailed, enums.SessionStatusFailed},
		{enums.SessionStatusRunning, enums.SessionStatusStopped, enums.SessionStatusStopped},
		{enums.SessionStatusRunning, enums.SessionStatusFinished, enums.SessionStatusFinished},
		{enums.SessionStatusStopped, enums.SessionStatusRunning, enums.SessionStatusStopped},
		{enums.SessionStatusFailed, enums.SessionStatusPending, enums.SessionStatusFailed},
		{enums.SessionStatusFinished, enums.SessionStatusStopped, enums.SessionStatusFinished},
	}

	for _, c := range cases {
		got := sm.Transition(c[0], c[1])
		if got != c[2] {
			t.Errorf("Transition(%v, %v) = %v; want %v", c[0], c[1], got, c[2])
		}
	}
}
