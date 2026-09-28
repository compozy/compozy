package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// preparedDerive is everything the commit needs, built before the child exists.
type preparedDerive struct {
	createOpts CreateOpts
	receipt    store.SessionDerivationReceipt
}

func (m *Manager) prepareDerive(ctx context.Context, spec deriveSpec, snapshot deriveSnapshot) (preparedDerive, error) {
	if err := m.validateDeriveSource(ctx, spec, snapshot); err != nil {
		return preparedDerive{}, err
	}
	if snapshot.cutErr != nil {
		return preparedDerive{}, snapshot.cutErr
	}
	meta := snapshot.meta
	agentName := spec.agentName
	if agentName == "" {
		agentName = strings.TrimSpace(meta.AgentName)
	}
	agentDef, _, err := m.resolveDeriveTargetAgent(ctx, meta, agentName)
	if err != nil {
		return preparedDerive{}, err
	}
	pendingRoute, routeSelection, err := derivePendingRoute(agentName, agentDef, spec.route)
	if err != nil {
		return preparedDerive{}, err
	}
	imported, err := m.buildImportedContext(snapshot, spec, m.deriveBudget())
	if err != nil {
		return preparedDerive{}, err
	}
	childID, err := m.createSessionID("")
	if err != nil {
		return preparedDerive{}, err
	}
	profileID := spec.profileID
	if profileID == "" {
		profileID = normalizeCreateProfileID(meta.ProfileID)
	}
	now := m.now().UTC()
	// The child's meta starts staged and moves to admitted only once the first message's
	// admission is claimed (a crash in between leaves it staged for the retry to finish);
	// the receipt's immutable outcome records the request's intent.
	firstPrompt := store.SessionFirstPrompt{State: store.SessionDerivationFirstPromptStaged}
	outcomePrompt := firstPrompt
	if spec.message != "" {
		firstPrompt.AdmissionKey = deriveFirstAdmissionKey(spec.key)
		firstPrompt.MessageID = deriveFirstMessageID(spec.workspaceID, spec.key)
		outcomePrompt = firstPrompt
		outcomePrompt.State = store.SessionDerivationFirstPromptAdmitted
	}
	derivation := &store.SessionDerivation{
		Kind: spec.kind, SourceSessionID: meta.ID, IdempotencyKey: spec.key,
		Seed: store.SessionDerivationSeedReplay, PendingRoute: pendingRoute,
		FirstPrompt: firstPrompt, CreatedAt: now,
	}
	receipt := store.SessionDerivationReceipt{
		WorkspaceID: spec.workspaceID, ProfileID: profileID, IdempotencyKey: spec.key,
		RequestFingerprint: spec.fingerprint, SourceSessionID: meta.ID, ChildSessionID: childID,
		Kind: spec.kind, Outcome: deriveOutcome(imported, outcomePrompt), CreatedAt: now,
	}
	opts := CreateOpts{
		DesiredSessionID: childID,
		ProfileID:        profileID,
		AgentName:        agentName,
		Name:             deriveChildName(spec.name, meta.Name),
		Workspace:        meta.WorkspaceID,
		Worktree:         meta.WorktreeIDValue(),
		CWD:              deriveSourceCWD(meta),
		Type:             SessionTypeUser,
		Lineage: &store.SessionLineage{
			ParentSessionID: meta.ID, Kind: spec.kind,
			OriginAgentName: strings.TrimSpace(meta.AgentName), OriginMessageID: spec.messageID,
		},
		Derivation:      derivation,
		ImportedContext: imported,
		deriveReceipt:   &receipt,
	}
	applyDeriveRuntime(&opts, spec.runtime, routeSelection)
	return preparedDerive{createOpts: opts, receipt: receipt}, nil
}

func applyDeriveRuntime(opts *CreateOpts, runtime *DeriveRuntime, route *FallbackRoute) {
	switch {
	case runtime != nil:
		opts.Provider, opts.Model = runtime.Provider, runtime.Model
		opts.ReasoningEffort, opts.Speed = runtime.ReasoningEffort, runtime.Speed
		opts.ACPOptions = acp.CloneSessionConfigOptionSelections(runtime.ACPOptions)
	case route != nil:
		opts.Provider, opts.Model = route.Provider, route.Model
		opts.ReasoningEffort, opts.Speed = route.ReasoningEffort, route.Speed
		opts.ACPOptions = acp.CloneSessionConfigOptionSelections(route.ACPOptions)
	}
}

// derivePendingRoute resolves a 1-based declared route through the fallback-account
// resolver; the chosen route is recorded on the child and applied at its first bind.
func derivePendingRoute(
	agentName string,
	agentDef compozyconfig.AgentDef,
	index int,
) (*store.SessionPendingRoute, *FallbackRoute, error) {
	if index <= 0 {
		return nil, nil, nil
	}
	routes := fallbackRoutesForAgent(agentDef)
	if index > len(routes) {
		return nil, nil, deriveErr(
			ErrDeriveRouteNotFound,
			"agent %q declares %d route(s); route %d does not exist", agentName, len(routes), index,
		)
	}
	route := routes[index-1]
	return &store.SessionPendingRoute{
		Index: index, Provider: route.Provider, Model: route.Model,
		ReasoningEffort: route.ReasoningEffort, Speed: route.Speed,
		ACPOptions:         storeOptionSelectionsFromACP(route.ACPOptions),
		CommandFingerprint: compozyconfig.CommandFingerprint(route.Command),
	}, &route, nil
}

// buildImportedContext bounds the snapshot's carried messages into the child's durable
// imported context. A source that is itself derived carries its imported context
// flattened in front of its own messages (one level, never nested).
func (m *Manager) buildImportedContext(
	snapshot deriveSnapshot,
	spec deriveSpec,
	budget replayBudget,
) (*store.SessionImportedContext, error) {
	messages := append([]transcript.Message(nil), snapshot.messages...)
	inherited, err := decodeImportedMessages(snapshot.meta.ImportedContext)
	if err != nil {
		return nil, err
	}
	if len(inherited) > 0 {
		messages = append(inherited, messages...)
	}
	var aborted *transcript.Message
	if snapshot.sourceTurnInProgress {
		note := abortedReplayNote(m.now().UTC())
		aborted = &note
		budget.MaxBytes -= replayMessageBytes(note) + 1
	}
	bounded, stats := boundReplay(messages, budget)
	if aborted != nil {
		bounded = append(bounded, *aborted)
	}
	if bounded == nil {
		bounded = []transcript.Message{}
	}
	encoded, err := json.Marshal(bounded)
	if err != nil {
		return nil, fmt.Errorf("session: encode imported context: %w", err)
	}
	return &store.SessionImportedContext{
		SourceSessionID: snapshot.meta.ID, Kind: spec.kind,
		OriginAgentName: strings.TrimSpace(snapshot.meta.AgentName), OriginMessageID: spec.messageID,
		ThroughTurnID: snapshot.cut.TurnID, MessagesJSON: string(encoded),
		MessageCount: stats.MessageCount, Bytes: len(encoded),
		Truncated: stats.Truncated, OmittedCount: stats.OmittedCount,
		SourceTurnInProgress: snapshot.sourceTurnInProgress,
		SourceEpoch:          snapshot.epoch, SourceGeneration: snapshot.generation,
		SourceMaxSequence: snapshot.maxSequence, CreatedAt: m.now().UTC(),
	}, nil
}

func deriveOutcome(
	imported *store.SessionImportedContext,
	firstPrompt store.SessionFirstPrompt,
) store.SessionDerivationOutcome {
	return store.SessionDerivationOutcome{
		Seed: store.SessionDerivationSeedReplay, OriginMessageID: imported.OriginMessageID,
		ThroughTurnID: imported.ThroughTurnID, ReplayMessageCount: imported.MessageCount,
		ReplayBytes: imported.Bytes, Truncated: imported.Truncated, OmittedCount: imported.OmittedCount,
		SourceTurnInProgress: imported.SourceTurnInProgress, SourceEpoch: imported.SourceEpoch,
		SourceGeneration: imported.SourceGeneration, SourceMaxSequence: imported.SourceMaxSequence,
		FirstPrompt: firstPrompt.State, FirstAdmissionKey: firstPrompt.AdmissionKey,
	}
}

func (m *Manager) deriveBudget() replayBudget {
	cfg := m.deriveConfig
	if cfg.MaxReplayBytes <= 0 {
		cfg.MaxReplayBytes = compozyconfig.DefaultSessionDeriveMaxReplayBytes
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = compozyconfig.DefaultSessionDeriveMaxMessageBytes
	}
	return replayBudget{
		MaxBytes: cfg.MaxReplayBytes, MaxMessageBytes: cfg.MaxMessageBytes, KeepRecent: deriveProtectedTail,
	}
}

func deriveChildName(requested string, sourceName string) string {
	if name := strings.TrimSpace(requested); name != "" {
		return name
	}
	return strings.TrimSpace(sourceName)
}

func deriveSourceCWD(meta store.SessionMeta) string {
	if meta.CreationProfile != nil && strings.TrimSpace(meta.CreationProfile.CWD) != "" {
		return strings.TrimSpace(meta.CreationProfile.CWD)
	}
	return strings.TrimSpace(meta.CWDValue())
}

func deriveFirstAdmissionKey(key string) string {
	return strings.TrimSpace(key) + ":first"
}

// deriveFirstMessageID is deterministic so a retried derive re-admits the same message.
func deriveFirstMessageID(workspaceID string, key string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(workspaceID) + "\x00" + strings.TrimSpace(key)))
	return "msg_derive_" + hex.EncodeToString(digest[:12])
}
