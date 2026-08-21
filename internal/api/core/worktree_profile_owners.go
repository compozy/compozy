package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
)

func (h *BaseHandlers) enrichWorktreeListingOwners(
	ctx context.Context,
	listing *worktree.DetailedListing,
) error {
	if listing == nil {
		return nil
	}
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return fmt.Errorf("api: load worktree profile owners: %w", err)
	}
	for index := range listing.Worktrees {
		applyWorktreeOwner(&listing.Worktrees[index].Worktree, owners)
	}
	return nil
}

func (h *BaseHandlers) enrichWorktreeOwner(ctx context.Context, item *worktree.Worktree) error {
	if item == nil {
		return nil
	}
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return fmt.Errorf("api: load worktree profile owner: %w", err)
	}
	applyWorktreeOwner(item, owners)
	return nil
}

func applyWorktreeOwner(item *worktree.Worktree, owners map[string]profileOwnerIdentity) {
	if item == nil {
		return
	}
	profileID := strings.TrimSpace(item.ProfileID)
	owner, ok := owners[profileID]
	if !ok && profileID == "" {
		owner, ok = owners[store.DefaultProfileID]
	}
	if !ok {
		return
	}
	item.ProfileID = owner.ID
	item.ProfileName = owner.Name
	item.ProfileColor = owner.Color
	item.ProfileIcon = owner.Icon
	item.ProfileEmoji = owner.Emoji
	item.ProfileArchived = owner.Archived
}
