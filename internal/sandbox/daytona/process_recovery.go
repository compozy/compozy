package daytona

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/sandbox"
)

var _ sandbox.ProcessController = (*daytonaProvider)(nil)

type remoteProcessController interface {
	processExitVerified(context.Context, sandboxInfo) (bool, error)
	signalProcess(context.Context, sandboxInfo, sandbox.ProcessSignal) error
}

func (p *daytonaProvider) ProcessExitVerified(ctx context.Context, state sandbox.SessionState) (bool, error) {
	controller, info, err := p.recoveredProcess(state)
	if err != nil {
		return false, err
	}
	return controller.processExitVerified(ctx, info)
}

func (p *daytonaProvider) SignalProcess(
	ctx context.Context,
	state sandbox.SessionState,
	signal sandbox.ProcessSignal,
) error {
	controller, info, err := p.recoveredProcess(state)
	if err != nil {
		return err
	}
	return controller.signalProcess(ctx, info, signal)
}

func (p *daytonaProvider) recoveredProcess(state sandbox.SessionState) (remoteProcessController, sandboxInfo, error) {
	var info sandboxInfo
	stored, err := decodeProviderState(state.ProviderState)
	if err != nil {
		return nil, info, err
	}
	if state.Backend != sandbox.BackendDaytona || stored.SandboxID == "" || stored.SandboxID != state.InstanceID {
		return nil, info, errors.New("sandbox/daytona: recovered process sandbox identity is missing or mismatched")
	}
	if !validRecoveredProcessID(stored.LauncherProcessID) {
		return nil, info, errors.New("sandbox/daytona: recovered launcher process identity is missing or invalid")
	}
	if _, err := launcherPortForVersion(stored.LauncherSidecarVersion); err != nil {
		return nil, info, err
	}
	controller, ok := p.launcherTransport.(remoteProcessController)
	if !ok {
		return nil, info, errors.New("sandbox/daytona: launcher transport cannot recover process identity")
	}
	info = sandboxInfo{
		ID: stored.SandboxID, APIURL: stored.APIURL, SSHHost: stored.SSHHost,
		LauncherProcessID:      stored.LauncherProcessID,
		LauncherSidecarVersion: stored.LauncherSidecarVersion,
	}
	return controller, info, nil
}

func validRecoveredProcessID(id string) bool {
	if len(id) < 16 || len(id) > 128 {
		return false
	}
	for _, char := range id {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '-' || char == '_'
		if !valid {
			return false
		}
	}
	return true
}
