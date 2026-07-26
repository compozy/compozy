package daytona

import (
	"context"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/sandbox"
)

func (p *daytonaProvider) findByLabels(
	ctx context.Context,
	client sandboxClient,
	labels map[string]string,
) (daytonaSandbox, error) {
	var instance daytonaSandbox
	err := p.withSDKTimeout(ctx, func(ctx context.Context) error {
		var findErr error
		instance, findErr = client.FindOne(ctx, labels)
		return findErr
	})
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func (p *daytonaProvider) createSandbox(
	ctx context.Context,
	client sandboxClient,
	req sandbox.PrepareRequest,
	labels map[string]string,
) (daytonaSandbox, error) {
	daytona := req.Sandbox.Daytona
	createReq := createSandboxRequest{
		Name:               "agh-" + req.SandboxID,
		Labels:             labels,
		EnvVars:            remoteEnvMap(req.AgentEnv, req.Sandbox.Env),
		Public:             req.Sandbox.Network.AllowPublicIngress,
		AutoStopMinutes:    parseDurationMinutes(daytona.AutoStop),
		AutoArchiveMinutes: parseDurationMinutes(daytona.AutoArchive),
		Timeout:            p.createTimeout,
	}
	switch daytona.StartupSource {
	case sandbox.DaytonaStartupSourceSnapshot:
		createReq.Snapshot = daytona.StartupRef
	case sandbox.DaytonaStartupSourceImage:
		createReq.Image = daytona.StartupRef
	default:
		return nil, fmt.Errorf("sandbox/daytona: unsupported startup source %q", daytona.StartupSource)
	}

	var created daytonaSandbox
	err := p.withSDKTimeout(ctx, func(ctx context.Context) error {
		var createErr error
		created, createErr = client.Create(ctx, createReq)
		return createErr
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (p *daytonaProvider) startSandbox(ctx context.Context, instance daytonaSandbox) (daytonaSandbox, error) {
	err := p.withSDKTimeout(ctx, func(ctx context.Context) error {
		return instance.Start(ctx)
	})
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func (p *daytonaProvider) runtimeRoot(ctx context.Context, instance daytonaSandbox, configured string) string {
	if strings.TrimSpace(configured) != "" {
		return strings.TrimSpace(configured)
	}
	var workingDir string
	err := p.withSDKTimeout(ctx, func(ctx context.Context) error {
		var workingErr error
		workingDir, workingErr = instance.WorkingDir(ctx)
		return workingErr
	})
	if err != nil {
		p.logger.Warn("sandbox/daytona: get working dir failed; using default runtime root", "error", err)
		return defaultRuntimeRoot
	}
	if strings.TrimSpace(workingDir) == "" {
		return defaultRuntimeRoot
	}
	return strings.TrimSpace(workingDir)
}
