package agent

// -----------------------------------------------------------------------------
// ActionsValidator
// -----------------------------------------------------------------------------

type ActionsValidator struct {
	actions []*ActionConfig
}

func NewActionsValidator(actions []*ActionConfig) *ActionsValidator {
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
