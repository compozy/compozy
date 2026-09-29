package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const deriveFingerprintVersion = "session-derive/v1"

// deriveSpec is the normalized input shared by continue and fork.
type deriveSpec struct {
	kind        store.LineageKind
	workspaceID string
	profileID   string
	sourceID    string
	agentName   string
	runtime     *DeriveRuntime
	route       int
	name        string
	message     string
	messageID   string
	key         string
	fingerprint string
	fences      DeriveFences
}

// ContinueSession continues a user session with another agent, runtime, or declared route.
// The source is only read; the child is committed with its derive receipt in one
// transaction; a retry with the same key returns the recorded outcome.
func (m *Manager) ContinueSession(ctx context.Context, opts ContinueSessionOpts) (DeriveResult, error) {
	if m == nil {
		return DeriveResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return DeriveResult{}, errors.New("session: continue context is required")
	}
	spec := deriveSpec{
		kind:        store.LineageKindContinue,
		workspaceID: strings.TrimSpace(opts.WorkspaceID),
		profileID:   strings.TrimSpace(opts.ProfileID),
		sourceID:    strings.TrimSpace(opts.SourceSessionID),
		agentName:   strings.TrimSpace(opts.AgentName),
		runtime:     normalizeDeriveRuntime(opts.Runtime),
		route:       opts.Route,
		name:        strings.TrimSpace(opts.Name),
		message:     strings.TrimSpace(opts.Message),
		key:         strings.TrimSpace(opts.IdempotencyKey),
		fences:      opts.Fences,
	}
	if spec.agentName == "" {
		return DeriveResult{}, fmt.Errorf("%w: agent name is required", ErrValidation)
	}
	if spec.runtime != nil && spec.route != 0 {
		return DeriveResult{}, fmt.Errorf("%w: runtime and route are mutually exclusive", ErrValidation)
	}
	if spec.route < 0 {
		return DeriveResult{}, fmt.Errorf("%w: route must be a positive 1-based index", ErrValidation)
	}
	return m.deriveSession(ctx, spec)
}

// ForkSession forks a user session with the same agent: the whole conversation through
// its last settled turn, or through one durable user message and its turn. When the
// agent can clone its own session (ADR-003) the child loads that clone at its first
// bind; otherwise, and as the clone's fallback, it carries the bounded context.
func (m *Manager) ForkSession(ctx context.Context, opts ForkSessionOpts) (DeriveResult, error) {
	if m == nil {
		return DeriveResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return DeriveResult{}, errors.New("session: fork context is required")
	}
	return m.deriveSession(ctx, deriveSpec{
		kind:        store.LineageKindFork,
		workspaceID: strings.TrimSpace(opts.WorkspaceID),
		profileID:   strings.TrimSpace(opts.ProfileID),
		sourceID:    strings.TrimSpace(opts.SourceSessionID),
		name:        strings.TrimSpace(opts.Name),
		messageID:   strings.TrimSpace(opts.MessageID),
		key:         strings.TrimSpace(opts.IdempotencyKey),
		fences:      opts.Fences,
	})
}

func (m *Manager) deriveSession(ctx context.Context, spec deriveSpec) (DeriveResult, error) {
	if err := validateDeriveSpec(spec); err != nil {
		return DeriveResult{}, err
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return DeriveResult{}, err
	}
	derivations, err := m.derivationStore()
	if err != nil {
		return DeriveResult{}, err
	}
	fingerprint, err := deriveRequestFingerprint(spec)
	if err != nil {
		return DeriveResult{}, err
	}
	spec.fingerprint = fingerprint
	unlock := m.lockDeriveAdmission(spec.workspaceID, spec.key)
	defer unlock()
	if result, found, err := m.replayDeriveReceipt(ctx, derivations, spec); err != nil || found {
		return result, err
	}
	snapshot, release, err := m.readDeriveSnapshot(ctx, spec.sourceID, spec.messageID)
	if err != nil {
		return DeriveResult{}, err
	}
	defer release()
	prepared, err := m.prepareDerive(ctx, spec, &snapshot)
	if err != nil {
		return DeriveResult{}, err
	}
	child, err := m.CreateAccepted(ctx, CreateAcceptedOpts{Session: prepared.createOpts})
	if errors.Is(err, store.ErrSessionDerivationExists) {
		release()
		result, found, replayErr := m.replayDeriveReceipt(ctx, derivations, spec)
		if replayErr != nil || !found {
			return DeriveResult{}, errors.Join(err, replayErr)
		}
		return result, nil
	}
	if err != nil {
		return DeriveResult{}, err
	}
	release()
	result := deriveResultFromReceipt(prepared.receipt)
	result.Child = child
	result.OriginAgentName = prepared.createOpts.Lineage.OriginAgentName
	if spec.message != "" {
		if err := m.admitDeriveFirstMessage(ctx, spec, child.ID); err != nil {
			return result, err
		}
		if refreshed, ok := m.Get(child.ID); ok {
			result.Child = refreshed.Info()
			if derivation := result.Child.Derivation; derivation != nil && derivation.Native != nil {
				result.NativeState = derivation.Native.State
			}
		}
	}
	return result, nil
}

func validateDeriveSpec(spec deriveSpec) error {
	switch {
	case spec.sourceID == "":
		return fmt.Errorf("%w: source session id is required", ErrValidation)
	case spec.key == "":
		return fmt.Errorf("%w: idempotency key is required", ErrValidation)
	case spec.workspaceID == "":
		return fmt.Errorf("%w: workspace id is required", ErrValidation)
	}
	fences := spec.fences
	set := 0
	for _, fence := range []*int64{fences.ExpectedEpoch, fences.ExpectedGeneration, fences.ExpectedMaxSequence} {
		if fence == nil {
			continue
		}
		if *fence < 0 {
			return fmt.Errorf("%w: transcript fences cannot be negative", ErrValidation)
		}
		set++
	}
	if set != 0 && set != 3 {
		return fmt.Errorf("%w: set all transcript fences together or omit all three", ErrValidation)
	}
	return nil
}

func normalizeDeriveRuntime(runtime *DeriveRuntime) *DeriveRuntime {
	if runtime == nil {
		return nil
	}
	normalized := DeriveRuntime{
		Provider:        strings.TrimSpace(runtime.Provider),
		Model:           strings.TrimSpace(runtime.Model),
		ReasoningEffort: strings.TrimSpace(runtime.ReasoningEffort),
		Speed:           runtime.Speed,
		ACPOptions:      acp.CloneSessionConfigOptionSelections(runtime.ACPOptions),
	}
	if normalized.Provider == "" && normalized.Model == "" && normalized.ReasoningEffort == "" &&
		normalized.Speed == "" && len(normalized.ACPOptions) == 0 {
		return nil
	}
	return &normalized
}

// deriveRequestFingerprint identifies one derive request; fences are excluded so a retry
// after the source moved still matches its recorded outcome.
func deriveRequestFingerprint(spec deriveSpec) (string, error) {
	canonical := struct {
		Version   string         `json:"version"`
		Operation string         `json:"operation"`
		Source    string         `json:"source_session_id"`
		Workspace string         `json:"workspace_id"`
		Profile   string         `json:"profile_id"`
		Agent     string         `json:"agent_name,omitempty"`
		Runtime   *DeriveRuntime `json:"runtime,omitempty"`
		Route     int            `json:"route,omitempty"`
		Name      string         `json:"name,omitempty"`
		Message   string         `json:"message,omitempty"`
		MessageID string         `json:"message_id,omitempty"`
	}{
		Version: deriveFingerprintVersion, Operation: string(spec.kind), Source: spec.sourceID,
		Workspace: spec.workspaceID, Profile: spec.profileID, Agent: spec.agentName, Runtime: spec.runtime,
		Route: spec.route, Name: spec.name, Message: spec.message, MessageID: spec.messageID,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("session: encode derive fingerprint: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (m *Manager) derivationStore() (store.SessionDerivationStore, error) {
	derivations, ok := m.creationStore.(store.SessionDerivationStore)
	if !ok || derivations == nil {
		return nil, errors.New("session: derivation receipt store is unavailable")
	}
	return derivations, nil
}

// lockDeriveAdmission serializes one (workspace, key) like lockPromptAdmission.
func (m *Manager) lockDeriveAdmission(workspaceID string, key string) func() {
	return m.lockPromptAdmission(workspaceID, "\x00derive", key)
}

// validateDeriveSource refuses sources that cannot be continued or forked.
func (m *Manager) validateDeriveSource(ctx context.Context, spec deriveSpec, snapshot *deriveSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, spec.sourceID)
	}
	meta := &snapshot.meta
	if strings.TrimSpace(meta.WorkspaceID) != spec.workspaceID {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, spec.sourceID)
	}
	lineage := store.NormalizeSessionLineage(meta.ID, meta.Lineage)
	if normalizeSessionType(Type(meta.SessionType)) != SessionTypeUser ||
		(lineage != nil && lineage.Kind == store.LineageKindSpawn) {
		return fmt.Errorf("%w: %s", ErrSessionNotDerivable, spec.sourceID)
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, meta.ID); err != nil {
		if errors.Is(err, ErrSessionArchived) {
			return deriveErr(ErrDeriveSourceArchived, "session %s is archived; unarchive it first", spec.sourceID)
		}
		return err
	}
	fences := spec.fences
	if fences.ExpectedEpoch != nil && (*fences.ExpectedEpoch != snapshot.epoch ||
		*fences.ExpectedGeneration != snapshot.generation ||
		*fences.ExpectedMaxSequence != snapshot.maxSequence) {
		return deriveErr(ErrDeriveFenceConflict, "transcript changed since the fences were read")
	}
	return nil
}

// resolveDeriveTargetAgent resolves the child's agent in the source workspace.
func (m *Manager) resolveDeriveTargetAgent(
	ctx context.Context,
	meta *store.SessionMeta,
	agentName string,
) (compozyconfig.AgentDef, workspacepkg.ResolvedWorkspace, error) {
	resolved, err := m.resolveResumeWorkspace(ctx, meta)
	if err != nil {
		return compozyconfig.AgentDef{}, workspacepkg.ResolvedWorkspace{}, err
	}
	artifacts, err := m.resolveWorkspaceAgentArtifactsForSession(agentName, SessionTypeUser, &resolved)
	if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		return compozyconfig.AgentDef{}, resolved, deriveErr(ErrDeriveAgentNotFound, "no agent named %q", agentName)
	}
	if err != nil {
		return compozyconfig.AgentDef{}, resolved, err
	}
	return artifacts.Agent, resolved, nil
}
