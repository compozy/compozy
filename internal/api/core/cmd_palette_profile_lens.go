package core

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) resolveCmdPaletteProfileLens(
	c *gin.Context,
	mutation bool,
) (cmdpalette.ProfileLens, bool) {
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return cmdpalette.ProfileLens{}, false
	}
	if mutation && readScope.AllProfiles {
		_, err = h.resolveProfileMutationScope(c)
		h.respondProfileReadScopeError(c, err)
		return cmdpalette.ProfileLens{}, false
	}
	if readScope.AllProfiles {
		return cmdpalette.AggregateProfileLens(), true
	}
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		h.respondCmdPaletteError(c, "", fmt.Errorf("resolve command palette profile owner: %w", err))
		return cmdpalette.ProfileLens{}, false
	}
	owner, ok := owners[strings.TrimSpace(readScope.ProfileID)]
	if !ok {
		h.respondCmdPaletteError(c, "", fmt.Errorf("command palette profile owner is unavailable"))
		return cmdpalette.ProfileLens{}, false
	}
	lens := cmdpalette.ScopedProfileLens(cmdpalette.ProfileLensID(owner.ID), owner.Name)
	if err := lens.Validate(); err != nil {
		h.respondCmdPaletteError(c, "", err)
		return cmdpalette.ProfileLens{}, false
	}
	return lens, true
}
