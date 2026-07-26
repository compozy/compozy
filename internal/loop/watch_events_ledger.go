package loop

import (
	"context"
	"strings"

	"github.com/compozy/agh/internal/hooks"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
)

// WatchEventsLedger reads durable replay streams for watch-events evaluation.
type WatchEventsLedger interface {
	ReadMatches(ctx context.Context, q WatchEventsQuery) ([]WatchEvent, error)
}

// WatchEventsCursorReader reads per-stream cursor maxima for initial parking.
type WatchEventsCursorReader interface {
	ReadCursors(ctx context.Context, q WatchEventsQuery) (map[string]int64, error)
}

// WatchEventsQuery selects replay rows strictly after each stream cursor.
type WatchEventsQuery struct {
	WorkspaceID string
	Streams     map[string]int64
	Kinds       []string
	Limit       int
}

// ParkedWatchEventSubscription is one dormant watch-events node recovered from
// persisted loop generation output state.
type ParkedWatchEventSubscription struct {
	WorkspaceID   string
	LoopRunID     string
	LoopName      string
	NodeID        string
	Generation    int
	Inputs        map[string]any
	Subscriptions []watchpkg.EventSubscriptionRef
	Cursors       map[string]int64
	Contracts     map[hooks.HookEvent]WatchEventsContract
}

// ParkedWatchEventScanCursor resumes a deterministic parked-subscription scan.
type ParkedWatchEventScanCursor struct {
	WorkspaceID string
	LoopRunID   string
	NodeID      string
}

// WatchEvent is the normalized loop-facing event envelope produced from a
// durable replay row.
type WatchEvent struct {
	Kind        string         `json:"kind"`
	Seq         int64          `json:"seq"`
	Stream      string         `json:"stream"`
	At          string         `json:"at"`
	WorkspaceID string         `json:"workspace_id"`
	TaskID      string         `json:"task_id,omitempty"`
	RunID       string         `json:"run_id,omitempty"`
	LoopRunID   string         `json:"loop_run_id,omitempty"`
	LoopName    string         `json:"loop_name,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Channel     string         `json:"channel,omitempty"`
	WorkID      string         `json:"work_id,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	LedgerKind  string         `json:"-"`
}

// EventMap returns the CEL-facing event namespace used by watch-events filters.
func (e WatchEvent) EventMap() map[string]any {
	event := map[string]any{
		actionKindMetaKey: strings.TrimSpace(e.Kind),
		"seq":             e.Seq,
		"stream":          strings.TrimSpace(e.Stream),
		"at":              strings.TrimSpace(e.At),
		"workspace_id":    strings.TrimSpace(e.WorkspaceID),
		"task_id":         strings.TrimSpace(e.TaskID),
		"run_id":          strings.TrimSpace(e.RunID),
		"loop_run_id":     strings.TrimSpace(e.LoopRunID),
		"loop_name":       strings.TrimSpace(e.LoopName),
		"session_id":      strings.TrimSpace(e.SessionID),
		"channel":         strings.TrimSpace(e.Channel),
		"work_id":         strings.TrimSpace(e.WorkID),
		"payload":         cloneAnyMap(e.Payload),
	}
	return event
}

func (e WatchEvent) eventMap() map[string]any {
	return e.EventMap()
}

func (e WatchEvent) ledgerKind() string {
	if strings.TrimSpace(e.LedgerKind) != "" {
		return strings.TrimSpace(e.LedgerKind)
	}
	return strings.TrimSpace(e.Kind)
}
