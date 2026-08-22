package core

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
)

func (h *BaseHandlers) decorateLoopRunOwners(ctx context.Context, response *contract.LoopRunsResponse) error {
	if response == nil {
		return nil
	}
	for index := range response.Runs {
		if err := h.decorateLoopRunOwner(ctx, &response.Runs[index]); err != nil {
			return err
		}
	}
	return nil
}

func (h *BaseHandlers) decorateLoopRunOwner(ctx context.Context, run *contract.LoopRunPayload) error {
	if run == nil {
		return nil
	}
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	owner, found := owners[run.ProfileID]
	if !found {
		return fmt.Errorf("api: loop run %q profile owner %q not found", run.ID, run.ProfileID)
	}
	if run.ProfileID == "" && (h == nil || h.Profiles == nil) {
		run.ProfileID = owner.ID
	}
	run.ProfileName, run.ProfileColor, run.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}
