package core

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
	"github.com/gin-gonic/gin"
)

const (
	defaultSessionReadLimit = 200
	maxSessionReadLimit     = 1000
)

func applyBoundedSessionReadDefault(query store.EventQuery) (store.EventQuery, error) {
	if query.Limit <= 0 {
		query.Limit = defaultSessionReadLimit
	}
	if query.Limit > maxSessionReadLimit {
		return store.EventQuery{}, fmt.Errorf("session read limit must be <= %d", maxSessionReadLimit)
	}
	if err := query.Validate(); err != nil {
		return store.EventQuery{}, err
	}
	return query, nil
}

func rejectTranscriptBackwardCursor(c *gin.Context) error {
	if strings.TrimSpace(c.Query("before_sequence")) == "" {
		return nil
	}
	return fmt.Errorf("before_sequence is only supported on /transcript")
}

func parseSessionTranscriptQuery(c *gin.Context) (transcript.PageQuery, error) {
	for _, name := range []string{"after_sequence", "since", "type", "agent_name", "turn_id"} {
		if _, ok := c.GetQuery(name); ok {
			return transcript.PageQuery{}, fmt.Errorf("%s is not supported on /transcript", name)
		}
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return transcript.PageQuery{}, err
	}
	beforeSequence, err := ParseOptionalInt64(c.Query("before_sequence"))
	if err != nil {
		return transcript.PageQuery{}, err
	}
	query := transcript.PageQuery{
		Limit:          limit,
		BeforeSequence: beforeSequence,
	}
	return query.Normalize()
}
