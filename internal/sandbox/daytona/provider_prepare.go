package daytona

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
)

func (p *daytonaProvider) foundSandboxState(
	ctx context.Context,
	req sandbox.FindSandboxRequest,
	config findSandboxConfig,
	found daytonaSandbox,
	sandboxID string,
) (sandbox.SessionState, error) {
	runtimeRoot := strings.TrimSpace(firstNonEmpty(config.existing.RuntimeRootDir, req.Sandbox.RuntimeRootDir))
	if runtimeRoot == "" {
		runtimeRoot = p.runtimeRoot(ctx, found, "")
	}
	localRoot := strings.TrimSpace(firstNonEmpty(req.LocalRootDir, config.existing.LocalRootDir))
	localAdditional := cloneStrings(req.LocalAdditionalDirs)
	if len(localAdditional) == 0 {
		localAdditional = cloneStrings(config.existing.LocalAdditionalDirs)
	}
	runtimeAdditional := cloneStrings(config.existing.RuntimeAdditionalDirs)
	if len(runtimeAdditional) == 0 {
		runtimeAdditional = remoteAdditionalDirs(runtimeRoot, localAdditional)
	}

	providerState := providerState{
		Version:               providerStateVersion,
		SandboxID:             found.ID(),
		SandboxName:           found.Name(),
		APIURL:                config.apiURL,
		SSHHost:               p.sshHost,
		LocalRootDir:          localRoot,
		LocalAdditionalDirs:   cloneStrings(localAdditional),
		RuntimeRootDir:        runtimeRoot,
		RuntimeAdditionalDirs: cloneStrings(runtimeAdditional),
		Persistence:           req.Sandbox.Persistence,
		StartupSource:         config.startupSource,
		StartupRef:            config.startupRef,
		PreparedAt:            p.now().UTC(),
	}
	rawState, err := encodeProviderState(providerState)
	if err != nil {
		return sandbox.SessionState{}, err
	}

	return sandbox.SessionState{
		SandboxID:             sandboxID,
		Backend:               sandbox.BackendDaytona,
		Profile:               req.Sandbox.Profile,
		State:                 "found",
		InstanceID:            found.ID(),
		RuntimeRootDir:        runtimeRoot,
		RuntimeAdditionalDirs: cloneStrings(runtimeAdditional),
		ProviderState:         rawState,
		PreparedAt:            providerState.PreparedAt,
	}, nil
}

func (p *daytonaProvider) buildPrepared(
	req sandbox.PrepareRequest,
	instance daytonaSandbox,
	info sandboxInfo,
	access sshAccess,
	runtimeRoot string,
	runtimeAdditional []string,
	remoteEnv map[string]string,
) (sandbox.Prepared, error) {
	daytona := req.Sandbox.Daytona
	providerState := providerState{
		Version:               providerStateVersion,
		SandboxID:             instance.ID(),
		SandboxName:           instance.Name(),
		APIURL:                info.APIURL,
		SSHHost:               p.sshHost,
		LocalRootDir:          req.LocalRootDir,
		LocalAdditionalDirs:   cloneStrings(req.LocalAdditionalDirs),
		RuntimeRootDir:        runtimeRoot,
		RuntimeAdditionalDirs: cloneStrings(runtimeAdditional),
		Persistence:           req.Sandbox.Persistence,
		StartupSource:         daytona.StartupSource,
		StartupRef:            daytona.StartupRef,
		SSHAccessExpiresAt:    &access.ExpiresAt,
		PreparedAt:            p.now().UTC(),
	}
	rawState, err := encodeProviderState(providerState)
	if err != nil {
		return sandbox.Prepared{}, err
	}

	permission := config.PermissionMode(strings.TrimSpace(req.Permissions))
	toolHost, err := newDaytonaToolHost(
		instance,
		p.shellTransport,
		info,
		runtimeRoot,
		permission,
		withDaytonaToolHostProcessRegistry(p.processRegistry),
	)
	if err != nil {
		return sandbox.Prepared{}, err
	}
	state := sandbox.SessionState{
		SandboxID:             req.SandboxID,
		Backend:               sandbox.BackendDaytona,
		Profile:               req.Sandbox.Profile,
		State:                 "ready",
		InstanceID:            instance.ID(),
		RuntimeRootDir:        runtimeRoot,
		RuntimeAdditionalDirs: cloneStrings(runtimeAdditional),
		ProviderState:         rawState,
		SSHAccessExpiresAt:    &access.ExpiresAt,
		PreparedAt:            providerState.PreparedAt,
	}
	return sandbox.Prepared{
		State:                 state,
		RuntimeRootDir:        runtimeRoot,
		RuntimeAdditionalDirs: cloneStrings(runtimeAdditional),
		Launcher:              &daytonaLauncher{transport: p.launcherTransport, sandbox: info},
		Launch: sandbox.LaunchSpec{
			Command:        req.AgentCommand,
			Cwd:            runtimeRoot,
			AdditionalDirs: cloneStrings(runtimeAdditional),
			Env:            remoteEnvList(remoteEnv),
		},
		ToolHost: toolHost,
	}, nil
}

func (p *daytonaProvider) prepareSandbox(
	ctx context.Context,
	client sandboxClient,
	req sandbox.PrepareRequest,
	existingState providerState,
) (daytonaSandbox, error) {
	sandboxID := strings.TrimSpace(firstNonEmpty(req.InstanceID, existingState.SandboxID))
	if sandboxID != "" {
		return p.getAndStart(ctx, client, sandboxID)
	}

	labels := aghLabels(req)
	if found, err := p.findByLabels(ctx, client, labels); err == nil {
		return p.startSandbox(ctx, found)
	} else if !errors.Is(err, errSandboxNotFound) {
		return nil, err
	}

	return p.createSandbox(ctx, client, req, labels)
}

func (p *daytonaProvider) getAndStart(
	ctx context.Context,
	client sandboxClient,
	sandboxID string,
) (daytonaSandbox, error) {
	var instance daytonaSandbox
	err := p.withSDKTimeout(ctx, func(ctx context.Context) error {
		var getErr error
		instance, getErr = client.Get(ctx, sandboxID)
		return getErr
	})
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: reattach sandbox %q: %w", sandboxID, err)
	}
	return p.startSandbox(ctx, instance)
}
