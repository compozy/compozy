package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

const sessionDerivedEventWriteTimeout = 5 * time.Second

// sessionDerivedEventPayload is the session.derived ledger payload.
type sessionDerivedEventPayload struct {
	Kind               store.LineageKind `json:"kind"`
	SourceSessionID    string            `json:"source_session_id"`
	OriginMessageID    string            `json:"origin_message_id,omitempty"`
	OriginAgentName    string            `json:"origin_agent_name"`
	ThroughTurnID      string            `json:"through_turn_id"`
	Seed               string            `json:"seed"`
	NativeState        string            `json:"native_state,omitempty"`
	ReplayMessageCount int               `json:"replay_message_count"`
	ReplayBytes        int               `json:"replay_bytes"`
	Truncated          bool              `json:"truncated"`
	OmittedCount       int               `json:"omitted_count"`
	NativeForkError    string            `json:"native_fork_error,omitempty"`
	FirstPrompt        string            `json:"first_prompt"`
	IdempotencyKey     string            `json:"idempotency_key"`
}

func cloneDerivationReceipt(receipt *store.SessionDerivationReceipt) *store.SessionDerivationReceipt {
	if receipt == nil {
		return nil
	}
	cloned := *receipt
	return &cloned
}

// registerDerivedSession commits a derived child: catalog row, creation identity, and
// derive receipt in one global-DB transaction. Nothing publishes the child before it.
func (m *Manager) registerDerivedSession(
	ctx context.Context,
	session *Session,
	receipt store.SessionDerivationReceipt,
) error {
	derivations, err := m.derivationStore()
	if err != nil {
		return err
	}
	meta := session.Meta()
	identity := creationIdentityFromMeta(&meta)
	if identity == nil || meta.CreationProfile == nil || m.creationStore == nil {
		return fmt.Errorf("session: derived session %q requires a creation identity", meta.ID)
	}
	profileRef, err := m.creationStore.PutSessionCreationProfile(ctx, *meta.CreationProfile)
	if err != nil {
		return fmt.Errorf("session: persist creation profile for %q: %w", meta.ID, err)
	}
	if profileRef != identity.CreationProfileRef {
		return fmt.Errorf("session: creation profile ref changed for %q", meta.ID)
	}
	if err := derivations.RegisterDerivedSession(
		ctx, sessionCatalogInfoFromRuntime(session.Info()), *identity, receipt,
	); err != nil {
		return fmt.Errorf("session: register derived session %q: %w", meta.ID, err)
	}
	// The commit is irreversible: the receipt now names this child, so nothing after it
	// may fail the registration or sweep the child. The attention projection of a child
	// that was never visible is empty, and the next lifecycle write hydrates it again.
	session.markDeriveCommitted()
	if err := m.hydrateSessionAttention(ctx, session); err != nil {
		m.sessionLogger(session).Warn("session.derived.attention_hydrate_failed", "error", err)
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, session.Info()))
	return nil
}

// recordSessionDerivedEvent appends session.derived for the committed child to the
// daemon ledger. The commit already happened; a ledger failure is logged, not fatal —
// boot reconciliation (ReconcileDerivedSessionEvents) re-emits a missing event.
func (m *Manager) recordSessionDerivedEvent(ctx context.Context, spec *sessionStartSpec, session *Session) {
	if m.eventLedger == nil || spec == nil || spec.derivation == nil || spec.deriveReceipt == nil {
		return
	}
	if err := m.writeSessionDerivedEvent(
		ctx,
		session.Info(),
		*spec.derivation,
		spec.deriveReceipt.Outcome,
	); err != nil {
		m.sessionLogger(session).Warn("session.derived.record_failed", "error", err)
	}
}

// writeSessionDerivedEvent writes one session.derived ledger row for a committed child.
func (m *Manager) writeSessionDerivedEvent(
	ctx context.Context,
	info *Info,
	derivation store.SessionDerivation,
	outcome store.SessionDerivationOutcome,
) error {
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	payload := sessionDerivedEventPayload{
		Kind: derivation.Kind, SourceSessionID: derivation.SourceSessionID,
		OriginMessageID: outcome.OriginMessageID, ThroughTurnID: outcome.ThroughTurnID,
		Seed: outcome.Seed, NativeState: outcome.NativeState,
		ReplayMessageCount: outcome.ReplayMessageCount, ReplayBytes: outcome.ReplayBytes,
		Truncated: outcome.Truncated, OmittedCount: outcome.OmittedCount,
		NativeForkError: outcome.NativeForkError, FirstPrompt: outcome.FirstPrompt,
		IdempotencyKey: derivation.IdempotencyKey,
	}
	if lineage != nil {
		payload.OriginAgentName = lineage.OriginAgentName
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("session: encode session.derived for %q: %w", info.ID, err)
	}
	summary := store.EventSummary{
		Type:        eventspkg.SessionDerived,
		AgentName:   info.AgentName,
		Outcome:     string(eventspkg.OutcomeFor(eventspkg.SessionDerived)),
		Summary:     fmt.Sprintf("%s from %s (%s)", payload.Kind, payload.SourceSessionID, payload.Seed),
		Timestamp:   m.now(),
		ProfileID:   strings.TrimSpace(info.ProfileID),
		SessionID:   info.ID,
		WorkspaceID: info.WorkspaceID,
	}
	if lineage != nil {
		summary.ParentSessionID = lineage.ParentSessionID
		summary.RootSessionID = lineage.RootSessionID
		summary.SpawnDepth = lineage.SpawnDepth
	}
	if summary.ProfileID == "" {
		summary.ProfileID = store.DefaultProfileID
	}
	summary.SetContent(content)
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sessionDerivedEventWriteTimeout)
	defer cancel()
	if err := m.eventLedger.WriteEventSummary(writeCtx, summary); err != nil {
		return fmt.Errorf("session: record session.derived for %q: %w", info.ID, err)
	}
	m.logger.Info(
		"session.derived",
		"derive.kind", payload.Kind,
		"derive.seed", payload.Seed,
		"source_session_id", payload.SourceSessionID,
		"child_session_id", info.ID,
		"replay_bytes", payload.ReplayBytes,
		"replay_message_count", payload.ReplayMessageCount,
		"truncated", payload.Truncated,
	)
	return nil
}

// replayDeriveReceipt answers a derive whose key already has a receipt: an identical
// request returns the recorded outcome (completing an undispatched first message), a
// different request is an idempotency conflict. No validation runs before it.
func (m *Manager) replayDeriveReceipt(
	ctx context.Context,
	derivations store.SessionDerivationStore,
	spec deriveSpec,
) (DeriveResult, bool, error) {
	receipt, found, err := derivations.SessionDerivationReceipt(ctx, spec.workspaceID, spec.key)
	if err != nil || !found {
		return DeriveResult{}, false, err
	}
	if receipt.RequestFingerprint != spec.fingerprint {
		return DeriveResult{}, true, deriveErr(
			ErrDeriveIdempotencyConflict, "idempotency key %s was used with a different request", spec.key,
		)
	}
	result := deriveResultFromReceipt(receipt)
	result.Replayed = true
	if receipt.ChildDeletedAt != nil {
		result.ChildDeleted = true
		return result, true, nil
	}
	child, err := m.Status(ctx, receipt.ChildSessionID)
	if errors.Is(err, ErrSessionNotFound) {
		result.ChildDeleted = true
		return result, true, nil
	}
	if err != nil {
		return DeriveResult{}, true, err
	}
	result.Child = child
	if lineage := store.NormalizeSessionLineage(child.ID, child.Lineage); lineage != nil {
		result.OriginAgentName = lineage.OriginAgentName
	}
	if receipt.Outcome.FirstAdmissionKey != "" && spec.message != "" &&
		!m.deriveFirstMessageDispatched(ctx, receipt.ChildSessionID, receipt.Outcome.FirstAdmissionKey) {
		if err := m.admitDeriveFirstMessage(ctx, spec, receipt.ChildSessionID); err != nil {
			return result, true, err
		}
	}
	return result, true, nil
}

func deriveResultFromReceipt(receipt store.SessionDerivationReceipt) DeriveResult {
	outcome := receipt.Outcome
	return DeriveResult{
		Kind: receipt.Kind, SourceSessionID: receipt.SourceSessionID, ChildSessionID: receipt.ChildSessionID,
		OriginMessageID: outcome.OriginMessageID, OriginAgentName: outcome.OriginAgentName,
		Seed:        DeriveSeed(outcome.Seed),
		NativeState: outcome.NativeState, ACPSessionID: outcome.ACPSessionID,
		NativeForkError: outcome.NativeForkError, ReplayMessageCount: outcome.ReplayMessageCount,
		ReplayBytes: outcome.ReplayBytes, Truncated: outcome.Truncated, OmittedCount: outcome.OmittedCount,
		SourceTurnInProgress: outcome.SourceTurnInProgress, ThroughTurnID: outcome.ThroughTurnID,
		FirstPrompt: outcome.FirstPrompt,
	}
}

// deriveFirstMessageDispatched reports whether the derive's first message already carried
// the imported context to the agent; only an undispatched admission is completed again
// (its runtime is still the one resolved for the unbound child).
func (m *Manager) deriveFirstMessageDispatched(ctx context.Context, childID string, admissionKey string) bool {
	meta, err := m.readSessionMetaReadOnly(ctx, childID)
	if err != nil || meta.ImportedContext == nil || meta.ImportedContext.Consumed == nil {
		return false
	}
	return meta.ImportedContext.Consumed.AdmissionKey == admissionKey
}
