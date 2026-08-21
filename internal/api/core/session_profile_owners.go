package core

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
)

func (h *BaseHandlers) decorateSessionOwners(
	ctx context.Context,
	payloads []contract.SessionPayload,
) ([]contract.SessionPayload, error) {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return nil, err
	}
	for index := range payloads {
		owner, found := owners[payloads[index].ProfileID]
		if !found {
			return nil, fmt.Errorf(
				"api: session %q profile owner %q not found",
				payloads[index].ID,
				payloads[index].ProfileID,
			)
		}
		if payloads[index].ProfileID == "" && (h == nil || h.Profiles == nil) {
			payloads[index].ProfileID = owner.ID
		}
		payloads[index].ProfileName = owner.Name
		payloads[index].ProfileColor = owner.Color
		payloads[index].ProfileIcon = owner.Icon
		payloads[index].ProfileEmoji = owner.Emoji
		payloads[index].ProfileArchived = owner.Archived
	}
	return payloads, nil
}
