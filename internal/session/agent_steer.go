package session

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
)

type AgentSteerer interface {
	Steer(ctx context.Context, proc *AgentProcess, turnID, text string) (acp.SteerResult, error)
}

var _ AgentSteerer = (*ACPDriverAdapter)(nil)

func (a *ACPDriverAdapter) Steer(
	ctx context.Context,
	proc *AgentProcess,
	turnID, text string,
) (acp.SteerResult, error) {
	native, err := a.nativeProcess(proc)
	if err != nil {
		return acp.SteerResult{Attempt: acp.SteerAttemptUnsupported}, err
	}
	return a.driver.Steer(ctx, native, turnID, text)
}
