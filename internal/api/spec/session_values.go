package spec

import (
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func sessionStateValues() []string {
	return []string{
		string(session.StateStarting),
		string(session.StateActive),
		string(session.StateStopping),
		string(session.StateStopped),
	}
}

func sessionTypeValues() []string {
	return []string{
		string(session.SessionTypeUser),
		string(session.SessionTypeDream),
		string(session.SessionTypeSystem),
		string(session.SessionTypeCoordinator),
		string(session.SessionTypeSpawned),
	}
}

func stopReasonValues() []string {
	return []string{
		string(store.StopCompleted),
		string(store.StopUserCanceled),
		string(store.StopMaxIterations),
		string(store.StopLoopDetected),
		string(store.StopTimeout),
		string(store.StopBudgetExceeded),
		string(store.StopError),
		string(store.StopAgentCrashed),
		string(store.StopHookStopped),
		string(store.StopShutdown),
	}
}
