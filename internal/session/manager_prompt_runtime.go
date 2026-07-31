package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

type promptRuntimePlan struct {
	selection RuntimeSelection
	spec      sessionStartSpec
	runtime   sessionStartRuntime
}

func (m *Manager) ensurePromptRuntime(
	ctx context.Context,
	session *Session,
	requested *RuntimeSelection,
	reservedProcess *AgentProcess,
) (*AgentProcess, error) {
	selection, err := normalizePromptRuntimeSelection(session, requested)
	if err != nil {
		return nil, err
	}
	snapshot := session.runtimeBindingSnapshot()
	if reservedProcess != snapshot.process {
		return nil, errors.New("session: runtime process changed while reserving prompt")
	}
	if snapshot.process != nil && runtimeSelectionsEqual(snapshot.selection, *selection) {
		return snapshot.process, nil
	}
	plan, err := m.preparePromptRuntimePlan(ctx, session, *selection)
	if err != nil {
		return nil, err
	}
	if snapshot.process != nil && runtimeSelectionsEqual(snapshot.selection, plan.selection) {
		return snapshot.process, nil
	}

	if snapshot.process != nil &&
		strings.TrimSpace(snapshot.selection.Provider) == strings.TrimSpace(plan.selection.Provider) {
		if configurator, ok := m.driver.(RuntimeConfigurator); ok {
			liveErr := m.configurePromptRuntime(ctx, session, configurator, &snapshot, plan.selection)
			if liveErr == nil {
				return snapshot.process, nil
			}
			proc, replacementErr := m.replacePromptRuntime(ctx, session, &snapshot, plan)
			if replacementErr == nil {
				return proc, nil
			}
			return nil, errors.Join(liveErr, replacementErr)
		}
	}

	return m.replacePromptRuntime(ctx, session, &snapshot, plan)
}

func (m *Manager) configurePromptRuntime(
	ctx context.Context,
	session *Session,
	configurator RuntimeConfigurator,
	snapshot *runtimeBindingSnapshot,
	selection RuntimeSelection,
) error {
	if err := session.beginRuntimeTransition(
		RuntimeStatusReconfiguring,
		RuntimeTransitionLiveConfiguration,
		m.now(),
	); err != nil {
		return err
	}
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		session.restoreRuntimeBinding(snapshot, err.Error(), m.now())
		cleanupCtx, cancel := m.runtimeCleanupContext()
		defer cancel()
		return errors.Join(err, m.persistSessionLifecycleState(cleanupCtx, session, false))
	}

	err := configurator.ConfigureRuntime(ctx, snapshot.process, runtimeConfigForSelection(selection))
	if err != nil {
		cleanupCtx, cancel := m.runtimeCleanupContext()
		defer cancel()
		rollbackErr := configurator.ConfigureRuntime(
			cleanupCtx,
			snapshot.process,
			runtimeConfigForSelection(snapshot.selection),
		)
		session.restoreRuntimeBinding(snapshot, err.Error(), m.now())
		persistErr := m.persistSessionLifecycleState(cleanupCtx, session, false)
		return errors.Join(fmt.Errorf("session: configure runtime: %w", err), rollbackErr, persistErr)
	}

	session.completeRuntimeTransition(
		snapshot.process,
		selection,
		RuntimeTransitionLiveConfiguration,
		m.now(),
	)
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		cleanupCtx, cancel := m.runtimeCleanupContext()
		defer cancel()
		rollbackErr := configurator.ConfigureRuntime(
			cleanupCtx,
			snapshot.process,
			runtimeConfigForSelection(snapshot.selection),
		)
		session.restoreRuntimeBinding(snapshot, err.Error(), m.now())
		restoreErr := m.persistSessionLifecycleState(cleanupCtx, session, false)
		return errors.Join(err, rollbackErr, restoreErr)
	}
	return nil
}

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
	plan.spec.resumeReplay = snapshot.process != nil
	plan.spec.startAction = "bind runtime"

	if err := session.beginRuntimeTransition(status, strategy, m.now()); err != nil {
		return nil, err
	}
	startOpts, err := m.prepareSessionLaunch(ctx, &plan.spec, session, &runtime, nil)
	if err != nil {
		return nil, m.restorePromptRuntime(session, snapshot, err)
	}
	candidate, err := m.startAgentProcess(ctx, &plan.spec, session, startOpts)
	if err != nil {
		return nil, m.restorePromptRuntime(session, snapshot, err)
	}

	previous := session.completeRuntimeTransition(candidate, plan.selection, strategy, m.now())
	session.setAgentDefinition(runtime.agentDef)
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		session.restoreRuntimeBinding(snapshot, err.Error(), m.now())
		cleanupCtx, cancel := m.runtimeCleanupContext()
		defer cancel()
		restoreErr := m.persistSessionLifecycleState(cleanupCtx, session, false)
		stopErr := m.stopReplacedRuntime(session, candidate, false)
		return nil, errors.Join(err, restoreErr, stopErr)
	}

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
	cleanupCtx, cancel := m.runtimeCleanupContext()
	defer cancel()
	persistErr := m.persistSessionLifecycleState(cleanupCtx, session, false)
	return errors.Join(cause, persistErr)
}

func (m *Manager) runtimeCleanupContext() (context.Context, context.CancelFunc) {
	base := m.lifecycleCtx
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(base), defaultLifecycleTimeout)
}

func runtimeConfigForSelection(selection RuntimeSelection) acp.RuntimeConfig {
	return acp.RuntimeConfig{
		Model:           selection.Model,
		ReasoningEffort: selection.ReasoningEffort,
		Speed:           selection.Speed,
	}
}

func (m *Manager) stopReplacedRuntime(session *Session, proc *AgentProcess, emitHook bool) error {
	if proc == nil {
		return nil
	}
	stopCtx, cancel := m.runtimeCleanupContext()
	defer cancel()
	if err := m.driver.Stop(stopCtx, proc); err != nil {
		return err
	}
	if emitHook {
		m.dispatchAgentStopped(stopCtx, session, proc, nil)
	}
	return nil
}

func (m *Manager) preparePromptRuntimePlan(
	ctx context.Context,
	session *Session,
	selection RuntimeSelection,
) (*promptRuntimePlan, error) {
	if session == nil {
		return nil, ErrSessionNotFound
	}
	meta := session.Meta()
	workspace, err := m.resolveResumeWorkspace(ctx, meta)
	if err != nil {
		return nil, fmt.Errorf("session: resolve runtime workspace: %w", err)
	}
	if err := validateSessionParticipationWorkspace(meta.NetworkSpecSnapshot(), workspace.ID); err != nil {
		return nil, err
	}
	cwd, err := resumeSessionCWD(meta, workspace.RootDir)
	if err != nil {
		return nil, err
	}

	spec, err := sessionStartSpecFromMeta(meta, workspace, cwd)
	if err != nil {
		return nil, fmt.Errorf("session: reconstruct runtime start spec: %w", err)
	}
	spec.provider = selection.Provider
	spec.model = selection.Model
	spec.reasoningEffort = selection.ReasoningEffort
	spec.speed = selection.Speed
	runtime, err := m.resolveSessionStartRuntime(ctx, &spec)
	if err != nil {
		return nil, err
	}
	existingDefinition := session.AgentDefinition()
	runtime.agentDef.Prompt = existingDefinition.Prompt
	runtime.agent.Prompt = existingDefinition.Prompt

	resolvedSelection := RuntimeSelection{
		Provider:        runtime.agent.Provider,
		Model:           spec.model,
		ReasoningEffort: spec.reasoningEffort,
		Speed:           spec.speed,
	}
	if resolvedSelection.Model == "" {
		resolvedSelection.Model = runtime.agent.Model
	}
	return &promptRuntimePlan{selection: resolvedSelection, spec: spec, runtime: runtime}, nil
}
