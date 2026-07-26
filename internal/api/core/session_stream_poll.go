package core

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

func sessionStreamStopped(ctx context.Context, streamDone <-chan struct{}) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}
	select {
	case <-streamDone:
		return true
	default:
		return false
	}
}

func (h *BaseHandlers) pollSessionStreamTick(
	c *gin.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	pollQuery store.EventQuery,
	afterSequence int64,
) (int64, *session.Info, bool) {
	pollQuery.AfterSequence = afterSequence

	startedAt := time.Now()
	events, pollErr := h.Sessions.Events(c.Request.Context(), sessionID, pollQuery)
	h.logSessionStreamPoll(
		c.Request.Context(),
		sessionID,
		info,
		contract.SessionStreamFrameRaw,
		afterSequence,
		len(events),
		time.Since(startedAt),
		pollErr,
	)
	if pollErr != nil {
		h.writeSSEBestEffort(writer, SSEMessage{
			Name: sessionStreamErrorKey,
			Data: ErrorPayloadForError(pollErr),
		})
		return afterSequence, info, true
	}

	nextSequence, err := h.writeSessionEventBatch(writer, events, info)
	if err != nil {
		return nextSequence, info, true
	}
	if nextSequence > afterSequence {
		return nextSequence, info, false
	}

	latest, statusErr := h.Sessions.Status(c.Request.Context(), sessionID)
	if statusErr != nil {
		h.writeSSEBestEffort(writer, SSEMessage{
			Name: sessionStreamErrorKey,
			Data: ErrorPayloadForError(statusErr),
		})
		return afterSequence, info, true
	}
	if latest != nil && latest.State == session.StateStopped {
		h.logSSEWriteFailure("session_stopped", h.writeSessionStoppedEvent(writer, latest))
		return afterSequence, latest, true
	}
	if h.IncludeSessionWorkspaceInSSE {
		info = latest
	}

	return afterSequence, info, false
}
