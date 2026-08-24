package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func (h *BaseHandlers) decorateLoopRunOwners(ctx context.Context, response *contract.LoopRunsResponse) error {
	if response == nil {
		return nil
	}
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	for index := range response.Runs {
		if err := decorateLoopRunOwnerWithOwners(
			&response.Runs[index],
			owners,
			h == nil || h.Profiles == nil,
		); err != nil {
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
	return decorateLoopRunOwnerWithOwners(run, owners, h == nil || h.Profiles == nil)
}

func decorateLoopRunOwnerWithOwners(
	run *contract.LoopRunPayload,
	owners map[string]profileOwnerIdentity,
	useDefaultID bool,
) error {
	if run == nil {
		return nil
	}
	owner, found := owners[strings.TrimSpace(run.ProfileID)]
	if !found {
		return fmt.Errorf("api: loop run %q profile owner %q not found", run.ID, run.ProfileID)
	}
	if strings.TrimSpace(run.ProfileID) == "" && useDefaultID {
		run.ProfileID = owner.ID
	}
	run.ProfileName, run.ProfileColor, run.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}
