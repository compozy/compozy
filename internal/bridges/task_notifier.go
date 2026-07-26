package bridges

import (
	"context"

	"errors"
	"fmt"

	"time"

	"github.com/compozy/agh/internal/notifications"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	terminalTaskEventRunCompleted      = "task.run_completed"
	terminalTaskEventRunFailed         = "task.run_failed"
	terminalTaskEventRunCanceled       = "task.run_canceled"
	terminalTaskEventCanceled          = "task.canceled"
	terminalTaskEventRunReviewApproved = "task.run_review_approved"

	defaultTerminalTaskNotifierLimit = 100
	maxTerminalTaskCursorErrorBytes  = 2048
)

// ErrTerminalTaskNotificationMismatch reports a replayed terminal task event
// that claims finality but no longer agrees with the durable task/run state.
var ErrTerminalTaskNotificationMismatch = errors.New("bridges: terminal task notification state mismatch")

// TerminalTaskEventReader reads the durable task state used to replay terminal notifications.
type TerminalTaskEventReader interface {
	GetTask(ctx context.Context, id string) (taskpkg.Task, error)
	GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error)
	ListTaskEventRecords(ctx context.Context, query taskpkg.EventRecordQuery) ([]taskpkg.EventRecord, error)
}

// BridgeInstanceReader loads bridge instances for direct adapter delivery.
type BridgeInstanceReader interface {
	GetBridgeInstance(ctx context.Context, id string) (BridgeInstance, error)
}

// TerminalTaskNotifierConfig wires the bridge terminal notifier.
type TerminalTaskNotifierConfig struct {
	Subscriptions BridgeTaskSubscriptionStore
	TaskEvents    TerminalTaskEventReader
	Instances     BridgeInstanceReader
	Cursors       notifications.CursorStore
	Transport     DeliveryTransport
	Now           func() time.Time
	EventLimit    int
}

// TerminalTaskNotificationSweep summarizes one notifier replay pass.
type TerminalTaskNotificationSweep struct {
	Subscriptions int `json:"subscriptions"`
	Delivered     int `json:"delivered"`
	Suppressed    int `json:"suppressed"`
	Deferred      int `json:"deferred"`
	Failed        int `json:"failed"`
}

// TerminalTaskNotifier replays durable task events into subscribed bridge targets.
type TerminalTaskNotifier struct {
	subscriptions BridgeTaskSubscriptionStore
	taskEvents    TerminalTaskEventReader
	instances     BridgeInstanceReader
	cursorStore   notifications.CursorStore
	cursors       *notifications.Service
	transport     DeliveryTransport
	now           func() time.Time
	eventLimit    int
}

// NewTerminalTaskNotifier constructs a bridge terminal notifier.
func NewTerminalTaskNotifier(cfg TerminalTaskNotifierConfig) *TerminalTaskNotifier {
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	limit := cfg.EventLimit
	if limit <= 0 {
		limit = defaultTerminalTaskNotifierLimit
	}
	return &TerminalTaskNotifier{
		subscriptions: cfg.Subscriptions,
		taskEvents:    cfg.TaskEvents,
		instances:     cfg.Instances,
		cursorStore:   cfg.Cursors,
		cursors:       notifications.NewService(cfg.Cursors),
		transport:     cfg.Transport,
		now:           now,
		eventLimit:    limit,
	}
}

// DeliverDue replays task events for subscriptions matching the query.
func (n *TerminalTaskNotifier) DeliverDue(
	ctx context.Context,
	query BridgeTaskSubscriptionQuery,
) (TerminalTaskNotificationSweep, error) {
	if err := n.checkReady(); err != nil {
		return TerminalTaskNotificationSweep{}, err
	}
	subscriptions, err := n.subscriptions.ListBridgeTaskSubscriptions(ctx, query)
	if err != nil {
		return TerminalTaskNotificationSweep{}, fmt.Errorf("bridges: list task notification subscriptions: %w", err)
	}

	sweep := TerminalTaskNotificationSweep{Subscriptions: len(subscriptions)}
	var joined error
	for _, subscription := range subscriptions {
		outcome, processErr := n.deliverSubscription(ctx, subscription)
		switch outcome {
		case terminalTaskNotificationDelivered:
			sweep.Delivered++
		case terminalTaskNotificationSuppressed:
			sweep.Suppressed++
		case terminalTaskNotificationDeferred:
			sweep.Deferred++
		case terminalTaskNotificationFailed:
			sweep.Failed++
		}
		if processErr != nil {
			joined = errors.Join(joined, processErr)
		}
	}
	return sweep, joined
}

type terminalTaskNotificationOutcome string

const (
	terminalTaskNotificationNoop       terminalTaskNotificationOutcome = ""
	terminalTaskNotificationDelivered  terminalTaskNotificationOutcome = "delivered"
	terminalTaskNotificationSuppressed terminalTaskNotificationOutcome = "suppressed"
	terminalTaskNotificationDeferred   terminalTaskNotificationOutcome = "deferred"
	terminalTaskNotificationFailed     terminalTaskNotificationOutcome = "failed"
)

type terminalTaskNotificationDecision string

const (
	terminalTaskNotificationDecisionIgnore   terminalTaskNotificationDecision = ""
	terminalTaskNotificationDecisionDeliver  terminalTaskNotificationDecision = "deliver"
	terminalTaskNotificationDecisionSuppress terminalTaskNotificationDecision = "suppress"
	terminalTaskNotificationDecisionDefer    terminalTaskNotificationDecision = "defer"
	terminalTaskNotificationDecisionMismatch terminalTaskNotificationDecision = "mismatch"
)

type terminalTaskNotificationResolution struct {
	notification TerminalTaskNotification
	decision     terminalTaskNotificationDecision
	diagnostic   error
}

type processTaskNotificationResult struct {
	decision   terminalTaskNotificationDecision
	diagnostic error
	sequence   int64
}

func (n *TerminalTaskNotifier) checkReady() error {
	switch {
	case n == nil:
		return errors.New("bridges: terminal task notifier is required")
	case n.subscriptions == nil:
		return errors.New("bridges: terminal task notifier subscriptions store is required")
	case n.taskEvents == nil:
		return errors.New("bridges: terminal task notifier task event reader is required")
	case n.instances == nil:
		return errors.New("bridges: terminal task notifier bridge instance reader is required")
	case n.cursorStore == nil || n.cursors == nil:
		return errors.New("bridges: terminal task notifier cursor store is required")
	case n.transport == nil:
		return ErrDeliveryTransportUnavailable
	default:
		return nil
	}
}
