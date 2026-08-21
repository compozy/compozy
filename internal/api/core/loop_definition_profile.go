package core

import (
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) loopDefinitionReadProfileID(c *gin.Context) (string, bool) {
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return "", false
	}
	if scope.AllProfiles {
		h.respondLoopError(c, fmt.Errorf("%w: loop definitions require one profile", looppkg.ErrValidation))
		return "", false
	}
	return scope.ProfileID, true
}

func (h *BaseHandlers) loopDefinitionMutationProfileID(c *gin.Context) (string, bool) {
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return "", false
	}
	return scope.ProfileID, true
}
