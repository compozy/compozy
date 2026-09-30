package session

import (
	"context"
	"errors"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	redactpkg "github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// Reasons a fork does not use the agent's own session clone (internal logs only).
const (
	nativeForkReasonMessageCut   = "message_cut"
	nativeForkReasonUnbound      = "source_unbound"
	nativeForkReasonBusy         = "source_busy"
	nativeForkReasonCapabilities = "capabilities"
	nativeForkReasonAgent        = "agent_changed"
	nativeForkReasonRoute        = "route_incompatible"
	nativeForkReasonDriver       = "driver_unsupported"
	nativeForkReasonQueueUnknown = "queue_unknown"
	nativeForkReasonSourceMoved  = "source_moved"
)

// nativeForkDecision reports whether a fork can use the agent's own session clone now.
type nativeForkDecision struct {
	Eligible bool
	Reason   string
}

// forkChildRoute is the route a fork child's first bind resolves to: the source's
// selected runtime, on the source's accepted account when that route is still declared.
type forkChildRoute struct {
	resolved compozyconfig.ResolvedAgent
	pending  *store.SessionPendingRoute
	route    *FallbackRoute
}

// nativeForkEligible is the ADR-003 gate: whole-session cut ∧ source bound on a live
// process ∧ idle (no prompt in setup or flight, nothing queued) ∧ the process advertises
// session/fork and session/load ∧ same agent ∧ the child's route is compatible with the
// source's (provider, auth mode, home policy, env policy, harness, runtime provider,
// transport, base URL, and command fingerprint).
func nativeForkEligible(
	source *Session,
	messageID string,
	childAgent string,
	childRoute compozyconfig.ResolvedAgent,
	queuedInputs int,
) nativeForkDecision {
	if strings.TrimSpace(messageID) != "" {
		return nativeForkDecision{Reason: nativeForkReasonMessageCut}
	}
	if source == nil {
		return nativeForkDecision{Reason: nativeForkReasonUnbound}
	}
	binding := source.runtimeBindingSnapshot()
	if binding.process == nil || binding.status != RuntimeStatusReady || isProcessDone(binding.process) {
		return nativeForkDecision{Reason: nativeForkReasonUnbound}
	}
	if source.IsPrompting() || queuedInputs > 0 {
		return nativeForkDecision{Reason: nativeForkReasonBusy}
	}
	caps := binding.process.CapsSnapshot()
	if !caps.SupportsForkSession || !caps.SupportsLoadSession {
		return nativeForkDecision{Reason: nativeForkReasonCapabilities}
	}
	if strings.TrimSpace(source.Info().AgentName) != strings.TrimSpace(childAgent) {
		return nativeForkDecision{Reason: nativeForkReasonAgent}
	}
	sourceRoute := source.providerRoutingSnapshot()
	if !compatibleSpawnProviderRoute(sourceRoute, childRoute) ||
		providerCommandFingerprint(sourceRoute.Command) != providerCommandFingerprint(childRoute.Command) {
		return nativeForkDecision{Reason: nativeForkReasonRoute}
	}
	return nativeForkDecision{Eligible: true}
}

// forkNativeDecision evaluates the gate for the live source of a snapshot. An unknown
// input-queue state is not eligible.
func (m *Manager) forkNativeDecision(
	ctx context.Context,
	snapshot *deriveSnapshot,
	messageID string,
	childAgent string,
	childRoute compozyconfig.ResolvedAgent,
) nativeForkDecision {
	if strings.TrimSpace(messageID) != "" {
		return nativeForkDecision{Reason: nativeForkReasonMessageCut}
	}
	if _, ok := m.driver.(ForkDriver); !ok {
		return nativeForkDecision{Reason: nativeForkReasonDriver}
	}
	source, ok := m.Get(snapshot.meta.ID)
	if !ok {
		return nativeForkDecision{Reason: nativeForkReasonUnbound}
	}
	queue, err := m.InputQueueSummary(ctx, source.ID)
	if err != nil {
		return nativeForkDecision{Reason: nativeForkReasonQueueUnknown}
	}
	return nativeForkEligible(source, messageID, childAgent, childRoute, queue.PendingInputs)
}

// deriveSeedOutcome is how a derived child is seeded: replay, or a pinned native clone.
type deriveSeedOutcome struct {
	seed        string
	native      *store.SessionNativeBootstrap
	nativeError string
}

// deriveForkSeed decides the fork's seed. The carried context is built either way, so a
// native child keeps its fallback; a failed attempt records its error and replays.
func (m *Manager) deriveForkSeed(
	ctx context.Context,
	spec deriveSpec,
	snapshot *deriveSnapshot,
	agentName string,
	childRoute compozyconfig.ResolvedAgent,
) deriveSeedOutcome {
	replay := deriveSeedOutcome{seed: store.SessionDerivationSeedReplay}
	decision := m.forkNativeDecision(ctx, snapshot, spec.messageID, agentName, childRoute)
	if !decision.Eligible {
		m.logger.Debug(
			"session.fork.native_not_eligible", "source_session_id", snapshot.meta.ID, "reason", decision.Reason,
		)
		return replay
	}
	source, ok := m.Get(snapshot.meta.ID)
	if !ok {
		return replay
	}
	native, errText := m.attemptNativeFork(ctx, source, snapshot, childRoute)
	if native == nil {
		replay.nativeError = errText
		return replay
	}
	return deriveSeedOutcome{seed: store.SessionDerivationSeedNativeFork, native: native}
}

// previewNativeForkPossible reports whether a whole-session fork of the snapshot's source
// would use the agent's own clone right now; false when not eligible or unknown.
func (m *Manager) previewNativeForkPossible(ctx context.Context, snapshot *deriveSnapshot) bool {
	meta := snapshot.meta
	agentName := strings.TrimSpace(meta.AgentName)
	agentDef, workspace, err := m.resolveDeriveTargetAgent(ctx, &meta, agentName)
	if err != nil {
		return false
	}
	fork, err := m.resolveForkChildRoute(&workspace, agentDef, &meta)
	if err != nil {
		return false
	}
	return m.forkNativeDecision(ctx, snapshot, "", agentName, fork.resolved).Eligible
}

// attemptNativeFork runs session/fork on the source's process while holding its exclusive
// prompt slot, so no prompt starts during the call. It returns the pinned bootstrap
// (state pending), or the redacted, bounded error of a failed attempt; a source that
// became busy, or whose conversation moved past the snapshot, returns neither and the
// fork replays: the clone must hold exactly the conversation the receipt records.
func (m *Manager) attemptNativeFork(
	ctx context.Context,
	source *Session,
	snapshot *deriveSnapshot,
	childRoute compozyconfig.ResolvedAgent,
) (*store.SessionNativeBootstrap, string) {
	forker, ok := m.driver.(ForkDriver)
	if !ok || source == nil || snapshot == nil || snapshot.sourceTurnInProgress {
		return nil, ""
	}
	proc, err := source.beginExclusivePromptSetup()
	if err != nil {
		m.sessionLogger(source).Info("session.fork.native_skipped", "reason", nativeForkReasonBusy, "error", err)
		return nil, ""
	}
	defer source.finishPromptSetup()
	if proc == nil || isProcessDone(proc) {
		return nil, ""
	}
	if moved, err := m.deriveSourceMoved(ctx, snapshot); err != nil || moved {
		m.sessionLogger(source).Info(
			"session.fork.native_skipped", "reason", nativeForkReasonSourceMoved, "error", err,
		)
		return nil, ""
	}
	result, err := forker.ForkSession(ctx, proc, proc.Cwd)
	if err != nil {
		text := nativeForkErrorText(err, proc.Command)
		m.sessionLogger(source).Warn("session.fork.native_failed", "error", text)
		return nil, text
	}
	return &store.SessionNativeBootstrap{
		ACPSessionID:       strings.TrimSpace(string(result.SessionID)),
		Provider:           strings.TrimSpace(childRoute.Provider),
		AuthMode:           string(childRoute.AuthMode),
		HomePolicy:         string(childRoute.HomePolicy),
		CommandFingerprint: providerCommandFingerprint(childRoute.Command),
		State:              store.SessionNativeStatePending,
	}, ""
}

// nativeBootstrapMatchesRoute reports whether a resolved route is the provider identity
// the clone was created under; only then may the child load it.
func nativeBootstrapMatchesRoute(native store.SessionNativeBootstrap, route compozyconfig.ResolvedAgent) bool {
	return strings.TrimSpace(route.Provider) == native.Provider &&
		string(route.AuthMode) == native.AuthMode &&
		string(route.HomePolicy) == native.HomePolicy &&
		providerCommandFingerprint(route.Command) == native.CommandFingerprint
}

// nativeForkErrorText renders a native fork or load failure for the read model: the
// source command never appears, secrets are redacted, and the text is bounded.
func nativeForkErrorText(err error, command string) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if prefix := "acp: session/fork \""; strings.HasPrefix(text, prefix) {
		if index := strings.Index(text[len(prefix):], "\": "); index >= 0 {
			text = "session/fork: " + text[len(prefix)+index+3:]
		}
	}
	if command = strings.TrimSpace(command); command != "" {
		text = strings.ReplaceAll(text, command, redactpkg.Marker)
	}
	return diagnostics.RedactAndBound(text, maxSessionFailureSummaryBytes)
}

// resolveForkChildRoute resolves the route the fork child binds on: the source's selected
// runtime, and — when the source runs on an overridden account (accepted route attempt
// > 0) — that declared route, matched by record, as the child's pending route (ADR-008).
// An accepted route no longer declared compatibly is not inherited; the child resolves
// its default route and the reason is logged.
func (m *Manager) resolveForkChildRoute(
	workspace *workspacepkg.ResolvedWorkspace,
	agentDef compozyconfig.AgentDef,
	meta *store.SessionMeta,
) (forkChildRoute, error) {
	if workspace == nil || meta == nil {
		return forkChildRoute{}, errors.New("session: fork workspace and source metadata are required")
	}
	primary, err := workspace.Config.ResolveSessionAgentWithRuntime(agentDef, compozyconfig.RuntimeOverrides{
		Provider: strings.TrimSpace(meta.Provider), Model: strings.TrimSpace(meta.Model),
		Reasoning: strings.TrimSpace(meta.ReasoningEffort),
	})
	if err != nil {
		return forkChildRoute{}, err
	}
	record := store.CloneSessionAcceptedRoute(meta.AcceptedRoute)
	if record == nil || record.Attempt <= 0 {
		return forkChildRoute{resolved: primary}, nil
	}
	if acceptedRouteMatchesResolved(*record, primary) {
		return forkChildRoute{resolved: primary}, nil
	}
	for index, route := range fallbackRoutesForAgent(agentDef) {
		resolved, resolveErr := workspace.Config.ResolveSessionAgentWithRuntime(
			agentDef,
			compozyconfig.RuntimeOverrides{
				Provider: route.Provider, Model: route.Model, Reasoning: route.ReasoningEffort, Command: route.Command,
			},
		)
		if resolveErr != nil || !acceptedRouteMatchesResolved(*record, resolved) {
			continue
		}
		pending, selected, pendingErr := derivePendingRoute(agentDef.Name, agentDef, index+1)
		if pendingErr != nil {
			return forkChildRoute{}, pendingErr
		}
		inheritSourceRuntime(pending, meta)
		return forkChildRoute{resolved: resolved, pending: pending, route: selected}, nil
	}
	m.logger.Info(
		"session.fork.accepted_route_not_inherited",
		"source_session_id", meta.ID,
		"reason", nativeForkReasonRoute,
		"accepted_route.provider", record.Provider,
		"accepted_route.attempt", record.Attempt,
	)
	return forkChildRoute{resolved: primary}, nil
}

// inheritSourceRuntime makes the inherited pending route carry the source's selected
// runtime, which the fork child copies too: the route index and command fingerprint name
// the account, and the runtime the source changed to live on that account is not an
// explicit child selection that would replace it at the first bind.
func inheritSourceRuntime(pending *store.SessionPendingRoute, meta *store.SessionMeta) {
	runtime := forkRuntimeFromMeta(meta)
	if pending == nil || runtime == nil {
		return
	}
	pending.Provider, pending.Model = runtime.Provider, runtime.Model
	pending.ReasoningEffort, pending.Speed = runtime.ReasoningEffort, runtime.Speed
	pending.ACPOptions = storeOptionSelectionsFromACP(runtime.ACPOptions)
}

func acceptedRouteMatchesResolved(record store.SessionAcceptedRoute, resolved compozyconfig.ResolvedAgent) bool {
	return strings.TrimSpace(resolved.Provider) == record.Provider &&
		string(resolved.AuthMode) == record.AuthMode &&
		string(resolved.HomePolicy) == record.HomePolicy &&
		providerCommandFingerprint(resolved.Command) == record.CommandFingerprint
}
