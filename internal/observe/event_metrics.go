package observe

import (
	"context"
	"strings"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/toolmeta"
)

// AcceptedUsage is the terminal usage aggregateObservedUsage committed to token_stats, with the canonical cost
// status it computed; metrics never recompute it.
type AcceptedUsage struct {
	Provider, Model, Agent                                       string
	InputTokens, OutputTokens, CacheReadTokens, CacheWriteTokens int64
	CostAmount                                                   float64
	CostCurrency                                                 string
	CostStatus                                                   modelcatalog.CostStatus
}

// EventKind is the activity an AcceptedEvent counts.
type EventKind string

// Counted activity kinds.
const (
	EventToolCallTerminal EventKind = "tool_call_terminal" // completed or failed tool call
	EventPermission       EventKind = "permission"
	EventAgentError       EventKind = "agent_error"
)

// AcceptedEvent is built by observeAgentEvent from the normalized event it already accepted. Outcome and ErrorType
// hold telemetry enum values.
type AcceptedEvent struct {
	Kind       EventKind
	Provider   string
	ToolID     string // canonical tool id for EventToolCallTerminal; "" otherwise
	ToolCallID string // used only for per-turn de-duplication; never exported
	TurnID     string // scope of de-duplication; never exported
	Outcome    string
	ErrorType  string
}

// EventMetrics receives the usage and activity observe accepted.
type EventMetrics interface {
	// RecordUsage is called once by aggregateObservedUsage after RecordTokenUsage succeeds.
	RecordUsage(ctx context.Context, usage AcceptedUsage)
	// RecordEvent is called at the end of observeAgentEvent. A tool call reaches it once, on its first terminal
	// update: observe drops repeats of the same ToolCallID within TurnID.
	RecordEvent(ctx context.Context, event AcceptedEvent)
}

// WithEventMetrics injects the usage and activity metrics sink; the default records nothing.
func WithEventMetrics(metrics EventMetrics) Option {
	return func(observer *Observer) {
		observer.eventMetrics = metrics
	}
}

type noopEventMetrics struct{}

func (noopEventMetrics) RecordUsage(context.Context, AcceptedUsage) {}

func (noopEventMetrics) RecordEvent(context.Context, AcceptedEvent) {}

// ActiveCounts returns the live session and distinct agent counts behind the activity gauges. It reads the same
// in-memory snapshot as Health, never Health itself (which stats databases and probes agents).
func (o *Observer) ActiveCounts(ctx context.Context) (sessions, agents int, err error) {
	sessions, agents, _, err = o.activeSnapshot(ctx)
	return sessions, agents, err
}

// Uptime is the time since the daemon started.
func (o *Observer) Uptime() time.Duration {
	return max(o.now().Sub(o.startedAt), 0)
}

// recordObservedEvent reports a counted activity event to the metrics sink.
func (o *Observer) recordObservedEvent(ctx context.Context, snapshot observedSession, event acp.AgentEvent) {
	accepted, ok := acceptedActivity(snapshot, event)
	switch {
	case event.Type == acp.EventTypeDone || event.Type == acp.EventTypeError:
		o.toolCalls.release(event.TurnID)
	case ok && accepted.Kind == EventToolCallTerminal && !o.toolCalls.first(accepted.TurnID, accepted.ToolCallID):
		return
	}
	if ok {
		o.eventMetrics.RecordEvent(ctx, accepted)
	}
}

func acceptedActivity(snapshot observedSession, event acp.AgentEvent) (AcceptedEvent, bool) {
	accepted := AcceptedEvent{Provider: snapshot.provider, TurnID: strings.TrimSpace(event.TurnID)}
	switch event.Type {
	case acp.EventTypeToolResult:
		if event.ToolResultUpdate() {
			return AcceptedEvent{}, false
		}
		accepted.Kind = EventToolCallTerminal
		accepted.ToolCallID = strings.TrimSpace(event.ToolCallID)
		accepted.ToolID = observedToolID(event)
		accepted.Outcome = telemetry.OutcomeOK
		if event.ToolError() || event.ToolStatus() == string(acpsdk.ToolCallStatusFailed) {
			accepted.Outcome = telemetry.OutcomeError
		}
	case acp.EventTypePermission:
		outcome, resolved := permissionOutcome(event)
		if !resolved {
			return AcceptedEvent{}, false
		}
		accepted.Kind = EventPermission
		accepted.Outcome = outcome
	case acp.EventTypeError:
		accepted.Kind = EventAgentError
		accepted.ErrorType = agentErrorType(event.Failure)
	default:
		return AcceptedEvent{}, false
	}
	return accepted, true
}

// observedToolID is the tool's canonical id: the provider's tool identity, else its title, without the hosted MCP
// server wrapper. Names outside the native tool registry export as "other".
func observedToolID(event acp.AgentEvent) string {
	name := event.ProviderToolName()
	if strings.TrimSpace(name) == "" {
		name = event.ToolName()
	}
	return toolmeta.NormalizeHostedToolName(name)
}

// permissionOutcome maps a resolved permission decision; a pending request is not counted yet.
func permissionOutcome(event acp.AgentEvent) (string, bool) {
	decision := acp.PermissionDecision(strings.TrimSpace(event.Decision))
	if decision == "" || decision == acp.PermissionDecisionPending {
		return "", false
	}
	return decision.TelemetryOutcome(), true
}

// agentErrorType classifies an agent error by its failure kind; the error text is never exported.
func agentErrorType(failure *store.SessionFailure) string {
	if failure == nil {
		return telemetry.ErrorProvider
	}
	switch failure.Kind {
	case store.FailureCanceled:
		return telemetry.ErrorCanceled
	case store.FailureTimeout:
		return telemetry.ErrorDeadlineExceeded
	case store.FailurePermission, store.FailureProviderAuth:
		return telemetry.ErrorPermissionDenied
	case store.FailureTransport, store.FailureProcess:
		return telemetry.ErrorIO
	default:
		return telemetry.ErrorProvider
	}
}

// Bounds of the per-turn tool call de-duplication: past them an arbitrary turn is forgotten, so a late repeat from
// it may count again.
const (
	maxDedupTurns         = 1024
	maxDedupCallsPerTurn  = 4096
	toolCallDedupMapStart = 8
)

// toolCallDedup remembers the tool calls already counted in each open turn; a turn is released at its terminal
// event.
type toolCallDedup struct {
	mu    sync.Mutex
	turns map[string]map[string]struct{}
}

// first reports whether callID is counted for the first time in turnID. Calls without identity always count.
func (d *toolCallDedup) first(turnID, callID string) bool {
	if turnID == "" || callID == "" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.turns == nil {
		d.turns = make(map[string]map[string]struct{})
	}
	calls, ok := d.turns[turnID]
	if !ok {
		if len(d.turns) >= maxDedupTurns {
			for id := range d.turns {
				delete(d.turns, id)
				break
			}
		}
		calls = make(map[string]struct{}, toolCallDedupMapStart)
		d.turns[turnID] = calls
	}
	if _, seen := calls[callID]; seen {
		return false
	}
	if len(calls) < maxDedupCallsPerTurn {
		calls[callID] = struct{}{}
	}
	return true
}

func (d *toolCallDedup) release(turnID string) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.turns, turnID)
}
