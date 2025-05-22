package wfuc

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
)

func FindAndValidateInput(wfs []*workflow.Config, wfID string, input common.Input) error {
	var cfg *workflow.Config
	for _, wf := range wfs {
		if wf.ID == wfID {
			cfg = wf
			break
		}
	}
	if cfg == nil {
		return fmt.Errorf("workflow not found")
	}
	if err := cfg.ValidateParams(input); err != nil {
		return fmt.Errorf("invalid workflow input: %w", err)
	}
	return nil
}
