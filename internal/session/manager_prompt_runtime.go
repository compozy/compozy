package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

type promptRuntimePlan struct {
	selection RuntimeSelection
	spec      sessionStartSpec
	runtime   sessionStartRuntime
	// attempt is the fallback chain index this plan binds (0 = primary route).
	attempt int
	// reloadACPSessionID rebinds a released runtime through session/load; a failed load
	// falls back to a transcript replay.
	reloadACPSessionID string
	// replayHistory carries the transcript into a bind that cannot load the prior session.
	replayHistory bool
}

func (m *Manager) ensurePromptRuntime(
	ctx context.Context,
	session *Session,
	requested *RuntimeSelection,
	reservedProcess *AgentProcess,
) (*AgentProcess, error) {
	// A live selection apply and prompt admission never reconfigure the same process at once.
	session.runtimeApplyMu.Lock()
	defer session.runtimeApplyMu.Unlock()
	selection, err := normalizePromptRuntimeSelection(session, requested)
	if err != nil {
		return nil, err
	}
	snapshot := session.runtimeBindingSnapshot()
	if reservedProcess != snapshot.process {
		return nil, errors.New("session: runtime process changed while reserving prompt")
	}
	controlsDrifted := runtimeControlsDrifted(&snapshot)
	if snapshot.process != nil && runtimeSelectionsEqual(snapshot.selection, *selection) &&
		!controlsDrifted {
		return snapshot.process, nil
	}
	plan, err := m.preparePromptRuntimePlan(ctx, session, *selection)
	if err != nil {
		return nil, err
	}
	if snapshot.process != nil && runtimeSelectionsEqual(snapshot.selection, plan.selection) &&
		!controlsDrifted {
		return snapshot.process, nil
	}
	if err := m.validateExplicitStartModel(ctx, &plan.runtime, &plan.spec); err != nil {
		return nil, err
	}
	if snapshot.process == nil {
		if proc, handled, err := m.bindUnboundPromptRuntime(ctx, session, &snapshot, plan); handled {
			return proc, err
		}
	}
	if snapshot.process != nil &&
		strings.TrimSpace(snapshot.selection.Provider) == strings.TrimSpace(plan.selection.Provider) {
		runtimeProvider := strings.TrimSpace(plan.runtime.agent.RuntimeProvider)
		if runtimeProvider == "" {
			runtimeProvider = strings.TrimSpace(plan.runtime.agent.Provider)
		}
		if RuntimeStrategyForProvider(runtimeProvider) == acp.RuntimeApplicationSessionConfig {
			if configurator, ok := m.driver.(RuntimeConfigurator); ok {
				return m.configureOrReplacePromptRuntime(ctx, session, configurator, &snapshot, plan)
			}
		}
	}

	return m.replacePromptRuntime(ctx, session, &snapshot, plan)
}

func (m *Manager) configureOrReplacePromptRuntime(
	ctx context.Context,
	session *Session,
	configurator RuntimeConfigurator,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
) (*AgentProcess, error) {
	refused, liveErr := m.configurePromptRuntime(ctx, session, configurator, snapshot, plan)
	if liveErr == nil {
		return snapshot.process, nil
	}
	if refused {
		return nil, liveErr
	}
	var proc *AgentProcess
	var replacementErr error
	if current := session.runtimeBindingSnapshot(); current.runtimeRelease != nil {
		proc, replacementErr = m.rebindReleasedRuntime(ctx, session, &current, plan)
	} else {
		proc, replacementErr = m.replacePromptRuntime(ctx, session, snapshot, plan)
	}
	if replacementErr == nil {
		return proc, nil
	}
	return nil, errors.Join(liveErr, replacementErr)
}

func runtimeControlsDrifted(snapshot *runtimeBindingSnapshot) bool {
	if snapshot.process == nil || snapshot.runtimeStrategy != acp.RuntimeApplicationSessionConfig {
		return false
	}
	caps, selection := snapshot.process.CapsSnapshot(), snapshot.selection
	if selection.ReasoningEffort != "" {
		if option, ok := acp.ReasoningConfigOption(caps.ConfigOptions); !ok ||
			strings.TrimSpace(option.CurrentValueID) != selection.ReasoningEffort {
			return true
		}
	}
	currentSpeed := acp.CurrentSpeed(caps.ConfigOptions)
	return selection.Speed != "" && acp.SupportsSpeed(selection.Speed, caps.ConfigOptions) &&
		currentSpeed != selection.Speed
}

// Provider negotiation refusals are retried only by a subsequent prompt request;
// replacing a healthy process cannot grant an unavailable capability or account tier.
func runtimeConfigurationRefused(err error, process *AgentProcess) bool {
	if _, ok := errors.AsType[*acp.NegotiationError](err); !ok || isProcessDone(process) {
		return false
	}
	if failure, ok := acp.FailureFromError(err, store.FailurePrompt); ok {
		switch failure.Kind {
		case store.FailureTransport, store.FailureProcess, store.FailureCanceled, store.FailureTimeout:
			return false
		}
	}
	return true
}

// bindUnboundPromptRuntime owns the binds that differ for a session without a process:
// a released idle runtime reloads its ACP session, a derived child binds its pending
// route, and a logical session's first bind walks the agent's fallback_chain (ADR-004).
func (m *Manager) bindUnboundPromptRuntime(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
) (*AgentProcess, bool, error) {
	if snapshot.runtimeRelease != nil {
		proc, err := m.rebindReleasedRuntime(ctx, session, snapshot, plan)
		return proc, true, err
	}
	if pending := session.pendingRouteSnapshot(); pending != nil {
		proc, handled, err := m.bindPendingRoute(ctx, session, snapshot, plan, *pending)
		if handled {
			return proc, true, err
		}
	}
	if routes := fallbackRoutesForAgent(plan.runtime.agentDef); len(routes) > 0 {
		proc, err := m.bindPromptRuntimeWithFallback(ctx, session, snapshot, plan, routes)
		return proc, true, err
	}
	return nil, false, nil
}

// configurePromptRuntime classifies the primary refusal before rollback or cleanup can change liveness.
func (m *Manager) configurePromptRuntime(
	ctx context.Context,
	session *Session,
	configurator RuntimeConfigurator,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
) (refused bool, err error) {
	if plan == nil {
		return false, errors.New("session: prompt runtime plan is required")
	}
	selection := plan.selection
	requestedConfig := runtimeConfigForSelection(selection, plan.spec.transportModel)
	rollbackConfig := runtimeConfigForSelection(
		session.Info().EffectiveRuntime(),
		currentTransportModel(snapshot.process.CapsSnapshot()),
	)
	if err := session.beginRuntimeTransition(
		RuntimeStatusReconfiguring,
		RuntimeTransitionLiveConfiguration,
		m.now(),
	); err != nil {
		return false, err
	}
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		session.restoreRuntimeBinding(snapshot, err.Error(), m.now())
		cleanupCtx, cancel := m.lifecycleCleanupContext()
		defer cancel()
		return false, errors.Join(err, m.persistSessionLifecycleState(cleanupCtx, session, false))
	}

	err = configurator.ConfigureRuntime(ctx, snapshot.process, requestedConfig)
	if err != nil {
		cleanupCtx, cancel := m.lifecycleCleanupContext()
		defer cancel()
		refused = runtimeConfigurationRefused(err, snapshot.process)
		var rollbackErr error
		if !runtimeSelectionsEqual(snapshot.selection, selection) ||
			!refused {
			rollbackErr = configurator.ConfigureRuntime(cleanupCtx, snapshot.process, rollbackConfig)
		}
		return refused, m.restoreConfiguredRuntime(
			cleanupCtx, session, snapshot, fmt.Errorf("session: configure runtime: %w", err), rollbackErr,
		)
	}

	session.completeRuntimeTransition(
		snapshot.process,
		selection,
		RuntimeTransitionLiveConfiguration,
		m.now(),
	)
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		cleanupCtx, cancel := m.lifecycleCleanupContext()
		defer cancel()
		rollbackErr := configurator.ConfigureRuntime(
			cleanupCtx,
			snapshot.process,
			rollbackConfig,
		)
		return false, m.restoreConfiguredRuntime(cleanupCtx, session, snapshot, err, rollbackErr)
	}
	// A live model switch keeps the process: later rebinds and a first turn that has not
	// sent its startup guidance yet must name the new model.
	if plan.runtime.startupRebuilt {
		session.setRuntimeDefinition(&plan.runtime)
		if replacer, ok := m.driver.(StartupPromptReplacer); ok {
			replacer.ReplacePendingStartupPrompt(
				snapshot.process, plan.runtime.agent.Prompt, plan.runtime.startupManifest,
			)
		}
	}
	return false, nil
}

// restoreConfiguredRuntime retains a binding only when its provider rollback succeeded.
// A failed rollback transfers process ownership to the existing verified-release path.
func (m *Manager) restoreConfiguredRuntime(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	cause error,
	rollbackErr error,
) error {
	failure := errors.Join(cause, rollbackErr)
	if rollbackErr == nil {
		session.restoreRuntimeBinding(snapshot, failure.Error(), m.now())
		return errors.Join(failure, m.persistSessionLifecycleState(ctx, session, false))
	}
	release := session.detachRuntimeAfterFailedRollback(snapshot, failure.Error(), m.now())
	persistErr := m.persistSessionLifecycleState(ctx, session, false)
	stopErr := m.stopReleasedRuntime(session, release)
	var identityErr error
	if stopErr == nil && session.clearReleasedRuntimeIdentity(release) {
		identityErr = m.persistSessionLifecycleState(ctx, session, false)
	}
	return errors.Join(failure, persistErr, stopErr, identityErr)
}

// replacePromptRuntime keeps the previous route authoritative until the new binding commits.
func (m *Manager) replacePromptRuntime(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
) (*AgentProcess, error) {
	strategy := RuntimeTransitionProcessReplacement
	status := RuntimeStatusReconfiguring
	if snapshot.process == nil {
		strategy = RuntimeTransitionInitialBind
		status = RuntimeStatusBinding
	}

	runtime := plan.runtime
	mcpServers, err := m.sessionMCPServers(ctx, &plan.spec, runtime.agent, runtime.agentDef)
	if err != nil {
		return nil, fmt.Errorf("session: resolve runtime MCP servers: %w", err)
	}
	runtime.mcpServers = mcpServers
	// A replacement rebuilds history; so does the first bind of a derived child, whose
	// carried context rides in front of its first prompt.
	plan.spec.resumeReplay = snapshot.process != nil || plan.replayHistory || session.hasImportedContext()
	plan.spec.startAction = "bind runtime"

	// A fork child's first bind loads its pending native clone instead of replaying.
	native := m.prepareNativeBootstrapBind(session, snapshot, plan)
	if native == nil && plan.reloadACPSessionID != "" {
		plan.spec.acpSessionID = plan.reloadACPSessionID
		plan.spec.resumeReplay = false
	}

	if err := session.beginRuntimeTransition(status, strategy, m.now()); err != nil {
		return nil, err
	}
	candidate, err := m.launchPromptRuntimeCandidate(ctx, session, plan, &runtime, native)
	if err != nil {
		return nil, m.restorePromptRuntime(session, snapshot, err)
	}

	previous := session.completeRuntimeTransition(candidate, plan.selection, strategy, m.now())
	// The accepted binding is committed with the ACP id in the same meta write.
	session.commitAcceptedRoute(
		acceptedRouteRecord(plan.attempt, runtime.agent, plan.selection.Model),
		plan.spec.command,
		plan.spec.sessionEnv,
	)
	session.setRuntimeDefinition(&runtime)
	if plan.spec.resumeReplay {
		session.setPendingResumeReplay(plan.spec.resumeReplayBlock)
	}
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		session.restoreRuntimeBinding(snapshot, err.Error(), m.now())
		cleanupCtx, cancel := m.lifecycleCleanupContext()
		defer cancel()
		restoreErr := m.persistSessionLifecycleState(cleanupCtx, session, false)
		stopErr := m.stopReplacedRuntime(session, candidate, false)
		// ACP already accepted the candidate: report an accepted failure (ADR-005).
		return nil, acp.WrapAcceptedStart(candidate.SessionID, errors.Join(err, restoreErr, stopErr))
	}

	session.setProviderRouting(runtime.agent)
	if plan.spec.resumeReplay {
		m.stageResumeReplay(session.ID, plan.spec.resumeReplayBlock)
	}
	if previous != nil {
		if stopErr := m.stopReplacedRuntime(session, previous, true); stopErr != nil {
			m.sessionLogger(session).Error("session: stop replaced runtime failed", "error", stopErr)
		}
	}
	m.dispatchAgentSpawned(ctx, session, candidate, runtime.agent)
	m.watchProcess(session)
	return candidate, nil
}

func (m *Manager) restorePromptRuntime(
	session *Session,
	snapshot *runtimeBindingSnapshot,
	cause error,
) error {
	session.restoreRuntimeBinding(snapshot, cause.Error(), m.now())
	cleanupCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	persistErr := m.persistSessionLifecycleState(cleanupCtx, session, false)
	return errors.Join(cause, persistErr)
}

func runtimeConfigForSelection(selection RuntimeSelection, transportModel string) acp.RuntimeConfig {
	model := strings.TrimSpace(transportModel)
	if model == "" {
		model = selection.Model
	}
	return acp.RuntimeConfig{
		Model:           model,
		ReasoningEffort: selection.ReasoningEffort,
		Speed:           selection.Speed,
		ACPOptions:      acp.CloneSessionConfigOptionSelections(selection.ACPOptions),
	}
}

func currentTransportModel(caps acp.Caps) string {
	model, ok := acp.ModelConfigOption(caps.ConfigOptions)
	if !ok {
		return ""
	}
	return strings.TrimSpace(model.CurrentValueID)
}

func (m *Manager) stopReplacedRuntime(session *Session, proc *AgentProcess, emitHook bool) error {
	if proc == nil {
		return nil
	}
	exited, err := m.processExitVerified(proc)
	if err != nil {
		return err
	}
	if !exited {
		stopCtx, cancel := m.runtimeStopContext()
		defer cancel()
		if err := m.driver.Stop(stopCtx, proc); err != nil {
			return err
		}
	}
	if emitHook {
		hookCtx, cancel := m.lifecycleCleanupContext()
		defer cancel()
		m.dispatchAgentStopped(hookCtx, session, proc, nil)
	}
	return nil
}

// preparePromptRuntimePlan leaves the running binding unchanged until the selected plan is applied.
func (m *Manager) preparePromptRuntimePlan(
	ctx context.Context,
	session *Session,
	selection RuntimeSelection,
) (*promptRuntimePlan, error) {
	return m.preparePromptRuntimePlanForRoute(ctx, session, selection, "", nil)
}

// preparePromptRuntimePlanForRoute plans a binding with an explicit route command
// (a fallback route's account or the accepted route's command); empty keeps resolution.
func (m *Manager) preparePromptRuntimePlanForRoute(
	ctx context.Context,
	session *Session,
	selection RuntimeSelection,
	command string,
	sessionEnv compozyconfig.SessionEnv,
) (*promptRuntimePlan, error) {
	if session == nil {
		return nil, ErrSessionNotFound
	}
	meta := session.Meta()
	workspace, err := m.resolveResumeWorkspace(ctx, &meta)
	if err != nil {
		return nil, fmt.Errorf("session: resolve runtime workspace: %w", err)
	}
	worktreeID, worktreeRoot, err := m.resolveSessionWorktree(ctx, workspace.ID, meta.WorktreeIDValue())
	if err != nil {
		return nil, fmt.Errorf("session: resolve runtime worktree: %w", err)
	}
	executionRoot := workspace.RootDir
	if worktreeRoot != "" {
		executionRoot = worktreeRoot
	}
	cwd, err := resumeSessionCWD(&meta, executionRoot)
	if err != nil {
		return nil, err
	}
	spec, err := sessionStartSpecFromMeta(&meta, &workspace, cwd)
	if err != nil {
		return nil, fmt.Errorf("session: reconstruct runtime start spec: %w", err)
	}
	spec.worktreeID = worktreeID
	spec.worktreeRoot = worktreeRoot
	spec.provider = selection.Provider
	spec.model = selection.Model
	spec.reasoningEffort = selection.ReasoningEffort
	spec.speed = selection.Speed
	spec.acpOptions = acp.CloneSessionConfigOptionSelections(selection.ACPOptions)
	spec.command = strings.TrimSpace(command)
	spec.sessionEnv = compozyconfig.MergeSessionEnv(nil, sessionEnv)
	// The bound route records whether an unrestricted preference was already present at session start.
	boundRoute := session.providerRoutingSnapshot()
	dropInheritedUnrestricted := hasUnrestrictedACPMode(boundRoute.ACPOptionsValue())
	runtime, err := m.resolveSessionStartRuntime(ctx, &spec, dropInheritedUnrestricted)
	if err != nil {
		return nil, err
	}
	if err := spec.materializeRootDelegationTools(runtime.agent, m.toolsetCatalog, m.toolUniverse); err != nil {
		return nil, fmt.Errorf("session: materialize delegation tools for %q: %w", spec.sessionID, err)
	}
	if err := acp.ValidateACPModePermissions(
		spec.acpOptions,
		m.startPermissions(startSpecPermissions(&spec, runtime.agent.Permissions)),
	); err != nil {
		return nil, fmt.Errorf("session: validate prompt runtime ACP mode: %w", err)
	}
	if runtime.agent.Harness != compozyconfig.ProviderHarnessPiACP {
		if err := m.preparePromptRuntimeStartupDefinition(ctx, session, &spec, &runtime); err != nil {
			return nil, err
		}
	}
	// Pi reassembles after readiness, using the authored prompt resolved above.

	resolvedSelection := RuntimeSelection{
		Provider:        runtime.agent.Provider,
		Model:           spec.model,
		ReasoningEffort: spec.reasoningEffort,
		Speed:           spec.speed,
		ACPOptions:      acp.CloneSessionConfigOptionSelections(spec.acpOptions),
	}
	if resolvedSelection.Model == "" {
		resolvedSelection.Model = runtime.agent.Model
	}
	return &promptRuntimePlan{
		selection: resolvedSelection, spec: spec, runtime: runtime,
	}, nil
}
