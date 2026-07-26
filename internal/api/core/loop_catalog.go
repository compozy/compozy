package core

import (
	"fmt"
	"net/http"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/gin-gonic/gin"
)

// ListLoops returns one bounded workspace Loop catalog page.
func (h *BaseHandlers) ListLoops(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	query, err := ParseLoopCatalogQuery(c)
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: %v", looppkg.ErrCatalogQueryInvalid, err))
		return
	}
	response, err := service.ListLoops(c.Request.Context(), c.Param("workspace_id"), query)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// ParseLoopCatalogQuery parses the shared HTTP/UDS catalog filters.
func ParseLoopCatalogQuery(c *gin.Context) (looppkg.CatalogQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return looppkg.CatalogQuery{}, err
	}
	return looppkg.CatalogQuery{
		Search:   c.Query("q"),
		Kind:     looppkg.CatalogKind(c.Query("kind")),
		Category: c.Query("category"),
		Status:   looppkg.Status(c.Query("status")),
		Sort:     c.Query("sort"),
		Cursor:   c.Query("cursor"),
		Limit:    limit,
	}, nil
}
