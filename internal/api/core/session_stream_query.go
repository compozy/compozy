package core

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) parseSessionStreamEventQuery(c *gin.Context) (store.EventQuery, error) {
	query, err := ParseSessionEventQuery(c)
	if err != nil {
		return store.EventQuery{}, err
	}
	if lastEventID := strings.TrimSpace(c.GetHeader("Last-Event-ID")); lastEventID != "" {
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
		return query, nil
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
