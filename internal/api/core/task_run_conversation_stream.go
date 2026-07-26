package core

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

const (
	taskRunConversationMessageEvent = "network.message"
	taskRunConversationUsageEvent   = "network.usage"
)

type taskRunConversationCursor struct {
	messageID string
	usage     *contract.NetworkUsageResponse
}

type taskRunConversationSnapshot struct {
	messages []store.NetworkConversationMessage
	usage    contract.NetworkUsageResponse
}

// StreamTaskRunConversation streams run-scoped coordination messages and usage changes over SSE.
func (h *BaseHandlers) StreamTaskRunConversation(c *gin.Context) {
	if h.NetworkStore == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: network store is unavailable"))
		return
	}
	if h.NetworkUsage == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: network usage store is unavailable"))
		return
	}
	run, conversation, ok := h.requireTaskRunConversation(c)
	if !ok {
		return
	}

	cursor := taskRunConversationCursor{
		messageID: firstNonEmpty(
			strings.TrimSpace(c.Query("after")),
			strings.TrimSpace(c.GetHeader("Last-Event-ID")),
		),
	}
	ref := store.NetworkConversationRef{
		WorkspaceID: conversation.WorkspaceID,
		Channel:     conversation.Channel,
		Surface:     conversation.Surface,
		ThreadID:    conversation.ThreadID,
	}
	snapshot, err := h.loadTaskRunConversationSnapshot(c.Request.Context(), run, ref, cursor)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNetworkCursorInvalid) {
			status = http.StatusUnprocessableEntity
		}
		h.respondError(c, status, err)
		return
	}
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if err := WriteSSEComment(writer, "task run conversation stream ready"); err != nil {
		h.logSSEWriteFailure("ready", err)
		return
	}

	if !h.writeTaskRunConversationSnapshot(writer, snapshot, &cursor) {
		return
	}

	interval := h.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.StreamDoneChannel():
			return
		case <-ticker.C:
			if !h.emitTaskRunConversation(c.Request.Context(), writer, run, ref, &cursor) {
				return
			}
		}
	}
}

func (h *BaseHandlers) requireTaskRunConversation(
	c *gin.Context,
) (taskpkg.Run, *contract.TaskRunConversationRefPayload, bool) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return taskpkg.Run{}, nil, false
	}
	runID, err := requiredPathID(c.Param("id"), "run id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return taskpkg.Run{}, nil, false
	}
	actor, err := h.taskActorContext(c, taskActionStream)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return taskpkg.Run{}, nil, false
	}
	view, err := manager.RunDetail(c.Request.Context(), runID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return taskpkg.Run{}, nil, false
	}
	if view == nil {
		h.respondError(c, http.StatusInternalServerError, errors.New("api: task run detail is required"))
		return taskpkg.Run{}, nil, false
	}
	conversation, err := taskRunConversationRefPayload(view.Run)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return taskpkg.Run{}, nil, false
	}
	if conversation == nil {
		h.respondError(c, http.StatusConflict, errors.New("api: local task run has no coordination conversation"))
		return taskpkg.Run{}, nil, false
	}
	return view.Run, conversation, true
}

func (h *BaseHandlers) emitTaskRunConversation(
	ctx context.Context,
	writer FlushWriter,
	run taskpkg.Run,
	ref store.NetworkConversationRef,
	cursor *taskRunConversationCursor,
) bool {
	snapshot, err := h.loadTaskRunConversationSnapshot(ctx, run, ref, *cursor)
	if err != nil {
		h.logSSEWriteFailure(taskRunConversationMessageEvent, err)
		return false
	}
	return h.writeTaskRunConversationSnapshot(writer, snapshot, cursor)
}

func (h *BaseHandlers) loadTaskRunConversationSnapshot(
	ctx context.Context,
	run taskpkg.Run,
	ref store.NetworkConversationRef,
	cursor taskRunConversationCursor,
) (taskRunConversationSnapshot, error) {
	messages, err := h.NetworkStore.ListConversationMessages(ctx, ref, store.NetworkConversationMessageQuery{
		AfterMessageID: cursor.messageID,
		Limit:          200,
	})
	if err != nil {
		return taskRunConversationSnapshot{}, err
	}
	report, err := h.NetworkUsage.GetNetworkUsage(ctx, store.NetworkUsageQuery{
		WorkspaceID: ref.WorkspaceID,
		RunID:       strings.TrimSpace(run.ID),
	})
	if err != nil {
		return taskRunConversationSnapshot{}, err
	}
	usage, err := NetworkUsageResponseFromReport(ref.WorkspaceID, report)
	if err != nil {
		return taskRunConversationSnapshot{}, err
	}
	return taskRunConversationSnapshot{messages: messages, usage: usage}, nil
}

func (h *BaseHandlers) writeTaskRunConversationSnapshot(
	writer FlushWriter,
	snapshot taskRunConversationSnapshot,
	cursor *taskRunConversationCursor,
) bool {
	for _, message := range snapshot.messages {
		payload := NetworkConversationMessagePayloadFromStore(message)
		if err := WriteSSE(writer, SSEMessage{
			ID:   payload.MessageID,
			Name: taskRunConversationMessageEvent,
			Data: contract.TaskRunConversationStreamPayload{Message: &payload},
		}); err != nil {
			h.logSSEWriteFailure(taskRunConversationMessageEvent, err)
			return false
		}
		cursor.messageID = payload.MessageID
	}
	if cursor.usage == nil || !reflect.DeepEqual(*cursor.usage, snapshot.usage) {
		if err := WriteSSE(writer, SSEMessage{
			Name: taskRunConversationUsageEvent,
			Data: contract.TaskRunConversationStreamPayload{Usage: &snapshot.usage},
		}); err != nil {
			h.logSSEWriteFailure(taskRunConversationUsageEvent, err)
			return false
		}
		usage := snapshot.usage
		cursor.usage = &usage
	}
	return true
}
