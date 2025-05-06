package workflow

import (
	"github.com/compozy/compozy/internal/parser/agent"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/task"
	"github.com/compozy/compozy/internal/parser/tool"
	"github.com/compozy/compozy/internal/parser/trigger"
)

// CWDValidator validates the current working directory
type CWDValidator struct {
	cwd *common.CWD
}

func NewCWDValidator(cwd *common.CWD) *CWDValidator {
	return &CWDValidator{cwd: cwd}
}

func (v *CWDValidator) Validate() error {
	if v.cwd == nil || v.cwd.Get() == "" {
		return NewMissingPathError()
	}
	return nil
}

// ComponentsValidator validates a list of components
type ComponentsValidator struct {
	components []common.ComponentConfig
	cwd        *common.CWD
}

func NewComponentsValidator(components []common.ComponentConfig, cwd *common.CWD) *ComponentsValidator {
	return &ComponentsValidator{
		components: components,
		cwd:        cwd,
	}
}

func (v *ComponentsValidator) Validate() error {
	for _, c := range v.components {
		if !TestMode {
			c.SetCWD(v.cwd.Get())
		}
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

// TriggerValidator validates the trigger configuration
type TriggerValidator struct {
	trigger trigger.TriggerConfig
}

func NewTriggerValidator(trigger trigger.TriggerConfig) *TriggerValidator {
	return &TriggerValidator{trigger: trigger}
}

func (v *TriggerValidator) Validate() error {
	if err := v.trigger.Validate(); err != nil {
		return NewTriggerValidationError(err)
	}
	return nil
}
