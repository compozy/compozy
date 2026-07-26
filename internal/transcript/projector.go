package transcript

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

// Projector routes ordered events to deterministic, independently rebuildable entries.
type Projector struct {
	state      ProjectionState
	resolver   ProjectionResolver
	identities map[string]EntryIdentity
	toolRoutes map[string]string
	latest     map[string]string
}

// NewProjector starts an incremental projection from durable routing state.
func NewProjector(state ProjectionState, resolver ProjectionResolver) (*Projector, error) {
	if state.Version != ProjectionVersion {
		return nil, fmt.Errorf(
			"%w: stored version %d, supported version %d",
			ErrProjectionIncompatible,
			state.Version,
			ProjectionVersion,
		)
	}
	if state.Generation < 0 {
		return nil, fmt.Errorf("%w: negative generation %d", ErrProjectionCorrupt, state.Generation)
	}
	return &Projector{
		state:      state,
		resolver:   resolver,
		identities: make(map[string]EntryIdentity),
		toolRoutes: make(map[string]string),
		latest:     make(map[string]string),
	}, nil
}

// State returns the routing state after all assigned events.
func (p *Projector) State() ProjectionState {
	return p.state
}

// Identity returns the latest metadata for an entry touched by this projector.
func (p *Projector) Identity(key string) (EntryIdentity, bool) {
	identity, ok := p.identities[strings.TrimSpace(key)]
	return identity, ok
}

// ToolRoutes returns tool lifecycle routes learned by this projector.
func (p *Projector) ToolRoutes() map[string]string {
	routes := make(map[string]string, len(p.toolRoutes))
	maps.Copy(routes, p.toolRoutes)
	return routes
}

// Assign binds one ordered, sequenced event to a stable logical entry.
func (p *Projector) Assign(ctx context.Context, stored store.SessionEvent) (Assignment, error) {
	if p == nil {
		return Assignment{}, errors.New("transcript: projector is required")
	}
	if ctx == nil {
		return Assignment{}, errors.New("transcript: projection context is required")
	}
	if stored.Sequence <= 0 {
		return Assignment{}, fmt.Errorf("transcript: event sequence must be positive: %d", stored.Sequence)
	}

	decoded := decodeStoredEvent(stored)
	kind := classifyProjectionEvent(decoded)
	completed, err := p.completeActiveBoundary(ctx, kind, stored.Sequence)
	if err != nil {
		return Assignment{}, err
	}

	identity, routed, err := p.routeExisting(ctx, decoded, kind)
	if err != nil {
		return Assignment{}, err
	}
	if !routed {
		identity = p.newIdentity(decoded, kind)
	}
	if stored.Sequence > identity.UpdatedSequence {
		identity.UpdatedSequence = stored.Sequence
	}
	if decoded.parsed.Type == acp.EventTypeDone || decoded.parsed.Type == acp.EventTypeError {
		identity.Complete = true
	}
	p.remember(identity)

	if kind == EntryKindAssistant && !routed {
		p.state.ActiveEntryKey = identity.Key
	}
	toolKey := strings.TrimSpace(toolLifecycleKey(decoded.parsed))
	if decoded.parsed.Type == acp.EventTypeToolCall && toolKey != "" {
		p.toolRoutes[toolKey] = identity.Key
	}
	return Assignment{
		Entry:         identity,
		CompletedKeys: completed,
		ToolKey:       toolKey,
	}, nil
}

func (p *Projector) completeActiveBoundary(
	ctx context.Context,
	kind EntryKind,
	boundarySequence int64,
) ([]string, error) {
	if kind == EntryKindAssistant || strings.TrimSpace(p.state.ActiveEntryKey) == "" {
		return nil, nil
	}
	identity, ok, err := p.resolveIdentity(ctx, p.state.ActiveEntryKey)
	if err != nil {
		return nil, err
	}
	p.state.ActiveEntryKey = ""
	if !ok || identity.Kind != EntryKindAssistant || identity.Complete {
		return nil, nil
	}
	identity.Complete = true
	if boundarySequence > identity.UpdatedSequence {
		identity.UpdatedSequence = boundarySequence
	}
	p.remember(identity)
	return []string{identity.Key}, nil
}

func (p *Projector) routeExisting(
	ctx context.Context,
	decoded *decodedStoredEvent,
	kind EntryKind,
) (EntryIdentity, bool, error) {
	if kind != EntryKindAssistant {
		return EntryIdentity{}, false, nil
	}
	toolKey := strings.TrimSpace(toolLifecycleKey(decoded.parsed))
	if decoded.parsed.Type == acp.EventTypeToolResult && toolKey != "" {
		identity, ok, err := p.resolveTool(ctx, toolKey)
		if err != nil || ok {
			return identity, ok, err
		}
	}

	logicalID := assistantMessageID(decoded)
	if p.state.ActiveEntryKey != "" {
		identity, ok, err := p.resolveIdentity(ctx, p.state.ActiveEntryKey)
		if err != nil {
			return EntryIdentity{}, false, err
		}
		if ok && identity.Kind == EntryKindAssistant && identity.LogicalID == logicalID {
			return identity, true, nil
		}
	}

	if decoded.parsed.Type == acp.EventTypeDone || decoded.parsed.Type == acp.EventTypeError {
		turnID := strings.TrimSpace(decoded.parsed.TurnID)
		if turnID != "" {
			identity, ok, err := p.resolveLatest(ctx, turnID)
			if err != nil || ok {
				return identity, ok, err
			}
		}
	}
	return EntryIdentity{}, false, nil
}

func (p *Projector) newIdentity(decoded *decodedStoredEvent, kind EntryKind) EntryIdentity {
	sequence := decoded.stored.Sequence
	logicalID := ""
	baseID := ""
	switch kind {
	case EntryKindUser:
		logicalID = inputMessageID(decoded, UIRoleUser)
		baseID = logicalID
	case EntryKindSystem:
		logicalID = inputMessageID(decoded, UIRoleSystem)
		baseID = logicalID
	case EntryKindMarker:
		message := runtimeMarkerUIMessage(decoded)
		logicalID = message.ID
		baseID = message.ID
	default:
		logicalID = assistantMessageID(decoded)
		baseID = logicalID
	}
	return EntryIdentity{
		Key:             fmt.Sprintf("g%d:s%d", p.state.Generation, sequence),
		Kind:            kind,
		LogicalID:       logicalID,
		TurnID:          strings.TrimSpace(decoded.parsed.TurnID),
		BaseMessageID:   baseID,
		StartSequence:   sequence,
		UpdatedSequence: sequence,
		Complete:        kind != EntryKindAssistant,
	}
}

func (p *Projector) remember(identity EntryIdentity) {
	p.identities[identity.Key] = identity
	if identity.Kind == EntryKindAssistant && identity.TurnID != "" {
		currentKey := p.latest[identity.TurnID]
		current, ok := p.identities[currentKey]
		if !ok || identity.StartSequence >= current.StartSequence {
			p.latest[identity.TurnID] = identity.Key
		}
	}
}

func (p *Projector) resolveIdentity(
	ctx context.Context,
	key string,
) (EntryIdentity, bool, error) {
	if identity, ok := p.identities[key]; ok {
		return identity, true, nil
	}
	if p.resolver == nil {
		return EntryIdentity{}, false, nil
	}
	identity, ok, err := p.resolver.EntryIdentity(ctx, key)
	if err != nil || !ok {
		return EntryIdentity{}, ok, err
	}
	p.remember(identity)
	return identity, true, nil
}

func (p *Projector) resolveTool(
	ctx context.Context,
	toolKey string,
) (EntryIdentity, bool, error) {
	if entryKey, ok := p.toolRoutes[toolKey]; ok {
		return p.resolveIdentity(ctx, entryKey)
	}
	if p.resolver == nil {
		return EntryIdentity{}, false, nil
	}
	identity, ok, err := p.resolver.ToolEntryIdentity(ctx, toolKey)
	if err != nil || !ok {
		return EntryIdentity{}, ok, err
	}
	p.toolRoutes[toolKey] = identity.Key
	p.remember(identity)
	return identity, true, nil
}

func (p *Projector) resolveLatest(
	ctx context.Context,
	turnID string,
) (EntryIdentity, bool, error) {
	if entryKey, ok := p.latest[turnID]; ok {
		return p.resolveIdentity(ctx, entryKey)
	}
	if p.resolver == nil {
		return EntryIdentity{}, false, nil
	}
	identity, ok, err := p.resolver.LatestAssistantIdentity(ctx, turnID)
	if err != nil || !ok {
		return EntryIdentity{}, ok, err
	}
	p.remember(identity)
	return identity, true, nil
}

func classifyProjectionEvent(decoded *decodedStoredEvent) EntryKind {
	if transcriptMarkerText(decoded.parsed) != "" {
		return EntryKindMarker
	}
	switch decoded.parsed.Type {
	case acp.EventTypeUserMessage:
		return EntryKindUser
	case acp.EventTypeSyntheticReentry:
		return EntryKindSystem
	default:
		return EntryKindAssistant
	}
}
