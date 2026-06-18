package commands

import (
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/workspace"
)

// RunStartCommand starts one run using the shared planning and execution pipeline.
type RunStartCommand struct {
	Runtime  model.RuntimeConfig
	Recovery workspace.AgentRecoveryConfig
}

// RunStartResult captures the run identifiers produced by a successful start command.
type RunStartResult struct {
	RunID        string
	ArtifactsDir string
	Status       string
}

// RunStartFromConfig translates the legacy core.Config shape into a typed run-start command.
func RunStartFromConfig(cfg core.Config) RunStartCommand {
	return RunStartCommand{
		Runtime:  runtimeConfigFromCore(cfg),
		Recovery: cfg.Recovery,
	}
}

// RuntimeConfig converts the command into the shared runtime configuration.
func (c RunStartCommand) RuntimeConfig() *model.RuntimeConfig {
	return cloneRuntimeConfig(c.Runtime)
}

// RecoveryConfig returns the resolved recovery configuration for this run.
func (c RunStartCommand) RecoveryConfig() workspace.AgentRecoveryConfig {
	return c.Recovery.ApplyDefaults()
}
