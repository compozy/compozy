package acp

import (
	"context"
	"errors"
	"fmt"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/sandbox"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/toolruntime"
)

func (d *Driver) launchAgentProcess(ctx context.Context, normalized StartOpts) (*AgentProcess, error) {
	normalized, command, args, err := d.resolvedAgentLaunch(ctx, normalized)
	if err != nil {
		return nil, err
	}

	policy, err := newPermissionPolicy(normalized.Permissions, normalized.Cwd, normalized.AdditionalDirs...)
	if err != nil {
		return nil, err
	}

	launcher := d.launcherForStart(normalized)
	identity := startOptsPreparedLaunchIdentity(normalized)

	handle, err := launcher.Launch(ctx, sandbox.LaunchSpec{
		Command:            normalized.Command,
		ResolvedExecutable: identity.spec.ResolvedExecutable,
		Args:               append([]string(nil), identity.spec.Args...),
		Cwd:                normalized.Cwd,
		AdditionalDirs:     append([]string(nil), normalized.AdditionalDirs...),
		Env:                append([]string(nil), normalized.Env...),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"acp: start agent %q subprocess %q in %q: %w",
			normalized.AgentName,
			normalized.Command,
			normalized.Cwd,
			err,
		)
	}
	procCtx, cancelProcess := context.WithCancel(context.WithoutCancel(ctx))

	toolHost := normalized.ToolHost
	if toolHost == nil {
		toolHost = d.toolHost
	}
	if toolHost == nil {
		toolHost = newLocalToolHostFromPolicy(
			procCtx,
			normalized.Cwd,
			policy,
			d.logger,
			WithLocalProcessRegistry(d.processRegistry),
			WithLocalTerminalManager(d.terminals, terminalScope{
				workspaceID: normalized.WorkspaceID,
				profileID:   normalized.ProfileID,
				sessionID:   normalized.CompozySessionID,
				generation:  normalized.RuntimeGeneration,
				actorID:     normalized.AgentName,
			}),
		)
	}

	process := d.newAgentProcess(procCtx, cancelProcess, normalized, command, args, handle, toolHost, policy)
	if localHost, ok := toolHost.(*localToolHost); ok {
		process.terminals = localHost.terminals
	}
	if localHandle, ok := handle.(*localProcessHandle); ok {
		process.managed = localHandle.process
	}
	process.conn = acpsdk.NewConnectionWithOptions(
		process.handleInbound,
		handle.Stdin(),
		handle.Stdout(),
		acpsdk.ConnectionOptions{
			NotificationQueueOverflow: acpsdk.NotificationQueueOverflowBlock,
		},
	)
	process.conn.SetLogger(d.logger)

	if err := d.registerAgentProcess(ctx, process); err != nil {
		cancelProcess()
		stopCtx, cancelStop := context.WithTimeout(context.Background(), d.stopTimeout)
		defer cancelStop()
		if stopErr := handle.Stop(stopCtx); stopErr != nil {
			return nil, errors.Join(err, fmt.Errorf("acp: cleanup unregistered agent process: %w", stopErr))
		}
		return nil, err
	}

	go process.waitForExit(ctx, d.processRecordTimeout)

	return process, nil
}

func (d *Driver) newAgentProcess(
	procCtx context.Context,
	cancelProcess context.CancelFunc,
	normalized StartOpts,
	command string,
	args []string,
	handle sandbox.Handle,
	toolHost ToolHost,
	policy permissionPolicy,
) *AgentProcess {
	return &AgentProcess{
		PID:           handle.PID(),
		AgentName:     normalized.AgentName,
		Command:       command,
		Args:          append([]string(nil), args...),
		Cwd:           normalized.Cwd,
		StartedAt:     timeNowUTC(),
		handle:        handle,
		toolHost:      toolHost,
		toolGateway:   normalized.ToolGateway,
		processCtx:    procCtx,
		cancelProcess: cancelProcess,
		permissions:   policy,
		terminalCore:  d.terminals,
		terminalScope: terminalScope{
			workspaceID: normalized.WorkspaceID,
			profileID:   normalized.ProfileID,
			sessionID:   normalized.CompozySessionID,
			generation:  normalized.RuntimeGeneration,
			actorID:     normalized.AgentName,
		},
		done:                 make(chan struct{}),
		pendingPermissions:   make(map[string]*pendingPermission),
		permissionTimeout:    d.permissionWait,
		systemPrompt:         normalized.SystemPrompt,
		systemPromptDelivery: normalized.SystemPromptDelivery,
		promptCacheControl:   promptCacheControlForStartOpts(normalized),
	}
}

func (d *Driver) registerAgentProcess(ctx context.Context, process *AgentProcess) error {
	if d == nil || process == nil {
		return nil
	}
	registry := d.processRegistry
	if registry == nil {
		if provider, ok := process.toolHost.(processRegistryProvider); ok {
			registry = provider.ProcessRegistry()
		}
	}
	process.processRegistry = registry
	if registry == nil || process.PID <= 0 {
		return nil
	}
	recordCtx, cancelRecord := processRecordContext(ctx, d.processRecordTimeout)
	defer cancelRecord()
	handle, err := registry.Register(recordCtx, toolruntime.RegisterConfig{
		Source: toolruntime.ProcessSourceACPAgent,
		Owner: toolruntime.ProcessOwner{
			SessionID: process.SessionID,
		},
		PID:            process.PID,
		ProcessGroupID: process.PID,
		Command:        process.Command,
		Args:           process.Args,
		Cwd:            process.Cwd,
		Interrupt: func(interruptCtx context.Context, _ toolruntime.ProcessRecord) error {
			return d.Stop(interruptCtx, process)
		},
	})
	if err != nil {
		return fmt.Errorf("acp: register agent process: %w", err)
	}
	process.processRecord = handle
	return nil
}

type processRegistryProvider interface {
	ProcessRegistry() *toolruntime.Registry
}

func (d *Driver) initializeConnection(ctx context.Context, process *AgentProcess, agentName string) error {
	initRequest := acpsdk.InitializeRequest{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		ClientCapabilities: acpsdk.ClientCapabilities{
			Fs: acpsdk.FileSystemCapabilities{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
		ClientInfo: &acpsdk.Implementation{
			Name:    defaultClientName,
			Version: defaultClientVersion,
		},
	}
	initializeResponse, err := acpsdk.SendRequest[acpsdk.InitializeResponse](
		process.conn,
		ctx,
		acpsdk.AgentMethodInitialize,
		initRequest,
	)
	if err != nil {
		return WrapFailure(
			store.FailureHandshake,
			"ACP initialize handshake failed",
			fmt.Errorf("acp: initialize session for %q: %w", agentName, err),
		)
	}

	process.setCaps(captureCaps(Caps{
		SupportsLoadSession:   initializeResponse.AgentCapabilities.LoadSession,
		SupportsCloseSession:  initializeResponse.AgentCapabilities.SessionCapabilities.Close != nil,
		PromptImage:           initializeResponse.AgentCapabilities.PromptCapabilities.Image,
		PromptAudio:           initializeResponse.AgentCapabilities.PromptCapabilities.Audio,
		PromptEmbeddedContext: initializeResponse.AgentCapabilities.PromptCapabilities.EmbeddedContext,
	}, nil, nil))
	return nil
}
