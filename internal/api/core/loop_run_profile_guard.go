package core

import (
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) requireLoopRunProfile(c *gin.Context, service LoopService, mutation bool) bool {
	return h.requireLoopRunProfileByID(c, service, c.Param("run_id"), mutation)
}

func (h *BaseHandlers) requireLoopRunProfileByID(
	c *gin.Context,
	service LoopService,
	runID string,
	mutation bool,
) bool {
	var (
		scope interface{ Matches(string) bool }
		err   error
	)
	if mutation {
		scope, err = h.resolveProfileMutationScope(c)
	} else {
		scope, err = h.resolveProfileReadScope(c)
	}
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return false
	}
	response, err := service.GetLoopRun(
		c.Request.Context(),
		c.Param("workspace_id"),
		strings.TrimSpace(runID),
	)
	if err != nil {
		h.respondLoopError(c, err)
		return false
	}
	if !scope.Matches(response.Run.ProfileID) {
		h.respondLoopError(c, looppkg.ErrRunNotFound)
		return false
	}
	return true
}
