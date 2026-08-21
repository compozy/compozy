package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (h *BaseHandlers) decorateNetworkChannelOwner(
	ctx context.Context,
	entry *store.NetworkChannelEntry,
) error {
	if entry == nil {
		return nil
	}
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return fmt.Errorf("api: list network channel profile owners: %w", err)
	}
	owner, found := owners[strings.TrimSpace(entry.ProfileID)]
	if !found {
		return fmt.Errorf("api: network channel %q profile owner %q not found", entry.Channel, entry.ProfileID)
	}
	entry.ProfileID = owner.ID
	entry.ProfileName = owner.Name
	entry.ProfileColor = owner.Color
	entry.ProfileIcon = owner.Icon
	entry.ProfileEmoji = owner.Emoji
	entry.ProfileArchived = owner.Archived
	return nil
}
