package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

// memoryControllerCallOptions is the invocation contract for the in-process
// write-controller tiebreaker.
type memoryControllerCallOptions struct {
	Timeout       time.Duration
	TopK          int
	PromptVersion string
	MaxTokensOut  int
	Config        aghconfig.Config
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
	resolved, effectiveConfig, err := r.resolveEffective(ctx, workspaceID, aghconfig.RoleMemoryController)
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
	runtime, err := effectiveConfig.ResolveAgent(aghconfig.AgentDef{
		Name:            memoryControllerTransientAgentName,
		Provider:        resolved.Provider,
		Model:           resolved.Model,
		ReasoningEffort: resolved.ReasoningEffort,
		Prompt:          "AGH memory controller transient runtime.",
	})
	if err != nil {
		return memoryControllerCallOptions{}, fmt.Errorf("daemon: resolve memory controller runtime: %w", err)
	}
	resolved.Provider = runtime.Provider
	resolved.Model = runtime.Model
	resolved.ReasoningEffort = runtime.ReasoningEffort
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
