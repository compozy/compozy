package core

import (
	"context"

	"errors"
	"fmt"

	"net/http"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/gateway"

	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
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
	readScope := store.ReadScope{ProfileID: taskRecord.ProfileID}
	instance, err := bridges.GetInstanceScoped(c.Request.Context(), readScope, subscription.BridgeInstanceID)
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
	stored, err := bridges.GetBridgeTaskSubscription(
		c.Request.Context(),
		readScope,
		subscription.SubscriptionID,
	)
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
	query, err := parseTaskBridgeNotificationSubscriptionQuery(c, taskRecord.ProfileID, taskRecord.ID)
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
	subscription, ok := h.taskBridgeNotificationSubscriptionByPath(
		c, bridges, taskRecord.ProfileID, taskRecord.ID,
	)
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
	subscription, ok := h.taskBridgeNotificationSubscriptionByPath(
		c, bridges, taskRecord.ProfileID, taskRecord.ID,
	)
	if !ok {
		return
	}
	if err := bridges.DeleteBridgeTaskSubscription(c.Request.Context(), subscription.SubscriptionID); err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
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
		if errors.Is(err, gateway.ErrIngressForbidden) {
			h.respondGatewayError(c, err)
			return
		}
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
	return h.transitionBridgeInstance(
		c,
		"bridges.enable",
		func(bridges BridgeService) (*bridgepkg.BridgeInstance, error) {
			return bridges.StartInstance(c.Request.Context(), c.Param("id"))
		},
	)
}

func (h *BaseHandlers) disableBridge(c *gin.Context) (*contract.BridgeResponse, error) {
	return h.transitionBridgeInstance(
		c,
		"bridges.disable",
		func(bridges BridgeService) (*bridgepkg.BridgeInstance, error) {
			return bridges.StopInstance(c.Request.Context(), c.Param("id"))
		},
	)
}

func (h *BaseHandlers) restartBridge(c *gin.Context) (*contract.BridgeResponse, error) {
	return h.transitionBridgeInstance(
		c,
		"bridges.restart",
		func(bridges BridgeService) (*bridgepkg.BridgeInstance, error) {
			return bridges.RestartInstance(c.Request.Context(), c.Param("id"))
		},
	)
}

func (h *BaseHandlers) transitionBridgeInstance(
	c *gin.Context,
	action string,
	transition func(BridgeService) (*bridgepkg.BridgeInstance, error),
) (*contract.BridgeResponse, error) {
	projectionCtx, err := h.gatewayIngressReadContext(c, action)
	if err != nil {
		return nil, gatewayIngressForbidden(err)
	}
	if err := h.authorizeBridgeIngressProjection(projectionCtx, c.Param("id")); err != nil {
		return nil, err
	}
	bridges, ok := h.bridgeService()
	if !ok {
		return nil, errBridgeServiceUnavailable
	}
	instance, err := transition(bridges)
	if err != nil {
		return nil, err
	}
	return h.bridgeResponse(projectionCtx, *instance)
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
	payload, err := TaskBridgeNotificationSubscriptionPayloadFromSubscription(normalized)
	if err != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, err
	}
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
	return TaskBridgeNotificationSubscriptionPayloadFromSubscriptionAndCursor(normalized, cursor)
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
	taskID, err := requiredOpaqueTaskBridgeNotificationPathID(c.Param("id"), "task id")
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
	taskWorkspaceID := taskRecord.WorkspaceID
	requestScope := req.Scope.Normalize()
	switch {
	case requestScope != "" && requestScope != taskScope:
		return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
			"%w: task bridge notification scope must match task scope %q",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
			taskScope,
		)
	case requestScope == bridgepkg.ScopeWorkspace && req.WorkspaceID != taskWorkspaceID:
		return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
			"%w: task bridge notification workspace must match task workspace %q",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
			taskWorkspaceID,
		)
	}
	subscriptionID := req.SubscriptionID
	if subscriptionID == "" {
		generatedID, err := store.NewID("bts")
		if err != nil {
			return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
				"api: generate bridge task subscription id: %w",
				err,
			)
		}
		subscriptionID = generatedID
	}
	subscription := bridgepkg.BridgeTaskSubscription{
		SubscriptionID:   subscriptionID,
		ProfileID:        taskRecord.ProfileID,
		TaskID:           taskRecord.ID,
		BridgeInstanceID: req.BridgeInstanceID,
		Scope:            taskScope,
		WorkspaceID:      taskWorkspaceID,
		PeerID:           req.PeerID,
		ThreadID:         req.ThreadID,
		GroupID:          req.GroupID,
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
	taskWorkspaceID := taskRecord.WorkspaceID
	instanceScope := instance.Scope.Normalize()
	instanceWorkspaceID := instance.WorkspaceID
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
