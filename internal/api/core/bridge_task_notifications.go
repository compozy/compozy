package core

import (
	"context"

	"errors"
	"fmt"

	"net/http"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/notifications"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

// CreateTaskBridgeNotificationSubscription persists one task-scoped terminal
// bridge notification target.
func (h *BaseHandlers) CreateTaskBridgeNotificationSubscription(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	taskRecord, actor, ok := h.authorizeTaskBridgeNotification(c, manager, taskActionCreateBridgeSub)
	if !ok {
		return
	}

	var req contract.CreateTaskBridgeNotificationSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode task bridge notification subscription request: %w", h.transportName(), err),
		)
		return
	}

	subscription, err := taskBridgeNotificationSubscriptionFromRequest(taskRecord, actor.Actor, h.Now(), &req)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	instance, err := bridges.GetInstance(c.Request.Context(), subscription.BridgeInstanceID)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	if err := validateTaskBridgeNotificationInstanceScope(taskRecord, instance); err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	if err := bridges.PutBridgeTaskSubscription(c.Request.Context(), subscription); err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	stored, err := bridges.GetBridgeTaskSubscription(c.Request.Context(), subscription.SubscriptionID)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	payload, err := h.taskBridgeNotificationSubscriptionPayload(c.Request.Context(), stored)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, contract.TaskBridgeNotificationSubscriptionResponse{
		Subscription: payload,
	})
}

// ListTaskBridgeNotificationSubscriptions returns task-scoped terminal bridge
// notification targets.
func (h *BaseHandlers) ListTaskBridgeNotificationSubscriptions(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	taskRecord, _, ok := h.authorizeTaskBridgeNotification(c, manager, taskActionListBridgeSubs)
	if !ok {
		return
	}
	query, err := parseTaskBridgeNotificationSubscriptionQuery(c, strings.TrimSpace(taskRecord.ID))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	subscriptions, err := bridges.ListBridgeTaskSubscriptions(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	payloads, err := h.taskBridgeNotificationSubscriptionPayloads(c.Request.Context(), subscriptions)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskBridgeNotificationSubscriptionsResponse{
		Subscriptions: payloads,
	})
}

// GetTaskBridgeNotificationSubscription returns one task-scoped terminal
// bridge notification target.
func (h *BaseHandlers) GetTaskBridgeNotificationSubscription(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	taskRecord, _, ok := h.authorizeTaskBridgeNotification(c, manager, taskActionGetBridgeSub)
	if !ok {
		return
	}
	subscription, ok := h.taskBridgeNotificationSubscriptionByPath(c, bridges, strings.TrimSpace(taskRecord.ID))
	if !ok {
		return
	}
	payload, err := h.taskBridgeNotificationSubscriptionPayload(c.Request.Context(), subscription)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, contract.TaskBridgeNotificationSubscriptionResponse{
		Subscription: payload,
	})
}

// DeleteTaskBridgeNotificationSubscription removes one task-scoped terminal
// bridge notification target.
func (h *BaseHandlers) DeleteTaskBridgeNotificationSubscription(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	taskRecord, _, ok := h.authorizeTaskBridgeNotification(c, manager, taskActionDeleteBridgeSub)
	if !ok {
		return
	}
	subscription, ok := h.taskBridgeNotificationSubscriptionByPath(c, bridges, strings.TrimSpace(taskRecord.ID))
	if !ok {
		return
	}
	if err := bridges.DeleteBridgeTaskSubscription(c.Request.Context(), subscription.SubscriptionID); err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) taskBridgeNotificationSubscriptionByPath(
	c *gin.Context,
	bridges BridgeService,
	taskID string,
) (bridgepkg.BridgeTaskSubscription, bool) {
	subscriptionID, err := requiredPathID(c.Param("subscription_id"), "bridge task subscription id")
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return bridgepkg.BridgeTaskSubscription{}, false
	}
	subscription, err := bridges.GetBridgeTaskSubscription(c.Request.Context(), subscriptionID)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return bridgepkg.BridgeTaskSubscription{}, false
	}
	if subscription.TaskID != taskID {
		h.respondError(
			c,
			StatusForBridgeError(bridgepkg.ErrBridgeTaskSubscriptionNotFound),
			bridgepkg.ErrBridgeTaskSubscriptionNotFound,
		)
		return bridgepkg.BridgeTaskSubscription{}, false
	}
	return subscription, true
}

func (h *BaseHandlers) transitionBridge(
	c *gin.Context,
	fn func(*BaseHandlers, *gin.Context) (*contract.BridgeResponse, error),
) {
	if h == nil {
		RespondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable, false)
		return
	}
	resp, err := fn(h, c)
	if err != nil {
		if errors.Is(err, errBridgeServiceUnavailable) {
			h.respondError(c, http.StatusServiceUnavailable, err)
			return
		}
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, *resp)
}

func (h *BaseHandlers) enableBridge(c *gin.Context) (*contract.BridgeResponse, error) {
	bridges, ok := h.bridgeService()
	if !ok {
		return nil, errBridgeServiceUnavailable
	}
	instance, err := bridges.StartInstance(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		return nil, err
	}
	return h.bridgeResponse(c.Request.Context(), *instance)
}

func (h *BaseHandlers) disableBridge(c *gin.Context) (*contract.BridgeResponse, error) {
	bridges, ok := h.bridgeService()
	if !ok {
		return nil, errBridgeServiceUnavailable
	}
	instance, err := bridges.StopInstance(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		return nil, err
	}
	return h.bridgeResponse(c.Request.Context(), *instance)
}

func (h *BaseHandlers) restartBridge(c *gin.Context) (*contract.BridgeResponse, error) {
	bridges, ok := h.bridgeService()
	if !ok {
		return nil, errBridgeServiceUnavailable
	}
	instance, err := bridges.RestartInstance(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		return nil, err
	}
	return h.bridgeResponse(c.Request.Context(), *instance)
}

func (h *BaseHandlers) bridgeService() (BridgeService, bool) {
	if h == nil || h.Bridges == nil {
		return nil, false
	}
	return h.Bridges, true
}

func (h *BaseHandlers) taskBridgeNotificationSubscriptionPayloads(
	ctx context.Context,
	subscriptions []bridgepkg.BridgeTaskSubscription,
) ([]contract.TaskBridgeNotificationSubscriptionPayload, error) {
	payloads := make([]contract.TaskBridgeNotificationSubscriptionPayload, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		payload, err := h.taskBridgeNotificationSubscriptionPayload(ctx, subscription)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

func (h *BaseHandlers) taskBridgeNotificationSubscriptionPayload(
	ctx context.Context,
	subscription bridgepkg.BridgeTaskSubscription,
) (contract.TaskBridgeNotificationSubscriptionPayload, error) {
	normalized := subscription.Normalize()
	payload := TaskBridgeNotificationSubscriptionPayloadFromSubscription(normalized)
	reader, ok := h.taskNotificationCursorReader()
	if !ok {
		return payload, nil
	}
	cursor, err := reader.GetCursor(ctx, normalized.CursorKey())
	if err != nil {
		if errors.Is(err, notifications.ErrCursorNotFound) {
			return payload, nil
		}
		return contract.TaskBridgeNotificationSubscriptionPayload{}, fmt.Errorf(
			"api: load task bridge notification cursor for subscription %q: %w",
			normalized.SubscriptionID,
			err,
		)
	}
	return TaskBridgeNotificationSubscriptionPayloadFromSubscriptionAndCursor(normalized, cursor), nil
}

func (h *BaseHandlers) taskNotificationCursorReader() (taskNotificationCursorReader, bool) {
	if h == nil || h.Bridges == nil {
		return nil, false
	}
	reader, ok := h.Bridges.(taskNotificationCursorReader)
	return reader, ok
}

func (h *BaseHandlers) authorizeTaskBridgeNotification(
	c *gin.Context,
	manager TaskService,
	action string,
) (taskpkg.Task, taskpkg.ActorContext, bool) {
	taskID, err := requiredPathID(c.Param("id"), "task id")
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return taskpkg.Task{}, taskpkg.ActorContext{}, false
	}
	actor, err := h.taskActorContext(c, action)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return taskpkg.Task{}, taskpkg.ActorContext{}, false
	}
	view, err := manager.GetTask(c.Request.Context(), taskID, actor)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return taskpkg.Task{}, taskpkg.ActorContext{}, false
	}
	return view.Task, actor, true
}

func taskBridgeNotificationSubscriptionFromRequest(
	taskRecord taskpkg.Task,
	actor taskpkg.ActorIdentity,
	now time.Time,
	req *contract.CreateTaskBridgeNotificationSubscriptionRequest,
) (bridgepkg.BridgeTaskSubscription, error) {
	if req == nil {
		return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
			"%w: request is required",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
		)
	}
	taskScope := bridgepkg.Scope(taskRecord.Scope.Normalize())
	taskWorkspaceID := strings.TrimSpace(taskRecord.WorkspaceID)
	requestScope := req.Scope.Normalize()
	switch {
	case requestScope != "" && requestScope != taskScope:
		return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
			"%w: task bridge notification scope must match task scope %q",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
			taskScope,
		)
	case requestScope == bridgepkg.ScopeWorkspace && strings.TrimSpace(req.WorkspaceID) != taskWorkspaceID:
		return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
			"%w: task bridge notification workspace must match task workspace %q",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
			taskWorkspaceID,
		)
	}
	subscriptionID := strings.TrimSpace(req.SubscriptionID)
	if subscriptionID == "" {
		subscriptionID = store.NewID("bts")
	}
	subscription := bridgepkg.BridgeTaskSubscription{
		SubscriptionID:   subscriptionID,
		TaskID:           strings.TrimSpace(taskRecord.ID),
		BridgeInstanceID: strings.TrimSpace(req.BridgeInstanceID),
		Scope:            taskScope,
		WorkspaceID:      taskWorkspaceID,
		PeerID:           strings.TrimSpace(req.PeerID),
		ThreadID:         strings.TrimSpace(req.ThreadID),
		GroupID:          strings.TrimSpace(req.GroupID),
		DeliveryMode:     req.DeliveryMode,
		CreatedBy:        actor,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := subscription.Validate(); err != nil {
		return bridgepkg.BridgeTaskSubscription{}, err
	}
	return subscription.Normalize(), nil
}

func validateTaskBridgeNotificationInstanceScope(
	taskRecord taskpkg.Task,
	instance *bridgepkg.BridgeInstance,
) error {
	if instance == nil {
		return bridgepkg.ErrBridgeInstanceNotFound
	}
	taskScope := bridgepkg.Scope(taskRecord.Scope.Normalize())
	taskWorkspaceID := strings.TrimSpace(taskRecord.WorkspaceID)
	instanceScope := instance.Scope.Normalize()
	instanceWorkspaceID := strings.TrimSpace(instance.WorkspaceID)
	if taskScope != instanceScope {
		return fmt.Errorf(
			"%w: bridge instance scope %q does not match task scope %q",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
			instanceScope,
			taskScope,
		)
	}
	if taskScope == bridgepkg.ScopeWorkspace && taskWorkspaceID != instanceWorkspaceID {
		return fmt.Errorf(
			"%w: bridge instance workspace %q does not match task workspace %q",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
			instanceWorkspaceID,
			taskWorkspaceID,
		)
	}
	return nil
}

func parseTaskBridgeNotificationSubscriptionQuery(
	c *gin.Context,
	taskID string,
) (bridgepkg.BridgeTaskSubscriptionQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return bridgepkg.BridgeTaskSubscriptionQuery{}, err
	}
	query := bridgepkg.BridgeTaskSubscriptionQuery{
		TaskID:           strings.TrimSpace(taskID),
		BridgeInstanceID: strings.TrimSpace(c.Query("bridge_instance_id")),
		Scope:            bridgepkg.Scope(c.Query("scope")),
		WorkspaceID:      strings.TrimSpace(c.Query("workspace_id")),
		Limit:            limit,
	}
	if query.Scope != "" {
		if err := query.Scope.Validate(); err != nil {
			return bridgepkg.BridgeTaskSubscriptionQuery{}, err
		}
	}
	return query.Normalize(), nil
}
