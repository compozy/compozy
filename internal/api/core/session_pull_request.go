package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *BaseHandlers) RefreshSessionPullRequest(c *gin.Context) {
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	info, err := h.Sessions.Status(c.Request.Context(), c.Param("session_id"))
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	if !scope.Matches(info.ProfileID) {
		c.JSON(http.StatusNotFound, contract.ErrorPayload{Code: "session_not_found", Error: "session not found"})
		return
	}
	c.JSON(http.StatusConflict, contract.ErrorPayload{Code: "session_branch_unavailable", Error: "session has no branch (default branch, detached HEAD, or no git repository)"})
}
