package project

import (
	"errors"
	"fmt"
)

// WorkflowsValidator validates the workflows configuration
type WorkflowsValidator struct {
	workflows []*WorkflowSourceConfig
}

func NewWorkflowsValidator(workflows []*WorkflowSourceConfig) *WorkflowsValidator {
	return &WorkflowsValidator{workflows: workflows}
}

func (v *WorkflowsValidator) Validate() error {
	if len(v.workflows) == 0 {
		return errors.New("no workflows defined in project")
	}
	for _, wf := range v.workflows {
		if wf.Source == "" {
			return fmt.Errorf("workflow %s source is empty", wf.Source)
		}
	}
	return nil
}
