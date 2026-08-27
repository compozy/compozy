package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// memoryControllerCallOptions is the invocation contract for the in-process
// write-controller tiebreaker.
type memoryControllerCallOptions struct {
	Timeout       time.Duration
	TopK          int
	PromptVersion string
	MaxTokensOut  int
	Config        compozyconfig.Config
	resolvedRole  ResolvedRole
}

const memoryControllerTransientAgentName = "memory-controller"

func (r *roleResolver) resolveMemoryControllerCallOptions(
	ctx context.Context,
	workspaceID string,
) (memoryControllerCallOptions, error) {
	if r == nil || r.config == nil {
		return memoryControllerCallOptions{}, errors.New("daemon: memory controller role resolver is required")
	}
	resolved, effectiveConfig, err := r.resolveEffective(ctx, workspaceID, compozyconfig.RoleMemoryController)
	if err != nil {
		return memoryControllerCallOptions{}, fmt.Errorf("daemon: resolve memory controller role: %w", err)
	}
	effective := effectiveConfig.Roles.MemoryController
	if !resolved.Enabled {
		return memoryControllerCallOptions{
			Timeout:       effective.Timeout,
			TopK:          effective.TopK,
			PromptVersion: effective.PromptVersion,
			MaxTokensOut:  effective.MaxTokensOut,
			Config:        *effectiveConfig,
			resolvedRole:  resolved,
		}, nil
	}
	runtime, err := effectiveConfig.ResolveAgent(compozyconfig.AgentDef{
		Name:            memoryControllerTransientAgentName,
		Provider:        resolved.Provider,
		Model:           resolved.Model,
		ReasoningEffort: resolved.ReasoningEffort,
		Speed:           resolved.Speed,
		ACPOptions:      compozyconfig.CloneACPOptionSelections(resolved.ACPOptions),
		Prompt:          "Compozy memory controller transient runtime.",
	})
	if err != nil {
		return memoryControllerCallOptions{}, fmt.Errorf("daemon: resolve memory controller runtime: %w", err)
	}
	resolved.Provider = runtime.Provider
	resolved.Model = runtime.Model
	resolved.ReasoningEffort = runtime.ReasoningEffort
	resolved.Speed = runtime.Speed
	resolved.ACPOptions = compozyconfig.CloneACPOptionSelections(runtime.ACPOptions)
	resolved.eventWriter = r.events
	return memoryControllerCallOptions{
		Timeout:       effective.Timeout,
		TopK:          effective.TopK,
		PromptVersion: effective.PromptVersion,
		MaxTokensOut:  effective.MaxTokensOut,
		Config:        *effectiveConfig,
		resolvedRole:  resolved,
	}, nil
}
