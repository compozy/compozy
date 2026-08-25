package terminal

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	EventKindOpened            EventKind = "opened"
	EventKindClosed            EventKind = "closed"
	EventKindLeaseChanged      EventKind = "lease_changed"
	EventKindTitleChanged      EventKind = "title_changed"
	EventKindModeChanged       EventKind = "mode_changed"
	EventKindCommandStarted    EventKind = "command_started"
	EventKindCommandFinished   EventKind = "command_finished"
	EventKindInputRequested    EventKind = "input_requested"
	EventKindInputProvided     EventKind = "input_provided"
	EventKindRecordingStarted  EventKind = "recording_started"
	EventKindRecordingStopped  EventKind = "recording_stopped"
	EventKindSubscriberEvicted EventKind = "subscriber_evicted"
	EventKindLimitRejected     EventKind = "limit_rejected"
)

type EventDetail struct {
	Mode        Mode
	Title       string
	LeaseFrom   LeaseState
	LeaseTo     LeaseState
	CommandID   string
	Command     string
	Cwd         string
	DetectedBy  string
	ExitCode    *int
	Signal      *string
	ExitCause   string
	DurationMS  int64
	Approval    string
	RequestID   InputRequestID
	Redacted    bool
	Length      int
	Outcome     string
	RecordingID string
	Digest      string
	Bytes       int64
	Truncated   bool
	Flow        string
	Limit       string
	Current     int
	Max         int
}

type TerminalEvent struct {
	Kind        EventKind
	WorkspaceID string
	ProfileID   string
	ProfileName string
	TerminalID  ID
	Actor       Actor
	Info        *Info
	Exit        *Exit
	Reason      string
	Detail      EventDetail
	At          time.Time
}

type eventObserver func(context.Context, TerminalEvent)

type EventBus struct {
	logger *slog.Logger
	mu     sync.Mutex
	draft  []eventObserver
	frozen atomic.Pointer[[]eventObserver]
}

func NewEventBus(logger *slog.Logger) *EventBus {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventBus{logger: logger}
}

func (b *EventBus) Observe(fn func(context.Context, TerminalEvent)) {
	if b == nil || fn == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.frozen.Load() != nil {
		b.logger.Warn("terminal: observer ignored after event bus freeze")
		return
	}
	b.draft = append(b.draft, fn)
}

func (b *EventBus) Emit(ctx context.Context, event TerminalEvent) {
	if b == nil {
		return
	}
	observers := b.observers()
	for _, observer := range observers {
		b.invoke(ctx, event, observer)
	}
}

func (b *EventBus) observers() []eventObserver {
	if frozen := b.frozen.Load(); frozen != nil {
		return *frozen
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if frozen := b.frozen.Load(); frozen != nil {
		return *frozen
	}
	copyOfObservers := append([]eventObserver(nil), b.draft...)
	b.draft = nil
	b.frozen.Store(&copyOfObservers)
	return copyOfObservers
}

func (b *EventBus) invoke(ctx context.Context, event TerminalEvent, observer eventObserver) {
	defer func() {
		if recovered := recover(); recovered != nil {
			b.logger.Error("terminal: observer panic recovered", "terminal_id", event.TerminalID, "event", event.Kind, "panic", recovered)
		}
	}()
	observer(ctx, event)
}
