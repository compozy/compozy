package session

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/providerexec"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/toolmeta"
	toolspkg "github.com/compozy/compozy/internal/tools"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (m *Manager) prepareSessionLaunch(
	ctx context.Context,
	spec *sessionStartSpec,
	session *Session,
	runtime *sessionStartRuntime,
	run *sessionStartRun,
) (acp.StartOpts, error) {
	startOpts, err := m.sessionStartOpts(spec, session, runtime.agent, runtime.mcpServers)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session runtime adapter failed", err)
	}
	startOpts.StartupManifest = runtime.startupManifest
	startOpts, err = m.prepareProviderForStart(ctx, session, runtime.agent, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session provider startup failed", err)
	}
	preHookCommand := strings.TrimSpace(startOpts.Command)
	preHookOpts := startOpts
	preHookOpts.MCPServers = slices.Clone(startOpts.MCPServers)
	startOpts, err = m.dispatchAgentPreStart(ctx, session, runtime.agent, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session pre-start hook failed", err)
	}
	if spec.sessionType == SessionTypeTriage && startOpts.Command != preHookCommand {
		return acp.StartOpts{}, acp.ErrTriageUnsupported
	}
	if err := validateInstanceLaunch(spec, preHookOpts, startOpts); err != nil {
		return acp.StartOpts{}, err
	}
	launchAgent := runtime.agent
	if strings.TrimSpace(startOpts.Command) != preHookCommand {
		launchAgent.CommandIsBuiltin = false
	}
	startOpts.Cwd, err = normalizeSessionLaunchCWD(spec, startOpts.Cwd)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session pre-start hook cwd is invalid", err)
	}
	startOpts, err = resolveProviderNativeCLI(launchAgent, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session native provider startup failed", err)
	}
	startOpts, err = m.prepareRuntimeMCPStartup(ctx, spec, runtime, startOpts)
	if err != nil {
		return acp.StartOpts{}, startupFailure("session runtime MCP startup failed", err)
	}
	startOpts = m.finalizeProviderProbeEnvForStart(session, launchAgent, startOpts)
	startOpts = m.preparePiBridgeStartup(spec, session, runtime, startOpts)
	startOpts = m.prepareHostedMCPStartup(session, runtime, startOpts)
	if err := m.prepareActivationCatalog(ctx, session, runtime); err != nil {
		return acp.StartOpts{}, err
	}
	m.constrainTriageLaunch(spec, runtime, &startOpts)
	startOpts.MCPServers = m.hostedMCPBudgetedServers(runtime.agent.Harness, startOpts)

	if spec.resumeReplay || spec.resumeReplayBlock != "" {
		if err := m.prepareSessionReplay(ctx, spec, session, runtime); err != nil {
			return acp.StartOpts{}, err
		}
	}
	if err := m.commitSessionLaunchIdentity(ctx, spec, session, run, startOpts.Permissions); err != nil {
		return acp.StartOpts{}, err
	}
	return startOpts, nil
}

func (m *Manager) commitSessionLaunchIdentity(
	ctx context.Context,
	spec *sessionStartSpec,
	session *Session,
	run *sessionStartRun,
	permissions compozyconfig.PermissionMode,
) error {
	releaseCommit, err := acquireSessionStartLaunchCommit(ctx, run)
	if err != nil {
		return fmt.Errorf("session: acquire launch commit for %q: %w", spec.sessionID, err)
	}
	defer releaseCommit()
	if err := context.Cause(ctx); err != nil {
		return err
	}
	session.SetEffectivePermissions(string(permissions))
	if err := finalizeStartCreationIdentityIfEnabled(spec, session); err != nil {
		return startupFailure("session creation identity changed", err)
	}
	if err := m.revalidateSessionWorktree(ctx, spec); err != nil {
		return startupFailure("session worktree binding changed", err)
	}
	if err := m.persistSessionLifecycleState(ctx, session, true); err != nil {
		m.sessionLogger(session).Warn("session.start.meta_write_failed", "phase", spec.startAction, "error", err)
		return fmt.Errorf(
			"session: persist %s launch identity for %q: %w",
			spec.startAction,
			spec.sessionID,
			err,
		)
	}
	return nil
}

func (m *Manager) prepareHostedMCPStartup(
	session *Session,
	runtime *sessionStartRuntime,
	startOpts acp.StartOpts,
) acp.StartOpts {
	const failedOutcome = string(BadgeFailed)
	// Only the Claude bridge is verified to consume list_changed. Other hosts
	// reload at the turn boundary, using the effective post-hook command.
	reloadTools := !providerexec.IsClaudeAgentBridge(startOpts.Command)
	session.mu.Lock()
	session.hostedMCPRequiresReload = m.hostedMCP != nil && len(runtime.mcpServers) > 0 && reloadTools
	session.hostedMCPDigest = ""
	session.directMCPDigest = ""
	if m.hostedMCP == nil {
		session.directMCPDigest = directMCPDigest(runtime.mcpServers)
	}
	session.mu.Unlock()
	if runtime.agent.Harness != compozyconfig.ProviderHarnessPiACP && m.hostedMCP != nil &&
		len(startOpts.MCPServers) > 0 &&
		(providerexec.IsClaudeAgentBridge(startOpts.Command) || providerexec.IsCodexAgentBridge(startOpts.Command)) {
		prompt, manifest := startOpts.SystemPrompt, startOpts.StartupManifest
		finalize := startOpts.FinalizeStartup
		startOpts.FinalizeStartup = func(ctx context.Context) (string, acp.StartupManifest, error) {
			if finalize != nil {
				var err error
				prompt, manifest, err = finalize(ctx)
				if err != nil {
					return prompt, manifest, err
				}
			}
			started := time.Now()
			var err error
			if reloadTools {
				err = m.captureHostedMCPDigest(ctx, session)
			} else {
				err = m.hostedMCP.WaitForBind(ctx, session.ID)
			}
			outcome := "succeeded"
			if err != nil {
				outcome = failedOutcome
				m.hostedMCP.CancelLaunch(session.ID)
			}
			m.sessionLogger(session).
				Info("acp.start.stage", "stage", "mcp_bind", "outcome", outcome, "duration_ms", time.Since(started).Milliseconds())
			return prompt, manifest, err
		}
	}

	return startOpts
}

func (m *Manager) hostedMCPBudgetedServers(
	harness compozyconfig.ProviderHarness, opts acp.StartOpts,
) []compozyconfig.MCPServer {
	if m.hostedMCPToolListThreshold <= 0 || m.hostedMCP == nil ||
		harness == compozyconfig.ProviderHarnessPiACP || !providerexec.IsClaudeAgentBridge(opts.Command) {
		return opts.MCPServers
	}
	index := slices.IndexFunc(opts.MCPServers, func(server compozyconfig.MCPServer) bool {
		return server.Name == toolmeta.HostedServerName && slices.Contains(server.Args, toolmeta.HostedBindNonceFlag)
	})
	if index < 0 {
		return opts.MCPServers
	}
	servers := slices.Clone(opts.MCPServers)
	servers[index].Args = append(slices.Clone(servers[index].Args),
		toolmeta.HostedToolListThresholdFlag, strconv.Itoa(m.hostedMCPToolListThreshold))
	return servers
}

func (m *Manager) prepareSessionReplay(
	ctx context.Context,
	spec *sessionStartSpec,
	session *Session,
	runtime *sessionStartRuntime,
) error {
	tools, err := concreteDelegationTools(runtime.agent, m.toolsetCatalog, m.toolUniverse)
	if err != nil {
		return startupFailure("session replay tool surface resolution failed", err)
	}
	historyAvailable := runtime.agent.SessionMCP && m.hostedMCP != nil &&
		slices.Contains(tools, toolspkg.ToolIDSessionHistory.String())
	meta := session.Meta()
	policy := store.NormalizeSessionLineage(meta.ID, meta.Lineage).PermissionPolicy
	if len(policy.Tools) > 0 {
		historyAvailable = historyAvailable && slices.Contains(policy.Tools, toolspkg.ToolIDSessionHistory.String())
	}
	options := rebuildReplayContext{
		workspace: &spec.workspace, historyAvailable: historyAvailable, reason: spec.resumeReplayReason,
	}
	if spec.resumeReplayBlock != "" {
		spec.resumeReplayBlock, spec.resumeReplayMessageCount, err = m.reboundResumeReplay(
			session, spec.resumeReplayBlock, options,
		)
	} else {
		spec.resumeReplayBlock, spec.resumeReplayMessageCount, err = m.buildResumeReplay(ctx, session, options)
	}
	if err != nil {
		return startupFailure("session replay preparation failed", err)
	}
	return nil
}

func normalizeSessionLaunchCWD(spec *sessionStartSpec, requested string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("session: start spec is required")
	}
	cwd, err := resolveContainedDirectory(spec.executionRoot(), requested)
	if err != nil {
		return "", fmt.Errorf("%w: pre-start hook cwd escapes execution root: %w", ErrValidation, err)
	}
	return cwd, nil
}

func (m *Manager) constrainTriageLaunch(
	spec *sessionStartSpec,
	runtime *sessionStartRuntime,
	startOpts *acp.StartOpts,
) {
	if spec.sessionType == SessionTypeTriage {
		startOpts.SetExecutionPolicy(acp.ExecutionPolicyTriage)
		startOpts.Permissions = compozyconfig.PermissionModeApproveReads
		startOpts.MCPServers = slices.Clone(runtime.mcpServers)
		startOpts.ActivateMCPServers = m.sessionMCPServerActivator(spec.sessionID, runtime.mcpServers)
		startOpts.CheckInTools = len(runtime.mcpServers) > 0
	}
}
