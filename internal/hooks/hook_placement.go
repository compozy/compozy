package hooks

import (
	"slices"
	"strings"
)

// HookPlacement keeps optional projection scope outside the copied declaration value.
type HookPlacement struct {
	ProfileID          string   `json:"profile_id,omitempty" yaml:"profile_id,omitempty"`
	ShadowedWorkspaces []string `json:"-"                    yaml:"-"`
}

func (d HookDecl) PlacementProfileID() string {
	if d.HookPlacement == nil {
		return ""
	}
	return d.HookPlacement.ProfileID
}

func (d HookDecl) PlacementWorkspaces() []string {
	if d.HookPlacement == nil {
		return nil
	}
	return d.HookPlacement.ShadowedWorkspaces
}

func (d HookDecl) WithPlacement(profileID string, workspaces []string) HookDecl {
	if strings.TrimSpace(profileID) == "" && len(workspaces) == 0 {
		d.HookPlacement = nil
	} else {
		d.HookPlacement = &HookPlacement{
			ProfileID:          strings.TrimSpace(profileID),
			ShadowedWorkspaces: slices.Clone(workspaces),
		}
	}
	return d
}

func (d HookDecl) ClonePlacement() *HookPlacement {
	return d.WithPlacement(d.PlacementProfileID(), d.PlacementWorkspaces()).HookPlacement
}
