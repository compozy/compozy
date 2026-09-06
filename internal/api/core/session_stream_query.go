package core

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) parseSessionStreamEventQuery(c *gin.Context) (store.EventQuery, error) {
	query, err := ParseSessionEventQuery(c)
	if err != nil {
		return store.EventQuery{}, err
	}
	if lastEventID := strings.TrimSpace(c.GetHeader("Last-Event-ID")); lastEventID != "" {
		query.Forward = true
		query.AfterSequence, err = parseLastEventID(lastEventID, h.transportName())
		if err != nil {
			return store.EventQuery{}, err
		}
	}
	if err := query.Validate(); err != nil {
		return store.EventQuery{}, fmt.Errorf("invalid stream query: %w", err)
	}
	return query, nil
}

func normalizeSessionStreamQuery(
	query store.EventQuery,
	options sessionStreamOptions,
) (store.EventQuery, error) {
	if options.frameMode == contract.SessionStreamFrameRaw {
		if query.AfterSequence == 0 && query.Limit == 0 {
			return store.EventQuery{}, fmt.Errorf(
				"raw initial catch-up requires a bounded limit; use frames=transcript for a snapshot",
			)
		}
		return applyBoundedSessionReadDefault(query)
	}
	if query.Type != "" || query.AgentName != "" || query.TurnID != "" || !query.Since.IsZero() {
		return store.EventQuery{}, fmt.Errorf("raw event filters are not supported for transcript streams")
	}
	changeQuery, err := (transcript.ChangeQuery{Limit: query.Limit}).Normalize()
	if err != nil {
		return store.EventQuery{}, err
	}
	query.Limit = changeQuery.Limit
	return query, nil
}
