package update

import (
	"context"
	"log/slog"
	"time"
)

// OperationEventName is the closed update lifecycle event vocabulary.
type OperationEventName string

const (
	EventOperationAcquired       OperationEventName = "update.operation.acquired"
	EventOperationBlocked        OperationEventName = "update.operation.blocked"
	EventOperationWaiting        OperationEventName = "update.operation.waiting"
	EventOperationResumed        OperationEventName = "update.operation.resumed"
	EventOperationCanceled       OperationEventName = "update.operation.canceled"
	EventOperationCancelDeclined OperationEventName = "update.operation.cancel_declined"
	EventLeaseAcquired           OperationEventName = "update.lease.acquired"
	EventLeaseRenewed            OperationEventName = "update.lease.renewed"
	EventLeaseExpired            OperationEventName = "update.lease.expired"
	EventCheckCompleted          OperationEventName = "update.check.completed"
	EventDownloadProgress        OperationEventName = "update.download.progress"
	EventVerifyCompleted         OperationEventName = "update.verify.completed"
	EventSwapCompleted           OperationEventName = "update.swap.completed"
	EventRestartCompleted        OperationEventName = "update.restart.completed"
	EventHealthCompleted         OperationEventName = "update.health.completed"
	EventFinalized               OperationEventName = "update.finalized"
	EventRolledBack              OperationEventName = "update.rolled_back"
	EventAppStaged               OperationEventName = "update.app.staged"
	EventAppApplying             OperationEventName = "update.app.applying"
	EventAppInstallerHandoff     OperationEventName = "update.app.installer_handoff"
	EventAppVerified             OperationEventName = "update.app.verified"
	EventAppFailed               OperationEventName = "update.app.failed"
	EventOperationRecovered      OperationEventName = "update.operation.recovered"
)

// OperationEvent carries safe correlation fields for one journal transition.
type OperationEvent struct {
	Name               OperationEventName `json:"name"`
	OperationID        string             `json:"operation_id"`
	Revision           int64              `json:"revision"`
	Target             Target             `json:"target"`
	InstallMethod      InstallMethod      `json:"install_method"`
	FromVersion        string             `json:"from_version"`
	ToVersion          string             `json:"to_version"`
	Actor              Actor              `json:"actor"`
	ExecutorGeneration string             `json:"executor_generation,omitempty"`
	Outcome            string             `json:"outcome"`
	OccurredAt         time.Time          `json:"occurred_at"`
}

// OperationEventEmitter receives events at the transition call site.
type OperationEventEmitter interface {
	EmitUpdateEvent(context.Context, OperationEvent)
}

type discardOperationEvents struct{}

func (discardOperationEvents) EmitUpdateEvent(context.Context, OperationEvent) {}

type slogOperationEvents struct {
	logger *slog.Logger
}

// NewSlogOperationEventEmitter writes canonical lifecycle events to structured logs.
func NewSlogOperationEventEmitter(logger *slog.Logger) OperationEventEmitter {
	if logger == nil {
		return nil
	}
	return slogOperationEvents{logger: logger}
}

func (e slogOperationEvents) EmitUpdateEvent(ctx context.Context, event OperationEvent) {
	e.logger.InfoContext(
		ctx,
		string(event.Name),
		"operation_id", event.OperationID,
		"revision", event.Revision,
		"target", event.Target,
		"install_method", event.InstallMethod,
		"from_version", event.FromVersion,
		"to_version", event.ToVersion,
		"actor", event.Actor,
		"executor_generation", event.ExecutorGeneration,
		"outcome", event.Outcome,
	)
}
