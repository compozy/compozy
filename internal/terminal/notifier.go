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
	EventKindAuditChanged      EventKind = "audit_changed"
)

type EventDetail struct {
	Mode         Mode
	Title        string
	LeaseFrom    LeaseState
	LeaseTo      LeaseState
	CommandID    string
	Command      string
	Cwd          string
	DetectedBy   string
	ExitCode     *int
	Signal       *string
	ExitCause    string
	DurationMS   int64
	Approval     string
	RequestID    InputRequestID
	Redacted     bool
	Length       int
	Outcome      string
	RecordingID  string
	Digest       string
	Bytes        int64
	Truncated    bool
	Flow         string
	Limit        string
	Current      int
	Max          int
	AuditBlocked bool
}

type Event struct {
	Kind        EventKind
	WorkspaceID string
	ProfileID   string
	ProfileName string
	TerminalID  ID
	Actor       Actor
	Info        *Info
	Exit        *Exit
	Reason      string
	Detail      *EventDetail
	At          time.Time
}

// DetailValue returns the event detail or its zero value when absent.
func (e Event) DetailValue() EventDetail {
	if e.Detail == nil {
		return EventDetail{}
	}
	return *e.Detail
}

type eventObserver func(context.Context, Event)

type Notifier struct {
	logger *slog.Logger
	mu     sync.Mutex
	draft  []eventObserver
	frozen atomic.Pointer[[]eventObserver]
}

func NewNotifier(logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{logger: logger}
}

// Observe registers an observer until the first Notify freezes the observer set.
// Registrations attempted after that point are ignored and logged.
func (n *Notifier) Observe(fn func(context.Context, Event)) {
	if n == nil || fn == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.frozen.Load() != nil {
		n.logger.Warn("terminal: observer ignored after notifier freeze")
		return
	}
	n.draft = append(n.draft, fn)
}

// Notify synchronously fans a typed terminal event out to the frozen observer set.
func (n *Notifier) Notify(ctx context.Context, event Event) {
	if n == nil {
		return
	}
	observers := n.observers()
	for _, observer := range observers {
		n.invoke(ctx, event, observer)
	}
}

func (n *Notifier) observers() []eventObserver {
	if frozen := n.frozen.Load(); frozen != nil {
		return *frozen
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if frozen := n.frozen.Load(); frozen != nil {
		return *frozen
	}
	copyOfObservers := append([]eventObserver(nil), n.draft...)
	n.draft = nil
	n.frozen.Store(&copyOfObservers)
	return copyOfObservers
}

func (n *Notifier) invoke(ctx context.Context, event Event, observer eventObserver) {
	defer func() {
		if recovered := recover(); recovered != nil {
			n.logger.Error(
				"terminal: observer panic recovered",
				"terminal_id",
				event.TerminalID,
				"event",
				event.Kind,
				"panic",
				recovered,
			)
		}
	}()
	observer(ctx, event)
}
