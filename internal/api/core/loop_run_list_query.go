package core

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

// ParseLoopRunListQuery parses shared Loop run list filters.
func ParseLoopRunListQuery(c *gin.Context) (LoopRunListQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return LoopRunListQuery{}, err
	}
	query := LoopRunListQuery{
		LoopName:      strings.TrimSpace(c.Query("loop")),
		Status:        strings.TrimSpace(c.Query("status")),
		Origin:        strings.TrimSpace(c.Query("origin")),
		OriginSession: strings.TrimSpace(c.Query("origin_session")),
		Cursor:        strings.TrimSpace(c.Query("cursor")),
		Limit:         limit,
	}
	if raw, present := c.GetQuery("live"); present {
		if strings.TrimSpace(raw) == "" {
			return LoopRunListQuery{}, errors.New("live must be nonempty")
		}
		live, err := ParseOptionalBool(raw)
		if err != nil {
			return LoopRunListQuery{}, err
		}
		query.Live = &live
	}
	return query, nil
}
