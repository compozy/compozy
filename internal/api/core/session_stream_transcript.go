package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

type transcriptStreamState struct {
	cursor     int64
	generation int64
	epoch      int64
}

func (h *BaseHandlers) streamTranscriptSessionEvents(
	c *gin.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	query store.EventQuery,
	initial []store.SessionEvent,
	options sessionStreamOptions,
	subscription sessionEventStreamSubscription,
) {
	defer subscription.cancelIfActive()

	state, err := h.initializeTranscriptStream(
		c.Request.Context(),
		writer,
		sessionID,
		info,
		query.AfterSequence,
		query.Limit,
		options,
		initial,
	)
	if err != nil {
		h.writeTranscriptStreamError(writer, fmt.Errorf("initialize transcript stream: %w", err))
		return
	}
	if info != nil && info.State == session.StateStopped {
		h.logSSEWriteFailure("session_stopped", h.writeSessionStoppedEvent(writer, info))
		return
	}
	if subscription.active() {
		h.pushAndStreamSessionTranscript(c, writer, sessionID, info, state, query.Limit, subscription)
		return
	}
	h.pollAndStreamSessionTranscript(c, writer, sessionID, info, state, query.Limit)
}

func (h *BaseHandlers) initializeTranscriptStream(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	cursor int64,
	limit int,
	options sessionStreamOptions,
	namedEvents []store.SessionEvent,
) (transcriptStreamState, error) {
	if err := h.writeGoalSnapshotChangedEvents(ctx, writer, sessionID, cursor, namedEvents); err != nil {
		return transcriptStreamState{}, err
	}
	if cursor == 0 {
		snapshot, err := h.writeTranscriptSnapshot(ctx, writer, sessionID, info, limit, false, "")
		return transcriptStreamState{
			cursor:     snapshot.cursor,
			generation: snapshot.generation,
			epoch:      transcriptEpoch(info),
		}, err
	}

	first, err := h.queryTranscriptChangesPage(ctx, sessionID, info, cursor, limit)
	if err != nil {
		return transcriptStreamState{}, err
	}
	reason := transcriptReconnectResetReason(
		cursor,
		options.expectedEpoch,
		options.expectedGeneration,
		transcriptEpoch(info),
		first.Generation,
		first.MaxSequence,
	)
	if reason != "" {
		snapshot, snapshotErr := h.writeTranscriptSnapshot(ctx, writer, sessionID, info, limit, true, reason)
		return transcriptStreamState{
			cursor:     snapshot.cursor,
			generation: snapshot.generation,
			epoch:      transcriptEpoch(info),
		}, snapshotErr
	}

	nextCursor, generation, err := h.writeTranscriptChangePages(
		ctx,
		writer,
		sessionID,
		info,
		cursor,
		limit,
		&first,
	)
	return transcriptStreamState{
		cursor:     nextCursor,
		generation: generation,
		epoch:      transcriptEpoch(info),
	}, err
}

func (h *BaseHandlers) refreshTranscriptStream(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	state transcriptStreamState,
	limit int,
	namedEvents []store.SessionEvent,
) (transcriptStreamState, *session.Info, error) {
	if err := h.writeGoalSnapshotChangedEvents(ctx, writer, sessionID, state.cursor, namedEvents); err != nil {
		return state, info, err
	}
	latest, err := h.Sessions.Status(ctx, sessionID)
	if err != nil {
		return state, info, fmt.Errorf("query transcript stream status: %w", err)
	}
	if latest != nil && transcriptEpoch(latest) != state.epoch {
		state, err = h.resetTranscriptStream(
			ctx,
			writer,
			sessionID,
			latest,
			limit,
			contract.TranscriptSnapshotReasonEpochMismatch,
		)
		return state, latest, err
	}
	if latest != nil {
		info = latest
	}

	first, err := h.queryTranscriptChangesPage(ctx, sessionID, info, state.cursor, limit)
	if err != nil {
		return state, info, err
	}
	resetReason := ""
	switch {
	case first.Generation != state.generation:
		resetReason = contract.TranscriptSnapshotReasonGenerationMismatch
	case state.cursor > first.MaxSequence:
		resetReason = contract.TranscriptSnapshotReasonSequenceReset
	}
	if resetReason != "" {
		state, err = h.resetTranscriptStream(ctx, writer, sessionID, info, limit, resetReason)
		return state, info, err
	}

	nextCursor, generation, err := h.writeTranscriptChangePages(
		ctx,
		writer,
		sessionID,
		info,
		state.cursor,
		limit,
		&first,
	)
	if err != nil {
		return state, info, err
	}
	state.cursor = nextCursor
	state.generation = generation
	return state, info, nil
}

func (h *BaseHandlers) resetTranscriptStream(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	limit int,
	reason string,
) (transcriptStreamState, error) {
	snapshot, err := h.writeTranscriptSnapshot(ctx, writer, sessionID, info, limit, true, reason)
	return transcriptStreamState{
		cursor:     snapshot.cursor,
		generation: snapshot.generation,
		epoch:      transcriptEpoch(info),
	}, err
}

func (h *BaseHandlers) pollAndStreamSessionTranscript(
	c *gin.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	state transcriptStreamState,
	limit int,
) {
	ticker := time.NewTicker(h.PollInterval)
	defer ticker.Stop()
	keepAlive := time.NewTicker(sessionStreamKeepAliveInterval)
	defer keepAlive.Stop()

	currentInfo := info
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-keepAlive.C:
			if !h.writeKeepAlive(writer) {
				return
			}
		case <-ticker.C:
			var err error
			state, currentInfo, err = h.refreshTranscriptStream(
				c.Request.Context(), writer, sessionID, currentInfo, state, limit, nil,
			)
			if err != nil {
				h.writeTranscriptStreamError(writer, err)
				return
			}
			if currentInfo != nil && currentInfo.State == session.StateStopped {
				h.logSSEWriteFailure("session_stopped", h.writeSessionStoppedEvent(writer, currentInfo))
				return
			}
		}
	}
}

func (h *BaseHandlers) pushAndStreamSessionTranscript(
	c *gin.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	state transcriptStreamState,
	limit int,
	subscription sessionEventStreamSubscription,
) {
	keepAlive := time.NewTicker(sessionStreamKeepAliveInterval)
	defer keepAlive.Stop()

	currentInfo := info
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-keepAlive.C:
			if !h.writeKeepAlive(writer) {
				return
			}
		case event, ok := <-subscription.events:
			if !ok {
				h.logSessionStreamSubscriptionClosed(
					c.Request.Context(), sessionID, currentInfo, contract.SessionStreamFrameTranscript, state.cursor,
				)
				subscription.cancelIfActive()
				h.pollAndStreamSessionTranscript(c, writer, sessionID, currentInfo, state, limit)
				return
			}
			var err error
			state, currentInfo, err = h.refreshTranscriptStream(
				c.Request.Context(), writer, sessionID, currentInfo, state, limit, []store.SessionEvent{event},
			)
			if err != nil {
				h.writeTranscriptStreamError(writer, err)
				return
			}
			if event.Type == session.EventTypeSessionStopped ||
				(currentInfo != nil && currentInfo.State == session.StateStopped) {
				h.logSSEWriteFailure("session_stopped", h.writeSessionStoppedEvent(writer, currentInfo))
				return
			}
		}
	}
}

func (h *BaseHandlers) writeTranscriptStreamError(writer FlushWriter, err error) {
	if errors.Is(err, session.ErrSessionNotFound) {
		return
	}
	h.writeSSEBestEffort(writer, SSEMessage{Name: sessionStreamErrorKey, Data: ErrorPayloadForError(err)})
}
