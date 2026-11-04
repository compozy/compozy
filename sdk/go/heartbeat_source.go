package compozysdk

import (
	"cmp"
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"time"

	"github.com/compozy/compozy/sdk/go/contracts"
)

var heartbeatSourceKindPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

const CapabilityProvideHeartbeatSource = "heartbeat.source"
const ExtensionServiceMethodHeartbeatSourcePoll = "heartbeat/source_poll"

type HeartbeatSourceKind = contracts.HeartbeatSourceKind
type HeartbeatSourceLimits struct {
	MaxItems        int `json:"max_items"`
	MaxSummaryBytes int `json:"max_summary_bytes"`
	MaxStateBytes   int `json:"max_state_bytes"`
}
type WorkspaceRef struct {
	ID      string `json:"id"`
	RootDir string `json:"root_dir"`
}
type HeartbeatSourceRequest struct {
	Version   int                   `json:"version"`
	SourceID  string                `json:"source_id"`
	Kind      string                `json:"kind,omitempty"`
	Params    json.RawMessage       `json:"params"`
	State     json.RawMessage       `json:"state"`
	Limits    HeartbeatSourceLimits `json:"limits"`
	Workspace WorkspaceRef          `json:"workspace"`
	Now       time.Time             `json:"now"`
	DryRun    bool                  `json:"dry_run"`
}
type HeartbeatSourceItem struct {
	Key        string          `json:"key"`
	Title      string          `json:"title"`
	Summary    string          `json:"summary,omitempty"`
	URL        string          `json:"url,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}
type HeartbeatSourceResult struct {
	Items []HeartbeatSourceItem `json:"items"`
	State json.RawMessage       `json:"state,omitempty"`
	More  bool                  `json:"more,omitzero"`
}

type registeredHeartbeatSource struct {
	descriptor HeartbeatSourceKind
	handler    func(context.Context, HeartbeatSourceRequest) (HeartbeatSourceResult, error)
}

func (e *Extension) HeartbeatSource(
	kind string,
	options HeartbeatSourceKind,
	handler func(context.Context, HeartbeatSourceRequest) (HeartbeatSourceResult, error),
) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.initialized || !heartbeatSourceKindPattern.MatchString(kind) || handler == nil {
		return NewInvalidParamsError("invalid heartbeat source registration", nil)
	}
	if _, ok := e.heartbeatHandlers[kind]; ok {
		return NewInvalidParamsError("heartbeat source already registered", nil)
	}
	if _, ok := e.handlers[ExtensionServiceMethodHeartbeatSourcePoll]; ok {
		return NewInvalidParamsError("heartbeat/source_poll is reserved", nil)
	}
	options.Kind = kind
	options.ParamsSchema = cloneRawMessage(options.ParamsSchema)
	e.heartbeatHandlers[kind] = registeredHeartbeatSource{options, handler}
	e.definition.Capabilities.Provides = normalizeStrings(
		append(e.definition.Capabilities.Provides, CapabilityProvideHeartbeatSource),
	)
	e.transport.Handle(ExtensionServiceMethodHeartbeatSourcePoll, e.dispatch)
	return nil
}
func (e *Extension) heartbeatSourceKindsLocked() []HeartbeatSourceKind {
	out := make([]HeartbeatSourceKind, 0, len(e.heartbeatHandlers))
	for _, h := range e.heartbeatHandlers {
		d := h.descriptor
		d.ParamsSchema = cloneRawMessage(d.ParamsSchema)
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b HeartbeatSourceKind) int { return cmp.Compare(a.Kind, b.Kind) })
	return out
}

func (e *Extension) handleHeartbeatSourcePoll(
	ctx context.Context,
	params json.RawMessage,
) (HeartbeatSourceResult, error) {
	var req HeartbeatSourceRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return HeartbeatSourceResult{}, NewInvalidParamsError(err.Error(), nil)
	}
	e.mu.RLock()
	h, ok := e.heartbeatHandlers[req.Kind]
	e.mu.RUnlock()
	if !ok {
		return HeartbeatSourceResult{}, NewInvalidParamsError("heartbeat source kind is not registered", nil)
	}
	return h.handler(ctx, req)
}
