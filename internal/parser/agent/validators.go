package agent

// ActionsValidator validates agent actions
type ActionsValidator struct {
	actions []*AgentActionConfig
}

func NewActionsValidator(actions []*AgentActionConfig) *ActionsValidator {
	return &ActionsValidator{actions: actions}
}

func (v *ActionsValidator) Validate() error {
	if v.actions == nil {
		return nil
	}
	for _, action := range v.actions {
		if err := action.Validate(); err != nil {
			return err
		}
	}
	return nil
}
