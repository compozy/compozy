package workflow

import (
	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
)

// -----------------------------------------------------------------------------
// ComponentsValidator
// -----------------------------------------------------------------------------

// ComponentsValidator validates a list of components
type ComponentsValidator struct {
	components []common.Config
	cwd        *common.CWD
}

func NewComponentsValidator(components []common.Config, cwd *common.CWD) *ComponentsValidator {
	return &ComponentsValidator{
		components: components,
		cwd:        cwd,
	}
}

func (v *ComponentsValidator) Validate() error {
	for _, c := range v.components {
		if err := c.Validate(); err != nil {
			switch c.(type) {
			case *agent.AgentConfig:
				return NewAgentValidationError(err)
			case *tool.ToolConfig:
				return NewToolValidationError(err)
			case *task.TaskConfig:
				return NewTaskValidationError(err)
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// TriggerValidator
// -----------------------------------------------------------------------------

// TriggerValidator validates the trigger configuration
type TriggerValidator struct {
	config WorkflowConfig
}

func NewTriggerValidator(config WorkflowConfig) *TriggerValidator {
	return &TriggerValidator{config: config}
}

func (v *TriggerValidator) Validate() error {
	trigger := v.config.Trigger
	if err := trigger.Validate(); err != nil {
		return NewTriggerValidationError(err)
	}

	return nil
}
