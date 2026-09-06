package core

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

const (
	logsStreamReplayDefaultLimit = 200
	logsStreamReplayMaxLimit     = 500
	logsStreamMinimumPoll        = time.Second
)

// StreamLogs streams runtime logs over SSE with a bounded retained replay.
func (h *BaseHandlers) StreamLogs(c *gin.Context) {
	if h.Observer == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: observer is required"))
		return
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	query, err := ParseLogsQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	query.ReadScope = readScope
	if err := query.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	replay, err := parseLogsReplay(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	cursor, err := ParseLogsCursor(c.GetHeader("Last-Event-ID"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	resume := !cursor.Timestamp.IsZero() || cursor.Sequence > 0
	if streamContextDone(c, h.StreamDoneChannel(), replay || resume) {
		return
	}

	query.SequenceOrder = cursor.ID == ""
	if resume && cursor.Sequence > 0 {
		query.AfterSequence = cursor.Sequence
		query.Forward = true
	} else if resume {
		query.Since = cursor.Timestamp
	}
	if replay || resume {
		query.Limit = normalizeLogsReplayLimit(query.Limit)
		initial, queryErr := h.Observer.QueryEvents(c.Request.Context(), query)
		if queryErr != nil {
			h.writeSSEBestEffort(writer, SSEMessage{
				Name: handlersErrorKey,
				Data: ErrorPayloadForError(queryErr),
			})
			return
		}
		cursor = EmitLogs(writer, initial, cursor)
	} else {
		cursor, err = h.currentLogsCursor(c.Request.Context(), query)
		if err != nil {
			h.writeSSEBestEffort(writer, SSEMessage{Name: handlersErrorKey, Data: ErrorPayloadForError(err)})
			return
		}
	}
	// An empty durable head starts at sequence zero, independently of event timestamps.
	h.pollLogs(c, writer, query, cursor)
}

func (h *BaseHandlers) currentLogsCursor(ctx context.Context, query store.EventSummaryQuery) (LogsCursor, error) {
	query.Limit = 1
	head, err := h.Observer.QueryEvents(ctx, query)
	if err != nil {
		return LogsCursor{}, err
	}
	if len(head) == 0 {
		return LogsCursor{}, nil
	}
	latest := head[len(head)-1]
	return LogsCursor{Timestamp: latest.Timestamp, Sequence: latest.Sequence, ID: latest.ID}, nil
}

func (h *BaseHandlers) pollLogs(
	c *gin.Context,
	writer FlushWriter,
	query store.EventSummaryQuery,
	cursor LogsCursor,
) {
	pollQuery := query
	pollQuery.Limit = logsStreamReplayDefaultLimit
	ticker := time.NewTicker(logsStreamPollInterval(h.PollInterval))
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-ticker.C:
			pollQuery.AfterSequence = cursor.Sequence
			pollQuery.Forward = cursor.Sequence > 0 || cursor.ID == ""
			if pollQuery.Forward {
				pollQuery.Since = query.Since
			} else {
				pollQuery.Since = cursor.Timestamp
			}
			events, pollErr := h.Observer.QueryEvents(c.Request.Context(), pollQuery)
			if pollErr != nil {
				h.writeSSEBestEffort(writer, SSEMessage{
					Name: handlersErrorKey,
					Data: ErrorPayloadForError(pollErr),
				})
				return
			}
			cursor = EmitLogs(writer, events, cursor)
		}
	}
}

func parseLogsReplay(c *gin.Context) (bool, error) {
	raw, exists := c.GetQuery("replay")
	if !exists {
		return true, nil
	}
	return ParseOptionalBool(raw)
}

func normalizeLogsReplayLimit(limit int) int {
	if limit <= 0 {
		return logsStreamReplayDefaultLimit
	}
	if limit > logsStreamReplayMaxLimit {
		return logsStreamReplayMaxLimit
	}
	return limit
}

func logsStreamPollInterval(interval time.Duration) time.Duration {
	if interval < logsStreamMinimumPoll {
		return logsStreamMinimumPoll
	}
	return interval
}

func streamContextDone(c *gin.Context, done <-chan struct{}, allowInitial bool) bool {
	select {
	case <-c.Request.Context().Done():
		return true
	default:
	}
	if allowInitial {
		return false
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}
