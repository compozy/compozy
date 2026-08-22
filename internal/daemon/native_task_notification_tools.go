package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	nativeTaskNotificationToolsDeletedKey = "deleted"
)

type taskNotificationSubscribeInput struct {
	TaskID           string `json:"task_id"`
	SubscriptionID   string `json:"subscription_id,omitempty"`
	BridgeInstanceID string `json:"bridge_instance_id"`
	Scope            string `json:"scope,omitempty"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	PeerID           string `json:"peer_id,omitempty"`
	ThreadID         string `json:"thread_id,omitempty"`
	GroupID          string `json:"group_id,omitempty"`
	DeliveryMode     string `json:"delivery_mode,omitempty"`
}

type taskNotificationListInput struct {
	TaskID           string `json:"task_id"`
	BridgeInstanceID string `json:"bridge_instance_id,omitempty"`
	Scope            string `json:"scope,omitempty"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	Limit            int    `json:"limit,omitempty"`
}

type taskNotificationSubscriptionInput struct {
	TaskID         string `json:"task_id"`
	SubscriptionID string `json:"subscription_id"`
}

type nativeTaskNotificationCursorReader interface {
	GetCursor(ctx context.Context, key notifications.CursorKey) (notifications.Cursor, error)
}

func (n *daemonNativeTools) taskNotificationSubscribe(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskNotificationSubscribeInput
	if err := decodeNativeTaskNotificationInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	taskRecord, actor, err := n.authorizedTaskNotificationTask(ctx, scope, req.ToolID, input.TaskID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	subscription, err := nativeTaskNotificationSubscriptionFromInput(taskRecord, actor.Actor, time.Now().UTC(), input)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	readScope := store.ReadScope{ProfileID: taskRecord.ProfileID}
	instance, err := n.deps.Bridges.GetInstanceScoped(ctx, readScope, subscription.BridgeInstanceID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	if err := nativeValidateTaskNotificationInstanceScope(taskRecord, instance); err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	if err := n.deps.Bridges.PutBridgeTaskSubscription(ctx, subscription); err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	stored, err := n.deps.Bridges.GetBridgeTaskSubscription(ctx, readScope, subscription.SubscriptionID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	payload, err := n.taskNotificationPayloadBestEffort(ctx, stored)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	return structuredResult(
		contract.TaskBridgeNotificationSubscriptionResponse{Subscription: payload},
		fmt.Sprintf("subscribed %s", payload.SubscriptionID),
	)
}

func (n *daemonNativeTools) taskNotificationList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskNotificationListInput
	if err := decodeNativeTaskNotificationInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	taskRecord, _, err := n.authorizedTaskNotificationTask(ctx, scope, req.ToolID, input.TaskID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query, err := nativeTaskNotificationQuery(req.ToolID, taskRecord.ProfileID, taskRecord.ID, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	subscriptions, err := n.deps.Bridges.ListBridgeTaskSubscriptions(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	payloads, err := n.taskNotificationPayloads(ctx, subscriptions)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	return structuredResult(
		contract.TaskBridgeNotificationSubscriptionsResponse{Subscriptions: payloads},
		fmt.Sprintf("%d task notification subscriptions", len(payloads)),
	)
}

func (n *daemonNativeTools) taskNotificationShow(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskNotificationSubscriptionInput
	if err := decodeNativeTaskNotificationInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	taskRecord, _, err := n.authorizedTaskNotificationTask(ctx, scope, req.ToolID, input.TaskID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	subscription, err := n.taskNotificationSubscriptionByID(ctx, req.ToolID, taskRecord, input.SubscriptionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload, err := n.taskNotificationPayload(ctx, subscription)
	if err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	return structuredResult(
		contract.TaskBridgeNotificationSubscriptionResponse{Subscription: payload},
		payload.SubscriptionID,
	)
}

func (n *daemonNativeTools) taskNotificationDelete(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskNotificationSubscriptionInput
	if err := decodeNativeTaskNotificationInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	taskRecord, _, err := n.authorizedTaskNotificationTask(ctx, scope, req.ToolID, input.TaskID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	subscription, err := n.taskNotificationSubscriptionByID(ctx, req.ToolID, taskRecord, input.SubscriptionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := n.deps.Bridges.DeleteBridgeTaskSubscription(ctx, subscription.SubscriptionID); err != nil {
		return toolspkg.ToolResult{}, nativeTaskNotificationToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{nativeTaskNotificationToolsDeletedKey: true, "subscription_id": subscription.SubscriptionID},
		fmt.Sprintf("deleted %s", subscription.SubscriptionID),
	)
}

func (n *daemonNativeTools) authorizedTaskNotificationTask(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	taskID string,
) (taskpkg.Task, taskpkg.ActorContext, error) {
	validatedTaskID, err := requiredNativeOpaqueIdentity(id, "task_id", taskID)
	if err != nil {
		return taskpkg.Task{}, taskpkg.ActorContext{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return taskpkg.Task{}, taskpkg.ActorContext{}, err
	}
	view, err := n.deps.Tasks.GetTask(ctx, validatedTaskID, actor)
	if err != nil {
		return taskpkg.Task{}, taskpkg.ActorContext{}, nativeTaskNotificationToolError(id, err)
	}
	if view == nil {
		return taskpkg.Task{}, taskpkg.ActorContext{}, nativeTaskNotificationToolError(
			id,
			fmt.Errorf("%w: %s", taskpkg.ErrTaskNotFound, validatedTaskID),
		)
	}
	return view.Task, actor, nil
}

func nativeTaskNotificationSubscriptionFromInput(
	taskRecord taskpkg.Task,
	actor taskpkg.ActorIdentity,
	now time.Time,
	input taskNotificationSubscribeInput,
) (bridgepkg.BridgeTaskSubscription, error) {
	taskScope := bridgepkg.Scope(taskRecord.Scope.Normalize())
	taskWorkspaceID := taskRecord.WorkspaceID
	requestScope := bridgepkg.Scope(input.Scope).Normalize()
	switch {
	case requestScope != "" && requestScope != taskScope:
		return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
			"%w: task bridge notification scope must match task scope %q",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
			taskScope,
		)
	case requestScope == bridgepkg.ScopeWorkspace && input.WorkspaceID != taskWorkspaceID:
		return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
			"%w: task bridge notification workspace must match task workspace %q",
			bridgepkg.ErrInvalidBridgeTaskSubscription,
			taskWorkspaceID,
		)
	}
	subscriptionID := input.SubscriptionID
	if subscriptionID == "" {
		generatedID, err := store.NewID("bts")
		if err != nil {
			return bridgepkg.BridgeTaskSubscription{}, fmt.Errorf(
				"daemon: generate bridge task subscription id: %w",
				err,
			)
		}
		subscriptionID = generatedID
	}
	subscription := bridgepkg.BridgeTaskSubscription{
		SubscriptionID:   subscriptionID,
		ProfileID:        taskRecord.ProfileID,
		TaskID:           taskRecord.ID,
		BridgeInstanceID: input.BridgeInstanceID,
		Scope:            taskScope,
		WorkspaceID:      taskWorkspaceID,
		PeerID:           input.PeerID,
		ThreadID:         input.ThreadID,
		GroupID:          input.GroupID,
		DeliveryMode:     bridgepkg.DeliveryMode(input.DeliveryMode),
		CreatedBy:        actor,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := subscription.Validate(); err != nil {
		return bridgepkg.BridgeTaskSubscription{}, err
	}
	return subscription.Normalize(), nil
}

func nativeValidateTaskNotificationInstanceScope(
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

func nativeTaskNotificationQuery(
	id toolspkg.ToolID,
	profileID string,
	taskID string,
	input taskNotificationListInput,
) (bridgepkg.BridgeTaskSubscriptionQuery, error) {
	query := bridgepkg.BridgeTaskSubscriptionQuery{
		ReadScope:        store.ReadScope{ProfileID: profileID},
		TaskID:           taskID,
		BridgeInstanceID: input.BridgeInstanceID,
		Scope:            bridgepkg.Scope(input.Scope),
		WorkspaceID:      input.WorkspaceID,
		Limit:            input.Limit,
	}
	if err := query.Validate(); err != nil {
		return bridgepkg.BridgeTaskSubscriptionQuery{}, nativeTaskNotificationToolError(id, err)
	}
	return query.Normalize(), nil
}

func (n *daemonNativeTools) taskNotificationSubscriptionByID(
	ctx context.Context,
	id toolspkg.ToolID,
	taskRecord taskpkg.Task,
	subscriptionID string,
) (bridgepkg.BridgeTaskSubscription, error) {
	validatedID, err := requiredNativeOpaqueIdentity(id, "subscription_id", subscriptionID)
	if err != nil {
		return bridgepkg.BridgeTaskSubscription{}, err
	}
	subscription, err := n.deps.Bridges.GetBridgeTaskSubscription(
		ctx,
		store.ReadScope{ProfileID: taskRecord.ProfileID},
		validatedID,
	)
	if err != nil {
		return bridgepkg.BridgeTaskSubscription{}, nativeTaskNotificationToolError(id, err)
	}
	if subscription.TaskID != taskRecord.ID {
		return bridgepkg.BridgeTaskSubscription{}, nativeTaskNotificationToolError(
			id,
			bridgepkg.ErrBridgeTaskSubscriptionNotFound,
		)
	}
	return subscription, nil
}

func decodeNativeTaskNotificationInput(req toolspkg.CallRequest, dst any) error {
	if !utf8.Valid(req.Input) {
		return nativeTaskNotificationToolError(
			req.ToolID,
			fmt.Errorf("%w: task notification input must be valid UTF-8", bridgepkg.ErrInvalidBridgeTaskSubscription),
		)
	}
	return decodeNativeInput(req, dst)
}

func requiredNativeOpaqueIdentity(id toolspkg.ToolID, field string, value string) (string, error) {
	if value == "" {
		return "", nativeRequiredInputError(id, field)
	}
	if !utf8.ValidString(value) {
		return "", nativeTaskNotificationToolError(
			id,
			fmt.Errorf("%w: %s must be valid UTF-8", bridgepkg.ErrInvalidBridgeTaskSubscription, field),
		)
	}
	return value, nil
}

func (n *daemonNativeTools) taskNotificationPayloads(
	ctx context.Context,
	subscriptions []bridgepkg.BridgeTaskSubscription,
) ([]contract.TaskBridgeNotificationSubscriptionPayload, error) {
	payloads := make([]contract.TaskBridgeNotificationSubscriptionPayload, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		payload, err := n.taskNotificationPayload(ctx, subscription)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

func (n *daemonNativeTools) taskNotificationPayloadBestEffort(
	ctx context.Context,
	subscription bridgepkg.BridgeTaskSubscription,
) (contract.TaskBridgeNotificationSubscriptionPayload, error) {
	payload, err := n.taskNotificationPayload(ctx, subscription)
	if err == nil {
		return payload, nil
	}
	if errors.Is(err, notifications.ErrInvalidCursor) {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, err
	}
	normalized := subscription.Normalize()
	slog.Default().Warn(
		"daemon: task notification cursor enrichment failed",
		"subscription_id",
		normalized.SubscriptionID,
		"error",
		err,
	)
	fallback, fallbackErr := core.TaskBridgeNotificationSubscriptionPayloadFromSubscription(normalized)
	if fallbackErr != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, errors.Join(err, fallbackErr)
	}
	return fallback, nil
}

func (n *daemonNativeTools) taskNotificationPayload(
	ctx context.Context,
	subscription bridgepkg.BridgeTaskSubscription,
) (contract.TaskBridgeNotificationSubscriptionPayload, error) {
	normalized := subscription.Normalize()
	payload, err := core.TaskBridgeNotificationSubscriptionPayloadFromSubscription(normalized)
	if err != nil {
		return contract.TaskBridgeNotificationSubscriptionPayload{}, err
	}
	reader, ok := n.deps.Bridges.(nativeTaskNotificationCursorReader)
	if !ok {
		return payload, nil
	}
	cursor, err := reader.GetCursor(ctx, normalized.CursorKey())
	if err != nil {
		if errors.Is(err, notifications.ErrCursorNotFound) {
			return payload, nil
		}
		return contract.TaskBridgeNotificationSubscriptionPayload{}, fmt.Errorf(
			"daemon: load task bridge notification cursor for subscription %q: %w",
			normalized.SubscriptionID,
			err,
		)
	}
	return core.TaskBridgeNotificationSubscriptionPayloadFromSubscriptionAndCursor(normalized, cursor)
}

func nativeTaskNotificationToolError(id toolspkg.ToolID, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, bridgepkg.ErrInvalidBridgeTaskSubscription),
		errors.Is(err, taskpkg.ErrValidation),
		errors.Is(err, taskpkg.ErrInvalidScopeBinding),
		errors.Is(err, taskpkg.ErrImmutableField):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, taskpkg.ErrTaskNotFound),
		errors.Is(err, bridgepkg.ErrBridgeTaskSubscriptionNotFound),
		errors.Is(err, bridgepkg.ErrBridgeInstanceNotFound),
		errors.Is(err, os.ErrNotExist):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
			toolspkg.ReasonToolUnknown,
		)
	case errors.Is(err, taskpkg.ErrPermissionDenied),
		errors.Is(err, bridgepkg.ErrBridgeInstanceReadOnly):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonPolicyDenied,
		)
	case errors.Is(err, bridgepkg.ErrBridgeInstanceUnavailable):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
			toolspkg.ReasonBackendUnhealthy,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			taskpkg.RedactClaimTokens(err.Error()),
			err,
			toolspkg.ReasonBackendUnhealthy,
		)
	}
}
