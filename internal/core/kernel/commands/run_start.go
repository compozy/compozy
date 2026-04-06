package commands

import (
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
)

// RunStartCommand starts one run using the shared planning and execution pipeline.
type RunStartCommand struct {
	RuntimeFields
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
		RuntimeFields: runtimeFieldsFromConfig(cfg),
	}
}

// RuntimeConfig converts the command into the shared runtime configuration.
func (c RunStartCommand) RuntimeConfig() *model.RuntimeConfig {
	return c.RuntimeFields.RuntimeConfig()
}
