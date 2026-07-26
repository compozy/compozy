package testutil

import (
	"context"

	core "github.com/compozy/agh/internal/api/core"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/notifications"
)

type StubBridgeService struct {
	CreateInstanceFn func(
		context.Context,
		bridgepkg.CreateInstanceRequest,
	) (*bridgepkg.BridgeInstance, error)
	GetInstanceFn        func(context.Context, string) (*bridgepkg.BridgeInstance, error)
	ListInstancesFn      func(context.Context) ([]bridgepkg.BridgeInstance, error)
	ListCatalogRecordsFn func(
		context.Context,
		bridgepkg.BridgeCatalogQuery,
	) ([]bridgepkg.BridgeCatalogRecord, error)
	ListInstancesByIDsFn             func(context.Context, []string) ([]bridgepkg.BridgeInstance, error)
	ListProvidersFn                  func(context.Context) ([]bridgepkg.BridgeProvider, error)
	CountBridgeRoutesFn              func(context.Context, []string) (map[string]int, error)
	ListSecretBindingsFn             func(context.Context, string) ([]bridgepkg.BridgeSecretBinding, error)
	ListSecretBindingsForInstancesFn func(
		context.Context,
		[]string,
	) (map[string][]bridgepkg.BridgeSecretBinding, error)
	PutSecretBindingFn      func(context.Context, bridgepkg.BridgeSecretBinding, *string) error
	DeleteSecretBindingFn   func(context.Context, string, string) error
	UpdateInstanceFn        func(context.Context, bridgepkg.UpdateInstanceRequest) (*bridgepkg.BridgeInstance, error)
	UpdateInstanceStateFn   func(context.Context, bridgepkg.UpdateInstanceStateRequest) (*bridgepkg.BridgeInstance, error)
	BuildRoutingKeyFn       func(context.Context, bridgepkg.RoutingKey) (bridgepkg.RoutingKey, error)
	ResolveRouteFn          func(context.Context, bridgepkg.RoutingKey) (*bridgepkg.BridgeRoute, error)
	ResolveOrCreateRouteFn  func(context.Context, bridgepkg.BridgeRoute) (*bridgepkg.BridgeRoute, bool, error)
	UpsertRouteFn           func(context.Context, bridgepkg.BridgeRoute) (*bridgepkg.BridgeRoute, error)
	ListRoutesFn            func(context.Context, string) ([]bridgepkg.BridgeRoute, error)
	PutTaskSubscriptionFn   func(context.Context, bridgepkg.BridgeTaskSubscription) error
	GetTaskSubscriptionFn   func(context.Context, string) (bridgepkg.BridgeTaskSubscription, error)
	ListTaskSubscriptionsFn func(context.Context, bridgepkg.BridgeTaskSubscriptionQuery) (
		[]bridgepkg.BridgeTaskSubscription,
		error,
	)
	DeleteTaskSubscriptionFn func(context.Context, string) error
	GetCursorFn              func(context.Context, notifications.CursorKey) (notifications.Cursor, error)
	ResolveDeliveryTargetFn  func(
		context.Context,
		bridgepkg.ResolveDeliveryTargetRequest,
	) (*bridgepkg.DeliveryTarget, error)
	RefreshBridgeTargetsFn func(
		context.Context,
		string,
		[]bridgepkg.BridgeTargetSnapshot,
	) ([]bridgepkg.BridgeTarget, error)
	ListBridgeTargetsFn   func(context.Context, bridgepkg.BridgeTargetQuery) (bridgepkg.BridgeTargetsResult, error)
	ResolveBridgeTargetFn func(context.Context, string, string) (bridgepkg.ResolveBridgeTargetResult, error)
	StartInstanceFn       func(context.Context, string) (*bridgepkg.BridgeInstance, error)
	StopInstanceFn        func(context.Context, string) (*bridgepkg.BridgeInstance, error)
	RestartInstanceFn     func(context.Context, string) (*bridgepkg.BridgeInstance, error)
	CheckBridgeFn         func(
		context.Context,
		string,
		bridgepkg.BridgeCheckRequest,
	) (bridgepkg.BridgeCheckResponse, error)
	RegisterBridgeWebhookFn func(
		context.Context,
		string,
		bridgepkg.BridgeWebhookRegistrationRequest,
	) (bridgepkg.BridgeWebhookRegistrationResponse, error)
	DeliverBridgeFn func(
		context.Context,
		string,
		bridgepkg.DeliveryRequest,
	) (bridgepkg.DeliveryAck, error)
}

func (s StubBridgeService) CreateInstance(
	ctx context.Context,
	req bridgepkg.CreateInstanceRequest,
) (*bridgepkg.BridgeInstance, error) {
	if s.CreateInstanceFn != nil {
		return s.CreateInstanceFn(ctx, req)
	}
	return nil, bridgepkg.ErrBridgeInstanceNotFound
}

func (s StubBridgeService) GetInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	if s.GetInstanceFn != nil {
		return s.GetInstanceFn(ctx, id)
	}
	return nil, bridgepkg.ErrBridgeInstanceNotFound
}

func (s StubBridgeService) ListInstances(ctx context.Context) ([]bridgepkg.BridgeInstance, error) {
	if s.ListInstancesFn != nil {
		return s.ListInstancesFn(ctx)
	}
	return nil, nil
}

func (s StubBridgeService) ListCatalogRecords(
	ctx context.Context,
	query bridgepkg.BridgeCatalogQuery,
) ([]bridgepkg.BridgeCatalogRecord, error) {
	if s.ListCatalogRecordsFn != nil {
		return s.ListCatalogRecordsFn(ctx, query)
	}
	instances, err := s.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]bridgepkg.BridgeCatalogRecord, 0, len(instances))
	for _, instance := range instances {
		records = append(records, bridgepkg.BridgeCatalogRecordFromInstance(instance))
	}
	return records, nil
}

func (s StubBridgeService) ListInstancesByIDs(
	ctx context.Context,
	bridgeInstanceIDs []string,
) ([]bridgepkg.BridgeInstance, error) {
	if s.ListInstancesByIDsFn != nil {
		return s.ListInstancesByIDsFn(ctx, bridgeInstanceIDs)
	}
	instances, err := s.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]bridgepkg.BridgeInstance, len(instances))
	for _, instance := range instances {
		byID[instance.ID] = instance
	}
	selected := make([]bridgepkg.BridgeInstance, 0, len(bridgeInstanceIDs))
	for _, id := range bridgeInstanceIDs {
		if instance, ok := byID[id]; ok {
			selected = append(selected, instance)
		}
	}
	return selected, nil
}

func (s StubBridgeService) ListProviders(ctx context.Context) ([]bridgepkg.BridgeProvider, error) {
	if s.ListProvidersFn != nil {
		return s.ListProvidersFn(ctx)
	}
	return nil, nil
}

func (s StubBridgeService) CountBridgeRoutes(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	if s.CountBridgeRoutesFn != nil {
		return s.CountBridgeRoutesFn(ctx, bridgeInstanceIDs)
	}
	counts := make(map[string]int, len(bridgeInstanceIDs))
	for _, id := range bridgeInstanceIDs {
		counts[id] = 0
	}
	return counts, nil
}

func (s StubBridgeService) ListSecretBindings(
	ctx context.Context,
	bridgeInstanceID string,
) ([]bridgepkg.BridgeSecretBinding, error) {
	if s.ListSecretBindingsFn != nil {
		return s.ListSecretBindingsFn(ctx, bridgeInstanceID)
	}
	return nil, nil
}

func (s StubBridgeService) ListSecretBindingsForInstances(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string][]bridgepkg.BridgeSecretBinding, error) {
	if s.ListSecretBindingsForInstancesFn != nil {
		return s.ListSecretBindingsForInstancesFn(ctx, bridgeInstanceIDs)
	}
	bindings := make(map[string][]bridgepkg.BridgeSecretBinding, len(bridgeInstanceIDs))
	for _, id := range bridgeInstanceIDs {
		bindings[id] = []bridgepkg.BridgeSecretBinding{}
	}
	return bindings, nil
}

func (s StubBridgeService) PutSecretBinding(
	ctx context.Context,
	binding bridgepkg.BridgeSecretBinding,
	secretValue *string,
) error {
	if s.PutSecretBindingFn != nil {
		return s.PutSecretBindingFn(ctx, binding, secretValue)
	}
	return nil
}

func (s StubBridgeService) DeleteSecretBinding(ctx context.Context, bridgeInstanceID string, bindingName string) error {
	if s.DeleteSecretBindingFn != nil {
		return s.DeleteSecretBindingFn(ctx, bridgeInstanceID, bindingName)
	}
	return bridgepkg.ErrBridgeSecretBindingNotFound
}

func (s StubBridgeService) UpdateInstance(
	ctx context.Context,
	req bridgepkg.UpdateInstanceRequest,
) (*bridgepkg.BridgeInstance, error) {
	if s.UpdateInstanceFn != nil {
		return s.UpdateInstanceFn(ctx, req)
	}
	return nil, bridgepkg.ErrBridgeInstanceNotFound
}

func (s StubBridgeService) UpdateInstanceState(
	ctx context.Context,
	req bridgepkg.UpdateInstanceStateRequest,
) (*bridgepkg.BridgeInstance, error) {
	if s.UpdateInstanceStateFn != nil {
		return s.UpdateInstanceStateFn(ctx, req)
	}
	return nil, bridgepkg.ErrBridgeInstanceNotFound
}

func (s StubBridgeService) BuildRoutingKey(
	ctx context.Context,
	key bridgepkg.RoutingKey,
) (bridgepkg.RoutingKey, error) {
	if s.BuildRoutingKeyFn != nil {
		return s.BuildRoutingKeyFn(ctx, key)
	}
	return bridgepkg.RoutingKey{}, nil
}

func (s StubBridgeService) ResolveRoute(ctx context.Context, key bridgepkg.RoutingKey) (*bridgepkg.BridgeRoute, error) {
	if s.ResolveRouteFn != nil {
		return s.ResolveRouteFn(ctx, key)
	}
	return nil, bridgepkg.ErrBridgeRouteNotFound
}

func (s StubBridgeService) ResolveOrCreateRoute(
	ctx context.Context,
	route bridgepkg.BridgeRoute,
) (*bridgepkg.BridgeRoute, bool, error) {
	if s.ResolveOrCreateRouteFn != nil {
		return s.ResolveOrCreateRouteFn(ctx, route)
	}
	return nil, false, bridgepkg.ErrBridgeRouteNotFound
}

func (s StubBridgeService) UpsertRoute(
	ctx context.Context,
	route bridgepkg.BridgeRoute,
) (*bridgepkg.BridgeRoute, error) {
	if s.UpsertRouteFn != nil {
		return s.UpsertRouteFn(ctx, route)
	}
	return nil, bridgepkg.ErrBridgeRouteNotFound
}

func (s StubBridgeService) ListRoutes(ctx context.Context, bridgeInstanceID string) ([]bridgepkg.BridgeRoute, error) {
	if s.ListRoutesFn != nil {
		return s.ListRoutesFn(ctx, bridgeInstanceID)
	}
	return nil, nil
}

func (s StubBridgeService) PutBridgeTaskSubscription(
	ctx context.Context,
	subscription bridgepkg.BridgeTaskSubscription,
) error {
	if s.PutTaskSubscriptionFn != nil {
		return s.PutTaskSubscriptionFn(ctx, subscription)
	}
	return nil
}

func (s StubBridgeService) GetBridgeTaskSubscription(
	ctx context.Context,
	subscriptionID string,
) (bridgepkg.BridgeTaskSubscription, error) {
	if s.GetTaskSubscriptionFn != nil {
		return s.GetTaskSubscriptionFn(ctx, subscriptionID)
	}
	return bridgepkg.BridgeTaskSubscription{}, bridgepkg.ErrBridgeTaskSubscriptionNotFound
}

func (s StubBridgeService) ListBridgeTaskSubscriptions(
	ctx context.Context,
	query bridgepkg.BridgeTaskSubscriptionQuery,
) ([]bridgepkg.BridgeTaskSubscription, error) {
	if s.ListTaskSubscriptionsFn != nil {
		return s.ListTaskSubscriptionsFn(ctx, query)
	}
	return nil, nil
}

func (s StubBridgeService) DeleteBridgeTaskSubscription(ctx context.Context, subscriptionID string) error {
	if s.DeleteTaskSubscriptionFn != nil {
		return s.DeleteTaskSubscriptionFn(ctx, subscriptionID)
	}
	return bridgepkg.ErrBridgeTaskSubscriptionNotFound
}

func (s StubBridgeService) GetCursor(
	ctx context.Context,
	key notifications.CursorKey,
) (notifications.Cursor, error) {
	if s.GetCursorFn != nil {
		return s.GetCursorFn(ctx, key)
	}
	return notifications.Cursor{}, notifications.ErrCursorNotFound
}

func (s StubBridgeService) ResolveDeliveryTarget(
	ctx context.Context,
	req bridgepkg.ResolveDeliveryTargetRequest,
) (*bridgepkg.DeliveryTarget, error) {
	if s.ResolveDeliveryTargetFn != nil {
		return s.ResolveDeliveryTargetFn(ctx, req)
	}
	return nil, bridgepkg.ErrBridgeInstanceNotFound
}

func (s StubBridgeService) RefreshBridgeTargets(
	ctx context.Context,
	bridgeID string,
	snapshots []bridgepkg.BridgeTargetSnapshot,
) ([]bridgepkg.BridgeTarget, error) {
	if s.RefreshBridgeTargetsFn != nil {
		return s.RefreshBridgeTargetsFn(ctx, bridgeID, snapshots)
	}
	return nil, bridgepkg.ErrBridgeTargetDirectoryUnavailable
}

func (s StubBridgeService) ListBridgeTargets(
	ctx context.Context,
	query bridgepkg.BridgeTargetQuery,
) (bridgepkg.BridgeTargetsResult, error) {
	if s.ListBridgeTargetsFn != nil {
		return s.ListBridgeTargetsFn(ctx, query)
	}
	return bridgepkg.BridgeTargetsResult{}, bridgepkg.ErrBridgeTargetDirectoryUnavailable
}

func (s StubBridgeService) ResolveBridgeTarget(
	ctx context.Context,
	bridgeID string,
	query string,
) (bridgepkg.ResolveBridgeTargetResult, error) {
	if s.ResolveBridgeTargetFn != nil {
		return s.ResolveBridgeTargetFn(ctx, bridgeID, query)
	}
	return bridgepkg.ResolveBridgeTargetResult{}, bridgepkg.ErrBridgeTargetDirectoryUnavailable
}

func (s StubBridgeService) StartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	if s.StartInstanceFn != nil {
		return s.StartInstanceFn(ctx, id)
	}
	return nil, bridgepkg.ErrBridgeInstanceNotFound
}

func (s StubBridgeService) StopInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	if s.StopInstanceFn != nil {
		return s.StopInstanceFn(ctx, id)
	}
	return nil, bridgepkg.ErrBridgeInstanceNotFound
}

func (s StubBridgeService) RestartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	if s.RestartInstanceFn != nil {
		return s.RestartInstanceFn(ctx, id)
	}
	return nil, bridgepkg.ErrBridgeInstanceNotFound
}

func (s StubBridgeService) CheckBridge(
	ctx context.Context,
	extensionName string,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if s.CheckBridgeFn != nil {
		return s.CheckBridgeFn(ctx, extensionName, request)
	}
	return bridgepkg.BridgeCheckResponse{}, bridgepkg.ErrBridgeControlTransportUnavailable
}

func (s StubBridgeService) RegisterBridgeWebhook(
	ctx context.Context,
	extensionName string,
	request bridgepkg.BridgeWebhookRegistrationRequest,
) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
	if s.RegisterBridgeWebhookFn != nil {
		return s.RegisterBridgeWebhookFn(ctx, extensionName, request)
	}
	return bridgepkg.BridgeWebhookRegistrationResponse{}, bridgepkg.ErrBridgeControlTransportUnavailable
}

func (s StubBridgeService) DeliverBridge(
	ctx context.Context,
	extensionName string,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if s.DeliverBridgeFn != nil {
		return s.DeliverBridgeFn(ctx, extensionName, request)
	}
	return bridgepkg.DeliveryAck{}, bridgepkg.ErrDeliveryTransportUnavailable
}

var _ core.BridgeService = (*StubBridgeService)(nil)
