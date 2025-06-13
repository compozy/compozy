package utils

import (
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/testsuite"
)

// -----
// Signal and Status Testing Helpers
// -----

// SignalHelper provides utilities for testing signal operations
type SignalHelper struct {
	env *testsuite.TestWorkflowEnvironment
	t   *testing.T
}

// NewSignalHelper creates a new signal testing helper
func NewSignalHelper(env *testsuite.TestWorkflowEnvironment, t *testing.T) *SignalHelper {
	return &SignalHelper{
		env: env,
		t:   t,
	}
}

// WaitAndSendSignal waits for a duration then sends a signal
func (sh *SignalHelper) WaitAndSendSignal(waitDuration time.Duration, signalFunc func()) {
	sh.env.RegisterDelayedCallback(func() {
		signalFunc()
	}, waitDuration)
}

// StatusValidator helps validate workflow and task status changes
type StatusValidator struct {
	t              *testing.T
	expectedStates []core.StatusType
	currentIndex   int
}

// NewStatusValidator creates a new status validator
func NewStatusValidator(t *testing.T, expectedStates []core.StatusType) *StatusValidator {
	return &StatusValidator{
		t:              t,
		expectedStates: expectedStates,
		currentIndex:   0,
	}
}

// ValidateStatusTransition validates that the status changed as expected
func (sv *StatusValidator) ValidateStatusTransition(actualStatus core.StatusType) {
	if sv.currentIndex >= len(sv.expectedStates) {
		sv.t.Errorf("Unexpected status transition to %s - no more expected transitions", actualStatus)
		return
	}

	expected := sv.expectedStates[sv.currentIndex]
	if expected != actualStatus {
		sv.t.Errorf("Status transition %d: expected %s, got %s", sv.currentIndex, expected, actualStatus)
		return
	}

	sv.currentIndex++
}

// IsComplete returns true if all expected status transitions have been validated
func (sv *StatusValidator) IsComplete() bool {
	return sv.currentIndex >= len(sv.expectedStates)
}

var loggerOnce sync.Once

func InitLogger(t *testing.T) {
	loggerOnce.Do(func() {
		if err := logger.InitForTests(); err != nil {
			// Log the error but don't fail test initialization
			t.Errorf("Warning: failed to initialize logger for tests: %v\n", err)
		}
	})
}
