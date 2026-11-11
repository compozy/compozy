package core

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) respondToolSearch(c *gin.Context, scope toolspkg.Scope, req contract.ToolSearchRequest) {
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Offset < 0 || req.Limit < 1 || req.Limit > 100 {
		h.respondToolError(
			c,
			toolspkg.NewToolError(
				toolspkg.ErrorCodeInvalidInput,
				"",
				"offset must be nonnegative and limit must be between 0 and 100",
				toolspkg.ErrToolInvalidInput,
				toolspkg.ReasonSchemaInvalid,
			),
		)
		return
	}
	views, err := h.Tools.Search(c.Request.Context(), scope, toolspkg.SearchQuery{
		Query: req.Query,
	})
	if err != nil {
		h.respondToolError(c, err)
		return
	}
	start := min(req.Offset, len(views))
	end := start + min(req.Limit, len(views)-start)
	next := 0
	if end < len(views) {
		next = end
	}
	c.JSON(
		http.StatusOK,
		contract.ToolsResponse{
			Tools:      ToolPayloadsFromViews(views[start:end]),
			Offset:     req.Offset,
			Count:      end - start,
			Total:      len(views),
			NextOffset: next,
			HasMore:    end < len(views),
		},
	)
}
