package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/workref"
	"github.com/gin-gonic/gin"
)

const (
	sessionStreamErrorKey = "error"
)

type sessionStreamOptions struct {
	frameMode          string
	expectedEpoch      *int64
	expectedGeneration *int64
}

func parseSessionStreamOptions(c *gin.Context) (sessionStreamOptions, error) {
	frameMode := strings.TrimSpace(c.Query("frames"))
	if frameMode == "" {
		frameMode = contract.SessionStreamFrameTranscript
	}
	switch frameMode {
	case contract.SessionStreamFrameRaw, contract.SessionStreamFrameTranscript:
	default:
		return sessionStreamOptions{}, fmt.Errorf("invalid frames query: %q", frameMode)
	}

	if replay, supplied := c.GetQuery("replay"); supplied {
		return sessionStreamOptions{}, fmt.Errorf("replay query is no longer supported: %q", strings.TrimSpace(replay))
	}
	expectedEpoch, err := parseSessionStreamExpected(c, "epoch", frameMode)
	if err != nil {
		return sessionStreamOptions{}, err
	}
	expectedGeneration, err := parseSessionStreamExpected(c, "generation", frameMode)
	if err != nil {
		return sessionStreamOptions{}, err
	}

	return sessionStreamOptions{
		frameMode:          frameMode,
		expectedEpoch:      expectedEpoch,
		expectedGeneration: expectedGeneration,
	}, nil
}

func parseSessionStreamExpected(c *gin.Context, name string, frameMode string) (*int64, error) {
	if frameMode == contract.SessionStreamFrameRaw {
		return nil, nil
	}
	raw, ok := c.GetQuery(name)
	if !ok {
		return nil, nil
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("invalid %s query: value is required when supplied", name)
	}
	value, err := ParseOptionalInt64(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s query: %w", name, err)
	}
	if value < 0 {
		return nil, fmt.Errorf("invalid %s query: value must be non-negative", name)
	}
	return &value, nil
}

func parseLastEventID(lastEventID string, transportName string) (int64, error) {
	trimmed := strings.TrimSpace(lastEventID)
	if trimmed == "" {
		return 0, nil
	}

	after, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid Last-Event-ID %q: %w", transportName, trimmed, err)
	}
	if after < 0 {
		return 0, fmt.Errorf("%s: invalid Last-Event-ID %q: sequence must be non-negative", transportName, trimmed)
	}
	return after, nil
}

func (h *BaseHandlers) streamSessionWithMode(
	c *gin.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	query store.EventQuery,
	initial []store.SessionEvent,
	options sessionStreamOptions,
	subscription sessionEventStreamSubscription,
) {
	switch options.frameMode {
	case contract.SessionStreamFrameRaw:
		h.streamRawSessionEvents(c, writer, sessionID, info, query, initial, subscription)
	default:
		h.streamTranscriptSessionEvents(c, writer, sessionID, info, query, initial, options, subscription)
	}
}

func (h *BaseHandlers) streamRawSessionEvents(
	c *gin.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	query store.EventQuery,
	initial []store.SessionEvent,
	subscription sessionEventStreamSubscription,
) {
	defer subscription.cancelIfActive()

	afterSequence := query.AfterSequence
	nextSequence, err := h.writeSessionEventBatch(writer, initial, info)
	if err != nil {
		return
	}
	if nextSequence > afterSequence {
		afterSequence = nextSequence
	}

	pollQuery := query
	pollQuery.Limit = 0
	if subscription.active() {
		h.pushAndStreamSessionEvents(c, writer, sessionID, info, pollQuery, afterSequence, subscription)
		return
	}
	h.pollAndStreamSessionEvents(c, writer, sessionID, info, pollQuery, afterSequence)
}

func (h *BaseHandlers) writeSessionEventBatch(
	writer FlushWriter,
	events []store.SessionEvent,
	info *session.Info,
) (int64, error) {
	payloadInfo := info
	if !h.IncludeSessionWorkspaceInSSE {
		payloadInfo = nil
	}
	var afterSequence int64
	for _, event := range events {
		afterSequence = event.Sequence
		if err := WriteSSE(writer, SSEMessage{
			ID:   strconv.FormatInt(event.Sequence, 10),
			Name: event.Type,
			Data: SessionEventPayloadFromEvent(event, payloadInfo),
		}); err != nil {
			return afterSequence, err
		}
	}
	return afterSequence, nil
}

func (h *BaseHandlers) writeSessionStoppedEvent(writer FlushWriter, latest *session.Info) error {
	if latest == nil || latest.State != session.StateStopped {
		return nil
	}

	workspaceID, workspacePath := h.streamWorkspaceFields(latest)
	return WriteSSE(writer, SSEMessage{
		Name: session.EventTypeSessionStopped,
		Data: contract.SessionEventPayload{
			SessionID:     latest.ID,
			Type:          session.EventTypeSessionStopped,
			WorkspaceID:   workspaceID,
			WorkspacePath: workspacePath,
			StopReason:    latest.StopReason,
			StopDetail:    latest.StopDetail,
			Failure:       SessionFailurePayloadFromStore(latest.Failure),
			Timestamp:     latest.UpdatedAt,
		},
	})
}

func (h *BaseHandlers) pollAndStreamSessionEvents(
	c *gin.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	pollQuery store.EventQuery,
	afterSequence int64,
) {
	ticker := time.NewTicker(h.PollInterval)
	defer ticker.Stop()
	keepAlive := time.NewTicker(sessionStreamKeepAliveInterval)
	defer keepAlive.Stop()

	currentInfo := info
	streamDone := h.StreamDoneChannel()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-streamDone:
			return
		case <-keepAlive.C:
			if !h.writeKeepAlive(writer) {
				return
			}
		case <-ticker.C:
			if sessionStreamStopped(c.Request.Context(), streamDone) {
				return
			}
			var done bool
			afterSequence, currentInfo, done = h.pollSessionStreamTick(
				c,
				writer,
				sessionID,
				currentInfo,
				pollQuery,
				afterSequence,
			)
			if done || sessionStreamStopped(c.Request.Context(), streamDone) {
				return
			}
		}
	}
}

func (h *BaseHandlers) pushAndStreamSessionEvents(
	c *gin.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
	pollQuery store.EventQuery,
	afterSequence int64,
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
					c.Request.Context(), sessionID, currentInfo, contract.SessionStreamFrameRaw, afterSequence,
				)
				subscription.cancelIfActive()
				h.pollAndStreamSessionEvents(c, writer, sessionID, currentInfo, pollQuery, afterSequence)
				return
			}
			if event.Sequence <= afterSequence {
				continue
			}
			if event.Sequence > afterSequence+1 {
				h.logSessionStreamSequenceGap(
					c.Request.Context(),
					sessionID,
					currentInfo,
					contract.SessionStreamFrameRaw,
					afterSequence,
					event.Sequence,
				)
				subscription.cancelIfActive()
				h.pollAndStreamSessionEvents(c, writer, sessionID, currentInfo, pollQuery, afterSequence)
				return
			}
			nextSequence, err := h.writeSessionEventBatch(writer, []store.SessionEvent{event}, currentInfo)
			if err != nil {
				return
			}
			if nextSequence > afterSequence {
				afterSequence = nextSequence
			}
			if event.Type == session.EventTypeSessionStopped {
				return
			}
		}
	}
}

func (h *BaseHandlers) streamWorkspaceFields(info *session.Info) (string, string) {
	if info == nil || !h.IncludeSessionWorkspaceInSSE {
		return "", ""
	}
	ref := workref.NewPath(info.WorkspaceID, info.Workspace)
	return ref.WorkspaceID, ref.WorkspacePath
}
