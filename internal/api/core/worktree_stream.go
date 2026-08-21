package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

const (
	worktreeCatalogChangedEvent = "worktree_catalog_changed"
	worktreeReplayLimit         = 500
)

func (h *BaseHandlers) StreamWorktreeCatalog(c *gin.Context) {
	subscriber, ok := h.Worktrees.(WorktreeCatalogEventSubscriber)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: worktree catalog stream is required"))
		return
	}
	events, cancel, err := subscriber.SubscribeWorktreeCatalogEvents(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	defer cancel()
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := WriteSSEComment(writer, "worktree catalog stream ready"); err != nil {
		h.logSSEWriteFailure(worktreeCatalogChangedEvent, err)
		return
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case event, open := <-events:
			if !open {
				return
			}
			if err := WriteSSE(writer, SSEMessage{
				Name: worktreeCatalogChangedEvent,
				Data: contract.WorktreeCatalogEventPayload{
					Kind: string(event.Kind), WorkspaceID: event.WorkspaceID, WorktreeID: event.WorktreeID,
					Error: event.Error,
				},
			}); err != nil {
				h.logSSEWriteFailure(worktreeCatalogChangedEvent, err)
				return
			}
		}
	}
}

func (h *BaseHandlers) StreamWorktree(c *gin.Context) {
	scope, id, ok := h.worktreeRoute(c)
	if !ok {
		return
	}
	if h.Observer == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: observer is required"))
		return
	}
	inspection, err := h.Worktrees.Inspect(c.Request.Context(), scope.RegistryID, id)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}
	worktreeID := inspection.Worktree.ID
	afterSequence, err := h.parseWorktreeAfterSequence(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := WriteSSEComment(writer, "worktree stream ready"); err != nil {
		h.logSSEWriteFailure("worktree_stream", err)
		return
	}
	query := store.EventSummaryQuery{
		ReadScope:   store.ReadScope{AllProfiles: true},
		WorkspaceID: scope.RegistryID, WorktreeID: worktreeID, AfterSequence: afterSequence, Limit: worktreeReplayLimit,
	}
	afterSequence, ok = h.replayWorktreeEvents(c, writer, query)
	if !ok {
		return
	}
	ticker := time.NewTicker(h.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-ticker.C:
			query.AfterSequence = afterSequence
			afterSequence, ok = h.replayWorktreeEvents(c, writer, query)
			if !ok {
				return
			}
		}
	}
}

func (h *BaseHandlers) replayWorktreeEvents(
	c *gin.Context,
	writer FlushWriter,
	query store.EventSummaryQuery,
) (int64, bool) {
	cursor := query.AfterSequence
	for {
		query.AfterSequence = cursor
		events, err := h.Observer.QueryEvents(c.Request.Context(), query)
		if err != nil {
			h.writeSSEBestEffort(writer, SSEMessage{Name: handlersErrorKey, Data: ErrorPayloadForError(err)})
			return cursor, false
		}
		for _, event := range events {
			if event.Sequence <= cursor {
				continue
			}
			payload := event.ContentValue()
			if len(payload) == 0 {
				payload = json.RawMessage("{}")
			}
			if err := WriteSSE(writer, SSEMessage{
				ID: strconv.FormatInt(event.Sequence, 10), Name: event.Type, Data: payload,
			}); err != nil {
				h.logSSEWriteFailure(event.Type, err)
				return cursor, false
			}
			cursor = event.Sequence
		}
		if len(events) < query.Limit {
			return cursor, true
		}
	}
}

func (h *BaseHandlers) parseWorktreeAfterSequence(c *gin.Context) (int64, error) {
	raw := strings.TrimSpace(c.Query("after_sequence"))
	if lastEventID := strings.TrimSpace(c.GetHeader("Last-Event-ID")); lastEventID != "" {
		raw = lastEventID
	}
	if raw == "" {
		return 0, nil
	}
	sequence, err := parseLastEventID(raw, h.transportName())
	if err != nil {
		return 0, fmt.Errorf("invalid worktree stream cursor: %w", err)
	}
	return sequence, nil
}

var _ WorktreeCatalogEventSubscriber = (*worktree.Service)(nil)
