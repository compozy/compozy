package core

import (
	"errors"
	"net/http"
	"time"

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
	query, err := ParseLogsQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
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
	resume := !cursor.Timestamp.IsZero()
	if streamContextDone(c, h.StreamDoneChannel(), replay || resume) {
		return
	}

	if resume {
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
	}
	if cursor.Timestamp.IsZero() {
		cursor.Timestamp = h.nowUTC()
	}

	pollQuery := query
	pollQuery.Limit = 0
	pollQuery.Since = cursor.Timestamp
	ticker := time.NewTicker(logsStreamPollInterval(h.PollInterval))
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-ticker.C:
			pollQuery.Since = cursor.Timestamp
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
