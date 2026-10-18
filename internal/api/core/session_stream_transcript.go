package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

type transcriptStreamState struct {
	subagentUpdates  <-chan session.SubagentUpdate
	cursor           int64
	usageCursor      int64
	generation       int64
	epoch            int64
	commandRevision  string
	commandCheckedAt time.Time
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
	updates, cancel, err := h.subscribeSubagents(c.Request.Context(), sessionID)
	if err != nil {
		h.writeTranscriptStreamError(writer, err)
		return
	}
	defer cancel()

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
	state.subagentUpdates = updates
	if err := h.writeSubagentsSnapshot(c.Request.Context(), writer, sessionID, info); err != nil {
		h.writeTranscriptStreamError(writer, err)
		return
	}
	state.commandRevision, err = h.writeSessionCommandsChanged(
		c.Request.Context(), writer, sessionID, "",
	)
	if err != nil {
		h.writeTranscriptStreamError(writer, err)
		return
	}
	state.commandCheckedAt = time.Now()
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
	usageCursor, err := h.writeUsageChangedEvents(ctx, writer, sessionID, cursor, namedEvents)
	if err != nil {
		return transcriptStreamState{}, err
	}
	if err := h.writeGoalSnapshotChangedEvents(ctx, writer, sessionID, cursor, namedEvents); err != nil {
		return transcriptStreamState{}, err
	}
	if cursor == 0 {
		snapshot, err := h.writeTranscriptSnapshot(ctx, writer, sessionID, info, limit, false, "")
		return transcriptStreamState{
			cursor:      snapshot.cursor,
			usageCursor: usageCursor,
			generation:  snapshot.generation,
			epoch:       transcriptEpoch(info),
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
	if reason == "" && first.MinSequence > cursor {
		reason = contract.TranscriptSnapshotReasonCursorExpired
	}
	if reason != "" {
		snapshot, snapshotErr := h.writeTranscriptSnapshot(ctx, writer, sessionID, info, limit, true, reason)
		return transcriptStreamState{
			cursor:      snapshot.cursor,
			usageCursor: usageCursor,
			generation:  snapshot.generation,
			epoch:       transcriptEpoch(info),
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
	if err == nil && nextCursor == cursor && len(first.Entries) == 0 {
		err = h.writeTranscriptDeltaPage(writer, sessionID, info, first, nextCursor)
	}
	return transcriptStreamState{
		cursor:      nextCursor,
		usageCursor: usageCursor,
		generation:  generation,
		epoch:       transcriptEpoch(info),
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
	state, err := h.refreshTranscriptCommands(ctx, writer, sessionID, state)
	if err != nil {
		return state, info, err
	}
	if err := h.writeGoalSnapshotChangedEvents(ctx, writer, sessionID, state.cursor, namedEvents); err != nil {
		return state, info, err
	}
	latest, err := h.Sessions.Status(ctx, sessionID)
	if err != nil {
		return state, info, fmt.Errorf("query transcript stream status: %w", err)
	}
	if latest != nil && transcriptEpoch(latest) != state.epoch {
		resetState, resetErr := h.resetTranscriptStream(
			ctx,
			writer,
			sessionID,
			latest,
			limit,
			contract.TranscriptSnapshotReasonEpochMismatch,
		)
		resetState.subagentUpdates = state.subagentUpdates
		resetState.commandRevision = state.commandRevision
		resetState.commandCheckedAt = state.commandCheckedAt
		return resetState, latest, resetErr
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
	case state.cursor > 0 && first.MinSequence > state.cursor:
		resetReason = contract.TranscriptSnapshotReasonCursorExpired
	}
	if resetReason != "" {
		resetState, resetErr := h.resetTranscriptStream(ctx, writer, sessionID, info, limit, resetReason)
		resetState.subagentUpdates = state.subagentUpdates
		resetState.commandRevision = state.commandRevision
		resetState.commandCheckedAt = state.commandCheckedAt
		return resetState, info, resetErr
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
	usageCursor, err := h.writeUsageChangedEvents(ctx, writer, sessionID, state.usageCursor, namedEvents)
	if err != nil {
		return state, info, err
	}
	state.usageCursor = usageCursor
	state.cursor = nextCursor
	state.generation = generation
	return state, info, nil
}

func (h *BaseHandlers) refreshTranscriptCommands(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	state transcriptStreamState,
) (transcriptStreamState, error) {
	if time.Since(state.commandCheckedAt) < time.Second {
		return state, nil
	}
	return h.updateTranscriptCommands(ctx, writer, sessionID, state)
}

func (h *BaseHandlers) updateTranscriptCommands(
	ctx context.Context, writer FlushWriter, sessionID string, state transcriptStreamState,
) (transcriptStreamState, error) {
	revision, err := h.writeSessionCommandsChanged(ctx, writer, sessionID, state.commandRevision)
	if err != nil {
		return state, err
	}
	state.commandRevision = revision
	state.commandCheckedAt = time.Now()
	return state, nil
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
		cursor:      snapshot.cursor,
		usageCursor: snapshot.cursor,
		generation:  snapshot.generation,
		epoch:       transcriptEpoch(info),
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
	commandRefresh := time.NewTicker(time.Second)
	defer commandRefresh.Stop()

	currentInfo := info
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case update, ok := <-state.subagentUpdates:
			if !h.streamSubagentUpdate(writer, sessionID, &update, ok) {
				return
			}
		case <-keepAlive.C:
			if !h.writeKeepAlive(writer) {
				return
			}
		case <-commandRefresh.C:
			updated, err := h.updateTranscriptCommands(c.Request.Context(), writer, sessionID, state)
			if err != nil {
				h.writeTranscriptStreamError(writer, err)
				return
			}
			state = updated
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
	c *gin.Context, writer FlushWriter, sessionID string, info *session.Info,
	state transcriptStreamState, limit int, subscription sessionEventStreamSubscription,
) {
	keepAlive := time.NewTicker(sessionStreamKeepAliveInterval)
	defer keepAlive.Stop()
	commandRefresh := time.NewTicker(time.Second)
	defer commandRefresh.Stop()
	flushTimer := time.NewTimer(time.Hour)
	flushTimer.Stop()
	defer flushTimer.Stop()
	var flush <-chan time.Time
	var pending *store.SessionEvent
	currentInfo := info
	refresh := func() bool {
		event := pending
		pending = nil
		flush = nil
		var keepStreaming bool
		state, currentInfo, keepStreaming = h.refreshTranscriptWake(
			c.Request.Context(), writer, sessionID, currentInfo, state, limit, event,
		)
		return keepStreaming
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case update, ok := <-state.subagentUpdates:
			if !h.streamSubagentUpdate(writer, sessionID, &update, ok) {
				return
			}
		case <-keepAlive.C:
			if !h.writeKeepAlive(writer) {
				return
			}
		case <-commandRefresh.C:
			updated, err := h.updateTranscriptCommands(c.Request.Context(), writer, sessionID, state)
			if err != nil {
				h.writeTranscriptStreamError(writer, err)
				return
			}
			state = updated
		case <-flush:
			if !refresh() {
				return
			}
		case event, ok := <-subscription.events:
			if !ok {
				if pending != nil && !refresh() {
					return
				}
				h.logSessionStreamSubscriptionClosed(
					c.Request.Context(),
					sessionID,
					currentInfo,
					contract.SessionStreamFrameTranscript,
					state.cursor,
				)
				subscription.cancelIfActive()
				h.pollAndStreamSessionTranscript(c, writer, sessionID, currentInfo, state, limit)
				return
			}
			immediate, err := h.prepareTranscriptWake(writer, sessionID, state.cursor, event)
			if err != nil {
				return
			}
			pending = &event
			if immediate {
				flushTimer.Stop()
				if !refresh() {
					return
				}
			} else if flush == nil {
				flushTimer.Reset(25 * time.Millisecond)
				flush = flushTimer.C
			}
		}
	}
}

func (h *BaseHandlers) prepareTranscriptWake(
	writer FlushWriter, sessionID string, cursor int64, event store.SessionEvent,
) (bool, error) {
	if event.Type == contract.SessionStreamEventConsumerDegraded {
		return true, h.writeConsumerDegraded(writer, sessionID, cursor, event)
	}
	return event.Type == session.EventTypeSessionStopped, nil
}

func (h *BaseHandlers) refreshTranscriptWake(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	state transcriptStreamState,
	limit int,
	event *store.SessionEvent,
) (transcriptStreamState, *session.Info, bool) {
	for {
		previous := state
		var err error
		state, info, err = h.refreshTranscriptStream(
			ctx,
			writer,
			sessionID,
			info,
			state,
			limit,
			nil,
		)
		if err != nil {
			h.writeTranscriptStreamError(writer, err)
			return state, info, false
		}
		if event == nil || state.cursor >= event.Sequence || state.generation != previous.generation ||
			state.epoch != previous.epoch {
			break
		}
		if state.cursor <= previous.cursor {
			h.writeTranscriptStreamError(
				writer,
				fmt.Errorf("transcript did not reach durable wake watermark %d", event.Sequence),
			)
			return state, info, false
		}
	}

	if (event != nil && event.Type == session.EventTypeSessionStopped) ||
		(info != nil && info.State == session.StateStopped) {
		h.logSSEWriteFailure("session_stopped", h.writeSessionStoppedEvent(writer, info))
		return state, info, false
	}
	return state, info, true
}

func (h *BaseHandlers) writeTranscriptStreamError(writer FlushWriter, err error) {
	if errors.Is(err, session.ErrSessionNotFound) {
		return
	}
	h.writeSSEBestEffort(writer, SSEMessage{Name: sessionStreamErrorKey, Data: ErrorPayloadForError(err)})
}
