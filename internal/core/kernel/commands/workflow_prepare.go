package commands

import (
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
)

// WorkflowPrepareCommand prepares workflow jobs without executing them.
type WorkflowPrepareCommand struct {
	RuntimeFields
}

// WorkflowPrepareResult captures the planned jobs and run artifact identifiers.
type WorkflowPrepareResult struct {
	Preparation  *core.Preparation
	RunID        string
	ArtifactsDir string
}

// WorkflowPrepareFromConfig translates the legacy core.Config shape into a typed prepare command.
func WorkflowPrepareFromConfig(cfg core.Config) WorkflowPrepareCommand {
	return WorkflowPrepareCommand{
		RuntimeFields: runtimeFieldsFromConfig(cfg),
	}
}

// RuntimeConfig converts the command into the shared runtime configuration.
func (c WorkflowPrepareCommand) RuntimeConfig() *model.RuntimeConfig {
	return c.RuntimeFields.RuntimeConfig()
}
