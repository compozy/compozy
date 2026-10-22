package core

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
)

func (h *BaseHandlers) decorateSessionOwners(
	ctx context.Context,
	payloads []contract.SessionPayload,
) ([]contract.SessionPayload, error) {
	if h == nil {
		return SessionPayloadsWithProfileOwners(ctx, payloads, nil)
	}
	return SessionPayloadsWithProfileOwners(ctx, payloads, h.Profiles)
}

type ProfileOwnerReader interface {
	List(context.Context) ([]profilepkg.WithCounts, error)
}

func SessionPayloadsWithProfileOwners(
	ctx context.Context,
	payloads []contract.SessionPayload,
	profiles ProfileOwnerReader,
) ([]contract.SessionPayload, error) {
	owners, err := profileOwnerIdentities(ctx, profiles)
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
		if payloads[index].ProfileID == "" && profiles == nil {
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
