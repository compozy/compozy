package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

// NetworkThread returns one public-thread summary.
func (h *BaseHandlers) NetworkThread(c *gin.Context) {
	if _, ok := h.requireNetworkReadDependencies(c); !ok {
		return
	}
	channel, err := normalizeNetworkChannel(c.Param("channel"))
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	threadID := strings.TrimSpace(c.Param("thread_id"))
	if err := network.ValidateConversationID(threadID, "thread_id"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}

	thread, err := h.NetworkStore.GetThread(c.Request.Context(), scope.NetworkChannelRef(channel), threadID)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	sessionCosts, err := h.networkThreadSessionCosts(c.Request.Context(), scope.NetworkWorkspaceID(), channel, threadID)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	taskLinks, err := h.networkThreadTaskLinks(c.Request.Context(), scope.NetworkWorkspaceID(), channel, threadID)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.NetworkThreadResponse{
		Thread:       NetworkThreadSummaryPayloadFromStore(thread),
		SessionCosts: NetworkThreadSessionCostPayloadsFromStore(sessionCosts),
		TaskLinks:    NetworkTaskThreadOriginPayloadsFromStore(taskLinks),
	})
}

// PromoteNetworkThreadTask creates a durable task from a public-thread message.
func (h *BaseHandlers) PromoteNetworkThreadTask(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	if _, ok := h.requireNetworkReadDependencies(c); !ok {
		return
	}
	channel, err := normalizeNetworkChannel(c.Param("channel"))
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	threadID := strings.TrimSpace(c.Param("thread_id"))
	if err := network.ValidateConversationID(threadID, "thread_id"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(err))
		return
	}
	var req contract.PromoteNetworkThreadTaskRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode promote network thread task request: %w", h.transportName(), err),
		)
		return
	}
	originMessageID := strings.TrimSpace(req.OriginMessageID)
	if originMessageID == "" {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(errors.New("origin_message_id is required")))
		return
	}
	source, err := h.networkThreadPromotionSource(
		c.Request.Context(),
		scope.NetworkWorkspaceID(),
		channel,
		threadID,
		originMessageID,
	)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	actor, err := h.taskActorContextForWorkspace(c, taskActionPromoteNetwork, scope.NetworkWorkspaceID())
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	taskRecord, err := h.createPromotedNetworkThreadTask(c.Request.Context(), manager, actor, source, req)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	origin, err := h.writePromotedNetworkThreadOrigin(c.Request.Context(), taskRecord.ID, source)
	if err != nil {
		if cleanupErr := manager.DeleteTask(c.Request.Context(), taskRecord.ID, actor); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("rollback promoted network task %q: %w", taskRecord.ID, cleanupErr))
		}
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.PromoteNetworkThreadTaskResponse{
		Task:   TaskPayloadFromTask(taskRecord),
		Origin: NetworkTaskThreadOriginPayloadFromStore(origin),
	})
}

type networkThreadPromotionSource struct {
	workspaceID      string
	channel          string
	threadID         string
	originMessageID  string
	digest           string
	sourceMessageIDs []string
	thread           store.NetworkThreadSummary
}

func (h *BaseHandlers) networkThreadPromotionSource(
	ctx context.Context,
	workspaceID string,
	channel string,
	threadID string,
	originMessageID string,
) (networkThreadPromotionSource, error) {
	ref := store.NetworkConversationRef{
		WorkspaceID: workspaceID,
		Channel:     channel,
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    threadID,
	}
	messages, err := h.NetworkStore.ListConversationMessages(
		ctx,
		ref,
		store.NetworkConversationMessageQuery{Limit: 200},
	)
	if err != nil {
		return networkThreadPromotionSource{}, err
	}
	digest, sourceMessageIDs, found := networkThreadPromotionDigest(messages, originMessageID)
	if !found {
		return networkThreadPromotionSource{}, NewNetworkValidationError(
			fmt.Errorf("origin_message_id %q is not part of thread %q", originMessageID, threadID),
		)
	}
	channelRef := store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: channel}
	thread, err := h.NetworkStore.GetThread(ctx, channelRef, threadID)
	if err != nil {
		return networkThreadPromotionSource{}, err
	}
	return networkThreadPromotionSource{
		workspaceID:      workspaceID,
		channel:          channel,
		threadID:         threadID,
		originMessageID:  originMessageID,
		digest:           digest,
		sourceMessageIDs: sourceMessageIDs,
		thread:           thread,
	}, nil
}

func (h *BaseHandlers) createPromotedNetworkThreadTask(
	ctx context.Context,
	manager TaskService,
	actor taskpkg.ActorContext,
	source networkThreadPromotionSource,
	req contract.PromoteNetworkThreadTaskRequest,
) (*taskpkg.Task, error) {
	spec := taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: source.workspaceID,
		Title:       promotedNetworkThreadTaskTitle(req, source),
		Description: firstNonEmpty(strings.TrimSpace(req.Description), source.digest),
		Priority:    taskpkg.Priority(strings.TrimSpace(req.Priority)).Normalize(),
		Metadata:    cloneRawMessage(req.Metadata),
	}
	if err := spec.Validate("promote_network_thread"); err != nil {
		return nil, err
	}
	return manager.CreateTask(ctx, spec, actor)
}

func promotedNetworkThreadTaskTitle(
	req contract.PromoteNetworkThreadTaskRequest,
	source networkThreadPromotionSource,
) string {
	return firstNonEmpty(
		strings.TrimSpace(req.Title),
		strings.TrimSpace(source.thread.Title),
		"Network thread "+source.threadID,
	)
}

func (h *BaseHandlers) writePromotedNetworkThreadOrigin(
	ctx context.Context,
	taskID string,
	source networkThreadPromotionSource,
) (store.NetworkTaskThreadOrigin, error) {
	now := h.nowUTC()
	origin := store.NetworkTaskThreadOrigin{
		TaskID:           taskID,
		WorkspaceID:      source.workspaceID,
		Channel:          source.channel,
		ThreadID:         source.threadID,
		OriginMessageID:  source.originMessageID,
		Digest:           source.digest,
		SourceMessageIDs: source.sourceMessageIDs,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := origin.Validate(); err != nil {
		return store.NetworkTaskThreadOrigin{}, err
	}
	return origin, h.NetworkStore.PutNetworkTaskThreadOrigin(ctx, origin)
}
