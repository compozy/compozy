package core

import (
	"context"

	profilepkg "github.com/compozy/compozy/internal/profile"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

type TerminalProvider interface {
	TerminalFor(profileID string) (terminalpkg.Manager, error)
	Observe(func(context.Context, terminalpkg.TerminalEvent))
}

func (h *BaseHandlers) terminalService(
	c *gin.Context,
	mutation bool,
) (terminalpkg.Manager, string, bool) {
	if h == nil || h.Terminal == nil {
		if h != nil {
			h.respondTerminalUnavailable(c)
		}
		return nil, "", false
	}
	var scopeID string
	var err error
	if mutation {
		scope, scopeErr := h.resolveProfileMutationScope(c)
		err, scopeID = scopeErr, scope.ProfileID
	} else {
		scope, scopeErr := h.resolveProfileReadScope(c)
		err, scopeID = scopeErr, scope.ProfileID
		if scopeErr == nil && scope.AllProfiles {
			err = &profilepkg.Error{
				Code: profileSelectionConflictCode, Message: "this terminal read belongs to one profile",
				Action: "choose one profile", Cause: profilepkg.ErrInvalidInput,
			}
		}
	}
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return nil, "", false
	}
	service, err := h.Terminal.TerminalFor(scopeID)
	if err != nil {
		h.respondTerminalError(c, err)
		return nil, "", false
	}
	return service, scopeID, true
}
