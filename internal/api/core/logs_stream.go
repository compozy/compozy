package core

import (
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

// StreamLogs streams runtime logs as SSE or WebSocket frames with bounded replay.
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
	cursor, err := ParseLogsCursor(eventStreamCursor(c))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	writer, closeStream, ok := h.prepareEventStream(c)
	if !ok {
		return
	}
	defer closeStream()
	resume := !cursor.Timestamp.IsZero() || cursor.Sequence > 0
	if streamContextDone(c, h.StreamDoneChannel(), replay || resume) {
		return
	}

	// The head is read before the first query so every row at or below it is
	// already visible to that query and to each later poll.
	scan, err := h.Observer.EventSequenceHead(c.Request.Context())
	if err != nil {
		h.writeSSEBestEffort(writer, SSEMessage{Name: handlersErrorKey, Data: ErrorPayloadForError(err)})
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
		scan = nextLogsScanSequence(cursor.Sequence, scan, initial, query.Limit, query.Forward || resume)
	}
	// A live-only stream starts at the ledger head without reading a row; an empty
	// ledger starts at sequence zero, independently of event timestamps.
	h.pollLogs(c, writer, query, cursor, scan)
}

// Full forward or resume pages retain their cursor until the backlog drains.
func nextLogsScanSequence(scan int64, head int64, events []store.EventSummary, limit int, keepFullPage bool) int64 {
	for _, event := range events {
		scan = max(scan, event.Sequence)
	}
	if keepFullPage && limit > 0 && len(events) >= limit {
		return scan
	}
	return max(scan, head)
}

func (h *BaseHandlers) pollLogs(
	c *gin.Context,
	writer FlushWriter,
	query store.EventSummaryQuery,
	cursor LogsCursor,
	scan int64,
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
			head, headErr := h.Observer.EventSequenceHead(c.Request.Context())
			if headErr != nil {
				h.writeSSEBestEffort(writer, SSEMessage{Name: handlersErrorKey, Data: ErrorPayloadForError(headErr)})
				return
			}
			pollQuery.AfterSequence = scan
			pollQuery.Forward = scan > 0 || cursor.ID == ""
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
			scan = nextLogsScanSequence(scan, head, events, pollQuery.Limit, pollQuery.Forward)
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
	return min(limit, logsStreamReplayMaxLimit)
}

func logsStreamPollInterval(interval time.Duration) time.Duration {
	return max(interval, logsStreamMinimumPoll)
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
