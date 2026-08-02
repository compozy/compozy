package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	loopEffectRelayBatchSize    = 50
	loopEffectRelayPollInterval = 250 * time.Millisecond
	loopEffectActorName         = "loop-effect"
	loopEffectApprovalCode      = "effect_tool_approval_required"
)

type loopEffectStore interface {
	ListPendingLoopEffects(context.Context, int) ([]looppkg.EffectOutboxEntry, error)
	AcknowledgeLoopEffect(context.Context, looppkg.EffectAcknowledgement) (bool, error)
}

type loopEffectToolCaller interface {
	Get(context.Context, toolspkg.Scope, toolspkg.ToolID) (toolspkg.ToolView, error)
	Call(context.Context, toolspkg.Scope, toolspkg.CallRequest) (toolspkg.ToolResult, error)
}

type loopEffectRelay struct {
	store        loopEffectStore
	tools        loopEffectToolCaller
	logger       *slog.Logger
	now          func() time.Time
	batchSize    int
	pollInterval time.Duration
	nudge        chan struct{}
	nudgeOnce    sync.Once
	drainMu      sync.Mutex
}

var _ looppkg.EffectDispatcher = (*loopEffectRelay)(nil)
var _ taskRunTerminalObserver = (*loopEffectRelay)(nil)
var _ loopTerminalObserver = (*loopEffectRelay)(nil)

type persistedEffectEntry struct {
	Kind string         `json:"kind"`
	Tool string         `json:"tool,omitempty"`
	With map[string]any `json:"with,omitempty"`
	Emit *struct {
		Kind    string         `json:"kind"`
		Payload map[string]any `json:"payload,omitempty"`
	} `json:"emit,omitempty"`
	RenderError bool   `json:"render_error,omitempty"`
	Diagnostic  string `json:"diagnostic,omitempty"`
}

func (r *loopEffectRelay) Run(ctx context.Context) {
	if r == nil || ctx == nil {
		return
	}
	interval := r.pollInterval
	if interval <= 0 {
		interval = loopEffectRelayPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := r.DrainPendingLoopEffects(ctx, looppkg.EffectDrainPage{}); err != nil &&
			!errors.Is(err, context.Canceled) {
			r.runtimeLogger().WarnContext(ctx, "daemon: relay loop effects", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-r.nudgeChannel():
		}
	}
}

func (r *loopEffectRelay) DrainPendingLoopEffects(
	ctx context.Context,
	page looppkg.EffectDrainPage,
) (looppkg.EffectDrainReport, error) {
	if r == nil || r.store == nil || r.tools == nil {
		return looppkg.EffectDrainReport{}, errors.New("daemon: loop effect relay is unavailable")
	}
	r.drainMu.Lock()
	defer r.drainMu.Unlock()
	limit := page.Limit
	if limit <= 0 {
		limit = r.claimBatchSize()
	}
	report := looppkg.EffectDrainReport{}
	for {
		entries, err := r.store.ListPendingLoopEffects(ctx, limit)
		if err != nil {
			return report, fmt.Errorf("daemon: list pending loop effects: %w", err)
		}
		if len(entries) == 0 {
			return report, nil
		}
		for _, entry := range entries {
			report.Read++
			ack := r.executeEntry(ctx, entry)
			acknowledged, err := r.store.AcknowledgeLoopEffect(ctx, ack)
			if err != nil {
				return report, fmt.Errorf("daemon: acknowledge loop effect %q: %w", entry.DeliveryID, err)
			}
			if !acknowledged {
				continue
			}
			if ack.Outcome == looppkg.EffectResultOK {
				report.Delivered++
			} else {
				report.Failed++
			}
		}
	}
}

func (r *loopEffectRelay) executeEntry(
	ctx context.Context,
	entry looppkg.EffectOutboxEntry,
) looppkg.EffectAcknowledgement {
	startedAt := r.currentTime()
	ack := looppkg.EffectAcknowledgement{Entry: entry, Outcome: looppkg.EffectResultFailed}
	var persisted persistedEffectEntry
	if err := json.Unmarshal(entry.Entry, &persisted); err != nil {
		ack.Code = "effect_payload_invalid"
		ack.Cause = diagnostics.RedactAndBound(err.Error(), 2048)
		return r.finishAcknowledgement(ack, startedAt)
	}
	if persisted.RenderError || persisted.Kind == "render_error" {
		ack.Code = "effect_render_failed"
		ack.Cause = diagnostics.RedactAndBound(persisted.Diagnostic, 2048)
		return r.finishAcknowledgement(ack, startedAt)
	}
	switch persisted.Kind {
	case "emit":
		return r.executeEmit(entry, persisted, ack, startedAt)
	case "tool":
		return r.executeTool(ctx, entry, persisted, ack, startedAt)
	default:
		ack.Code = "effect_kind_invalid"
		ack.Cause = "effect entry kind is invalid"
		return r.finishAcknowledgement(ack, startedAt)
	}
}

func (r *loopEffectRelay) executeEmit(
	entry looppkg.EffectOutboxEntry,
	persisted persistedEffectEntry,
	ack looppkg.EffectAcknowledgement,
	startedAt time.Time,
) looppkg.EffectAcknowledgement {
	if persisted.Emit == nil || strings.TrimSpace(persisted.Emit.Kind) == "" {
		ack.Code = "effect_emit_invalid"
		ack.Cause = "emit effect kind is required"
		return r.finishAcknowledgement(ack, startedAt)
	}
	payload, err := json.Marshal(map[string]any{
		"delivery_id":     entry.DeliveryID,
		"source_event_id": entry.SourceEventID,
		"authored_kind":   strings.TrimSpace(persisted.Emit.Kind),
		"payload":         persisted.Emit.Payload,
		"trigger":         entry.Trigger,
		"generation":      entry.Generation,
		"node_id":         entry.NodeID,
		"item_index":      entry.ItemIndex,
	})
	if err != nil {
		ack.Code = "effect_emit_invalid"
		ack.Cause = diagnostics.RedactAndBound(err.Error(), 2048)
		return r.finishAcknowledgement(ack, startedAt)
	}
	ack.Outcome = looppkg.EffectResultOK
	ack.CustomEvent = payload
	return r.finishAcknowledgement(ack, startedAt)
}

func (r *loopEffectRelay) executeTool(
	ctx context.Context,
	entry looppkg.EffectOutboxEntry,
	persisted persistedEffectEntry,
	ack looppkg.EffectAcknowledgement,
	startedAt time.Time,
) looppkg.EffectAcknowledgement {
	toolID := toolspkg.ToolID(strings.TrimSpace(persisted.Tool))
	input, err := json.Marshal(persisted.With)
	if err != nil {
		ack.Code = "effect_tool_input_invalid"
		ack.Cause = diagnostics.RedactAndBound(err.Error(), 2048)
		return r.finishAcknowledgement(ack, startedAt)
	}
	scope := toolspkg.Scope{
		WorkspaceID: string(entry.WorkspaceID), AgentName: loopEffectActorName,
		ActorKind: string(workspaceaccess.ActorDaemon),
	}
	view, err := r.tools.Get(ctx, scope, toolID)
	if err != nil {
		return r.effectToolError(ack, startedAt, err)
	}
	if view.Decision.ApprovalRequired {
		ack.Outcome = looppkg.EffectResultApprovalRequired
		ack.Code = loopEffectApprovalCode
		ack.Cause = "effect tool requires interactive approval"
		return r.finishAcknowledgement(ack, startedAt)
	}
	_, err = r.tools.Call(ctx, scope, toolspkg.CallRequest{
		ToolID: toolID, ToolCallID: "loop-effect:" + entry.DeliveryID,
		WorkspaceID: string(entry.WorkspaceID), AgentName: loopEffectActorName,
		ActorKind: string(workspaceaccess.ActorDaemon), CorrelationID: entry.DeliveryID,
		Input: input,
	})
	if err != nil {
		return r.effectToolError(ack, startedAt, err)
	}
	ack.Outcome = looppkg.EffectResultOK
	return r.finishAcknowledgement(ack, startedAt)
}

func (r *loopEffectRelay) effectToolError(
	ack looppkg.EffectAcknowledgement,
	startedAt time.Time,
	err error,
) looppkg.EffectAcknowledgement {
	switch {
	case errors.Is(err, toolspkg.ErrToolNotFound):
		ack.Outcome = looppkg.EffectResultToolMissing
		ack.Code = "effect_tool_missing"
	case errors.Is(err, toolspkg.ErrToolApprovalRequired):
		ack.Outcome = looppkg.EffectResultApprovalRequired
		ack.Code = loopEffectApprovalCode
	default:
		ack.Code = "effect_tool_failed"
		var toolErr *toolspkg.ToolError
		if errors.As(err, &toolErr) && toolErr.Code != "" {
			ack.Code = string(toolErr.Code)
		}
	}
	ack.Cause = diagnostics.RedactAndBound(err.Error(), 2048)
	return r.finishAcknowledgement(ack, startedAt)
}

func (r *loopEffectRelay) finishAcknowledgement(
	ack looppkg.EffectAcknowledgement,
	startedAt time.Time,
) looppkg.EffectAcknowledgement {
	ack.At = r.currentTime()
	ack.Duration = max(ack.At.Sub(startedAt), 0)
	return ack
}

func (r *loopEffectRelay) Nudge() {
	if r == nil {
		return
	}
	select {
	case r.nudgeChannel() <- struct{}{}:
	default:
	}
}

func (r *loopEffectRelay) OnTaskRunTerminal(context.Context, hookspkg.TaskRunLeasePayload) error {
	r.Nudge()
	return nil
}

func (r *loopEffectRelay) OnLoopTerminal(context.Context, hookspkg.LoopTerminalPayload) error {
	r.Nudge()
	return nil
}

func (r *loopEffectRelay) nudgeChannel() chan struct{} {
	r.nudgeOnce.Do(func() {
		if r.nudge == nil {
			r.nudge = make(chan struct{}, 1)
		}
	})
	return r.nudge
}

func (r *loopEffectRelay) claimBatchSize() int {
	if r.batchSize > 0 {
		return r.batchSize
	}
	return loopEffectRelayBatchSize
}

func (r *loopEffectRelay) currentTime() time.Time {
	if r != nil && r.now != nil {
		return r.now().UTC()
	}
	return time.Now().UTC()
}

func (r *loopEffectRelay) runtimeLogger() *slog.Logger {
	if r != nil && r.logger != nil {
		return r.logger
	}
	return slog.Default()
}
