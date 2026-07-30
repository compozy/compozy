package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/toolruntime"
)

// ACPDriverAdapter adapts the concrete ACP driver to the session-local interface.
type ACPDriverAdapter struct {
	driver *acp.Driver
}

var _ AgentDriver = (*ACPDriverAdapter)(nil)
var _ ScopedInterrupter = (*ACPDriverAdapter)(nil)
var _ RuntimeConfigurator = (*ACPDriverAdapter)(nil)

// NewACPDriverAdapter wraps the provided ACP driver for use by the session manager.
func NewACPDriverAdapter(driver *acp.Driver) *ACPDriverAdapter {
	return &ACPDriverAdapter{driver: driver}
}

// Start launches a new ACP-backed runtime process.
func (a *ACPDriverAdapter) Start(ctx context.Context, opts acp.StartOpts) (*AgentProcess, error) {
	proc, err := a.driver.Start(ctx, opts)
	if err != nil {
		return nil, err
	}
	return wrapACPProcess(proc), nil
}

// Prompt streams prompt events from the wrapped ACP runtime.
func (a *ACPDriverAdapter) Prompt(
	ctx context.Context,
	proc *AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	native, err := a.nativeProcess(proc)
	if err != nil {
		return nil, err
	}
	return a.driver.Prompt(ctx, native, req)
}

// Cancel lets higher layers stop prompt work without bypassing ACP-owned lifecycle handling.
func (a *ACPDriverAdapter) Cancel(ctx context.Context, proc *AgentProcess) error {
	native, err := a.nativeProcess(proc)
	if err != nil {
		return err
	}
	return a.driver.Cancel(ctx, native)
}

// ConfigureRuntime applies mutable ACP settings to the active process.
func (a *ACPDriverAdapter) ConfigureRuntime(
	ctx context.Context,
	proc *AgentProcess,
	config acp.RuntimeConfig,
) error {
	native, err := a.nativeProcess(proc)
	if err != nil {
		return err
	}
	return a.driver.ConfigureRuntime(ctx, native, config)
}

// Interrupt signals only registered tool processes scoped to the session turn.
func (a *ACPDriverAdapter) Interrupt(
	ctx context.Context,
	sessionID string,
	turnID string,
) (toolruntime.InterruptReport, error) {
	return a.driver.Interrupt(ctx, toolruntime.InterruptScope{
		SessionID: sessionID,
		TurnID:    turnID,
		Reason:    "prompt canceled",
	})
}

// Stop stops the wrapped ACP runtime process.
func (a *ACPDriverAdapter) Stop(ctx context.Context, proc *AgentProcess) error {
	native, err := a.nativeProcess(proc)
	if err != nil {
		return err
	}
	return a.driver.Stop(ctx, native)
}

func (a *ACPDriverAdapter) nativeProcess(proc *AgentProcess) (*acp.AgentProcess, error) {
	if proc == nil {
		return nil, errors.New("session: agent process is required")
	}

	native, ok := proc.native.(*acp.AgentProcess)
	if !ok || native == nil {
		return nil, errors.New("session: unsupported agent process implementation")
	}
	return native, nil
}
