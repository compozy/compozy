package loop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/loop/dsl"
)

const maxRenderedEffectBytes = 16 * 1024

const (
	effectScopeLoop = "loop"
	effectScopeNode = "node"
)

// EffectTrigger is the closed authored reaction vocabulary.
type EffectTrigger string

const (
	EffectTriggerOnError      EffectTrigger = "on_error"
	EffectTriggerOnRetry      EffectTrigger = "on_retry"
	EffectTriggerOnSuccess    EffectTrigger = "on_success"
	EffectTriggerOnPause      EffectTrigger = "on_pause"
	EffectTriggerOnTimeout    EffectTrigger = "on_timeout"
	EffectTriggerOnCancel     EffectTrigger = "on_cancel"
	EffectTriggerOnQuarantine EffectTrigger = "on_quarantine"
	EffectTriggerOnDone       EffectTrigger = "on_done"
	EffectTriggerOnNoOp       EffectTrigger = "on_noop"
	EffectTriggerOnBlocked    EffectTrigger = "on_blocked"
	EffectTriggerOnFailed     EffectTrigger = "on_failed"
	EffectTriggerOnExhausted  EffectTrigger = "on_exhausted"
	EffectTriggerOnStalled    EffectTrigger = "on_stalled"
	EffectTriggerOnCanceled   EffectTrigger = "on_canceled"
)

// Valid reports whether the trigger belongs to the authored reaction vocabulary.
func (t EffectTrigger) Valid() bool {
	switch t {
	case EffectTriggerOnError, EffectTriggerOnRetry, EffectTriggerOnSuccess,
		EffectTriggerOnPause, EffectTriggerOnTimeout, EffectTriggerOnCancel,
		EffectTriggerOnQuarantine, EffectTriggerOnDone, EffectTriggerOnNoOp,
		EffectTriggerOnBlocked, EffectTriggerOnFailed, EffectTriggerOnExhausted,
		EffectTriggerOnStalled, EffectTriggerOnCanceled:
		return true
	default:
		return false
	}
}

// NodeScoped reports whether the trigger belongs to one node lifecycle.
func (t EffectTrigger) NodeScoped() bool {
	switch t {
	case EffectTriggerOnError, EffectTriggerOnRetry, EffectTriggerOnSuccess,
		EffectTriggerOnPause, EffectTriggerOnTimeout, EffectTriggerOnCancel,
		EffectTriggerOnQuarantine:
		return true
	default:
		return false
	}
}

// RenderedEffectIntent is one immutable entry ready for same-transaction outbox insertion.
type RenderedEffectIntent struct {
	Trigger    EffectTrigger   `json:"trigger"`
	Generation int             `json:"generation"`
	NodeID     NodeID          `json:"node_id,omitempty"`
	ItemIndex  int             `json:"item_index,omitempty"`
	EntryIndex int             `json:"entry_index"`
	Entry      json.RawMessage `json:"entry"`
}

// Validate rejects incomplete rendered delivery intents before persistence.
func (i RenderedEffectIntent) Validate() error {
	if !i.Trigger.Valid() || i.Generation < 1 || i.ItemIndex < 0 || i.EntryIndex < 0 {
		return fmt.Errorf("%w: rendered effect identity is invalid", ErrValidation)
	}
	if i.Trigger.NodeScoped() != (i.NodeID != "") {
		return fmt.Errorf("%w: rendered effect scope does not match trigger %q", ErrValidation, i.Trigger)
	}
	if len(i.Entry) == 0 || !json.Valid(i.Entry) {
		return fmt.Errorf("%w: rendered effect entry must be valid JSON", ErrValidation)
	}
	return nil
}

// EffectDeliveryID derives the stable at-least-once identity for one entry.
func EffectDeliveryID(runID RunID, sourceEventID string, trigger EffectTrigger, entryIndex int) string {
	identity := strings.Join([]string{
		strings.TrimSpace(string(runID)), strings.TrimSpace(sourceEventID),
		strings.TrimSpace(string(trigger)), fmt.Sprintf("%d", entryIndex),
	}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(digest[:])
}

// RenderEffectIntents resolves each entry independently and persists failures as data.
func RenderEffectIntents(
	effects []dsl.EffectSpec,
	req EffectContextRequest,
) ([]RenderedEffectIntent, error) {
	if len(effects) == 0 {
		return nil, nil
	}
	effectContext, err := BuildEffectContext(req)
	if err != nil {
		return nil, err
	}
	namespace := map[string]any{namespaceInputsKey: cloneAnyMap(req.Run.Inputs), "effect": effectContext}
	intents := make([]RenderedEffectIntent, 0, len(effects))
	for index, spec := range effects {
		entry, renderErr := renderEffectEntry(spec, namespace)
		if renderErr != nil {
			entry = renderEffectErrorEntry(renderErr)
		}
		intent := RenderedEffectIntent{
			Trigger: req.Trigger, Generation: req.Generation, NodeID: req.NodeID,
			ItemIndex: req.ItemIndex, EntryIndex: index, Entry: entry,
		}
		if err := intent.Validate(); err != nil {
			return nil, err
		}
		intents = append(intents, intent)
	}
	return intents, nil
}

type renderedEffectEntry struct {
	Kind        string         `json:"kind"`
	Tool        string         `json:"tool,omitempty"`
	With        map[string]any `json:"with,omitempty"`
	Emit        *dsl.EmitSpec  `json:"emit,omitempty"`
	RenderError bool           `json:"render_error,omitempty"`
	Diagnostic  string         `json:"diagnostic,omitempty"`
}

func renderEffectEntry(spec dsl.EffectSpec, namespace map[string]any) (json.RawMessage, error) {
	entry := renderedEffectEntry{}
	switch {
	case strings.TrimSpace(spec.Tool) != "" && spec.Emit == nil:
		rendered, err := renderEffectMap("effect.with", spec.With, namespace)
		if err != nil {
			return nil, err
		}
		entry.Kind, entry.Tool, entry.With = "tool", strings.TrimSpace(spec.Tool), rendered
	case strings.TrimSpace(spec.Tool) == "" && spec.Emit != nil:
		payload, err := renderEffectMap("effect.emit.payload", spec.Emit.Payload, namespace)
		if err != nil {
			return nil, err
		}
		entry.Kind = "emit"
		entry.Emit = &dsl.EmitSpec{Kind: strings.TrimSpace(spec.Emit.Kind), Payload: payload}
	default:
		return nil, fmt.Errorf("%w: effect must declare exactly one emit or tool", ErrValidation)
	}
	return sanitizeRenderedEffect(entry)
}

func renderEffectMap(name string, value map[string]any, namespace map[string]any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	rendered, err := renderActionParam(name, value, namespace)
	if err != nil {
		return nil, err
	}
	result, ok := rendered.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: rendered effect payload is not an object", ErrValidation)
	}
	return result, nil
}

func sanitizeRenderedEffect(entry renderedEffectEntry) (json.RawMessage, error) {
	raw, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal rendered effect: %w", err)
	}
	redacted, err := diagnostics.RedactJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("loop: redact rendered effect: %w", err)
	}
	if len(redacted) > maxRenderedEffectBytes {
		return nil, fmt.Errorf("%w: rendered effect exceeds %d bytes", ErrValidation, maxRenderedEffectBytes)
	}
	return json.RawMessage(redacted), nil
}

func renderEffectErrorEntry(err error) json.RawMessage {
	entry := renderedEffectEntry{
		Kind: "render_error", RenderError: true,
		Diagnostic: diagnostics.RedactAndBound(err.Error(), 2048),
	}
	raw, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return json.RawMessage(`{"kind":"render_error","render_error":true,"diagnostic":"effect rendering failed"}`)
	}
	return raw
}

// EffectDrainPage controls one relay sweep.
type EffectDrainPage struct {
	Limit int
}

// EffectDrainReport summarizes one relay sweep.
type EffectDrainReport struct {
	Read      int
	Delivered int
	Failed    int
}

// EffectDispatcher drains committed effect deliveries without observing event-table tails.
type EffectDispatcher interface {
	DrainPendingLoopEffects(context.Context, EffectDrainPage) (EffectDrainReport, error)
}

// EffectResultOutcome is the stable per-entry result vocabulary.
type EffectResultOutcome string

const (
	EffectResultOK               EffectResultOutcome = "ok"
	EffectResultFailed           EffectResultOutcome = "failed"
	EffectResultApprovalRequired EffectResultOutcome = "approval_required"
	EffectResultToolMissing      EffectResultOutcome = "tool_missing"
)

// Valid reports whether the outcome belongs to the stable result vocabulary.
func (o EffectResultOutcome) Valid() bool {
	switch o {
	case EffectResultOK, EffectResultFailed, EffectResultApprovalRequired, EffectResultToolMissing:
		return true
	default:
		return false
	}
}

// EffectAcknowledgement atomically records result events and closes one pending row.
type EffectAcknowledgement struct {
	Entry       EffectOutboxEntry
	Outcome     EffectResultOutcome
	Code        string
	Cause       string
	Duration    time.Duration
	CustomEvent json.RawMessage
	At          time.Time
}

// Validate rejects acknowledgements that cannot preserve delivery identity.
func (a EffectAcknowledgement) Validate() error {
	if a.Entry.LoopRunID == "" || a.Entry.WorkspaceID == "" || len(a.Entry.DeliveryID) != 64 ||
		!a.Outcome.Valid() || a.At.IsZero() || a.Duration < 0 {
		return fmt.Errorf("%w: effect acknowledgement identity is invalid", ErrValidation)
	}
	if len(a.CustomEvent) > 0 && !json.Valid(a.CustomEvent) {
		return fmt.Errorf("%w: effect custom event must be valid JSON", ErrValidation)
	}
	return nil
}
