package core

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) requireAggregateSurface(c *gin.Context) bool {
	return h.requireOperatorOrGrantedAgent(c, "aggregate catalogs")
}

func (h *BaseHandlers) aggregateBreadth(c *gin.Context) (WorkspaceBreadth, bool) {
	if !h.requireAggregateSurface(c) {
		return WorkspaceBreadth{}, false
	}
	breadth, err := parseWorkspaceBreadth(c, "", breadthEncoding{Selector: workspaceIDKey})
	if err == nil && !breadth.AllWorkspaces {
		err = invalidWorkspaceBreadth("aggregate routes require all_workspaces=true")
	}
	if err != nil {
		h.respondAggregateBreadthError(c, err)
		return WorkspaceBreadth{}, false
	}
	return breadth, true
}

func (h *BaseHandlers) respondAggregateBreadthError(c *gin.Context, err error) {
	code := workspaceBreadthInvalidCode
	if typed, ok := err.(*workspaceBreadthError); ok {
		code = typed.code
	}
	c.JSON(http.StatusBadRequest, contract.ErrorPayload{Code: code, Error: err.Error()})
}
