package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/deadentity"
	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/notifications"
	"github.com/compozy/agh/internal/resources"
)

var errBridgeSecretResolverRequired = errors.New("daemon: bridge secret resolver is required")

const (
	bridgeTargetRefreshInterval = 5 * time.Minute
	bridgeTargetRefreshTimeout  = 30 * time.Second
)

// BridgeSecretResolver resolves daemon-owned bound secret material for one
// persisted bridge secret binding.
type BridgeSecretResolver interface {
	ResolveBridgeSecret(ctx context.Context, binding bridgepkg.BridgeSecretBinding) (string, error)
}

type bridgeSecretValueWriter interface {
	PutBridgeSecretValue(ctx context.Context, binding bridgepkg.BridgeSecretBinding, plaintext string) error
}

type bridgeRuntime struct {
	*bridgepkg.Service

	store           bridgeRuntimeStore
	registry        *extensionpkg.Registry
	secretResolver  BridgeSecretResolver
	broker          *bridgepkg.Broker
	logger          *slog.Logger
	now             func() time.Time
	deadEntities    *deadentity.Service
	resourceStore   resources.Store[bridgepkg.BridgeInstanceSpec]
	resourceActor   resources.MutationActor
	resourceTrigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error

	lifecycleMu             sync.Mutex
	lifecycleLocks          map[string]*bridgeLifecycleLock
	extensionLifecycleLocks map[string]*bridgeLifecycleLock
	mu                      sync.RWMutex
	extensions              extensionRuntime
	targetRefreshMu         sync.Mutex
	targetRefreshCancel     context.CancelFunc
	targetRefreshDone       chan struct{}
}

var _ extensionpkg.BridgeRuntimeResolver = (*bridgeRuntime)(nil)
var _ bridgepkg.DeliveryTransport = (*bridgeRuntime)(nil)

func (r *bridgeRuntime) PutBridgeTaskSubscription(
	ctx context.Context,
	subscription bridgepkg.BridgeTaskSubscription,
) error {
	if r == nil || r.store == nil {
		return bridgepkg.ErrBridgeTaskSubscriptionNotFound
	}
	return r.store.PutBridgeTaskSubscription(ctx, subscription)
}

func (r *bridgeRuntime) GetBridgeTaskSubscription(
	ctx context.Context,
	subscriptionID string,
) (bridgepkg.BridgeTaskSubscription, error) {
	if r == nil || r.store == nil {
		return bridgepkg.BridgeTaskSubscription{}, bridgepkg.ErrBridgeTaskSubscriptionNotFound
	}
	return r.store.GetBridgeTaskSubscription(ctx, subscriptionID)
}

func (r *bridgeRuntime) ListBridgeTaskSubscriptions(
	ctx context.Context,
	query bridgepkg.BridgeTaskSubscriptionQuery,
) ([]bridgepkg.BridgeTaskSubscription, error) {
	if r == nil || r.store == nil {
		return nil, bridgepkg.ErrBridgeTaskSubscriptionNotFound
	}
	return r.store.ListBridgeTaskSubscriptions(ctx, query)
}

func (r *bridgeRuntime) DeleteBridgeTaskSubscription(ctx context.Context, subscriptionID string) error {
	if r == nil || r.store == nil {
		return bridgepkg.ErrBridgeTaskSubscriptionNotFound
	}
	return r.store.DeleteBridgeTaskSubscription(ctx, subscriptionID)
}

func (r *bridgeRuntime) GetBridgeInstance(ctx context.Context, id string) (bridgepkg.BridgeInstance, error) {
	if r == nil {
		return bridgepkg.BridgeInstance{}, bridgepkg.ErrBridgeInstanceNotFound
	}
	instance, err := r.GetInstance(ctx, id)
	if err != nil {
		return bridgepkg.BridgeInstance{}, err
	}
	if instance == nil {
		return bridgepkg.BridgeInstance{}, bridgepkg.ErrBridgeInstanceNotFound
	}
	return *instance, nil
}

func (r *bridgeRuntime) GetCursor(
	ctx context.Context,
	key notifications.CursorKey,
) (notifications.Cursor, error) {
	if r == nil || r.store == nil {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	cursorStore, ok := r.store.(interface {
		GetCursor(context.Context, notifications.CursorKey) (notifications.Cursor, error)
	})
	if !ok {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	return cursorStore.GetCursor(ctx, key)
}

func (r *bridgeRuntime) ListCursors(
	ctx context.Context,
	query notifications.CursorQuery,
) ([]notifications.Cursor, error) {
	if r == nil || r.store == nil {
		return nil, notifications.ErrCursorNotFound
	}
	cursorStore, ok := r.store.(notifications.CursorStore)
	if !ok {
		return nil, notifications.ErrCursorNotFound
	}
	return cursorStore.ListCursors(ctx, query)
}

func (r *bridgeRuntime) AdvanceCursor(
	ctx context.Context,
	update notifications.AdvanceCursor,
) (notifications.Cursor, error) {
	if r == nil || r.store == nil {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	cursorStore, ok := r.store.(notifications.CursorStore)
	if !ok {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	return cursorStore.AdvanceCursor(ctx, update)
}

func (r *bridgeRuntime) ResetCursor(
	ctx context.Context,
	reset notifications.ResetCursor,
) (notifications.Cursor, error) {
	if r == nil || r.store == nil {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	cursorStore, ok := r.store.(notifications.CursorStore)
	if !ok {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	return cursorStore.ResetCursor(ctx, reset)
}

func (r *bridgeRuntime) RecordCursorError(
	ctx context.Context,
	report notifications.CursorError,
) (notifications.Cursor, error) {
	if r == nil || r.store == nil {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	cursorStore, ok := r.store.(notifications.CursorStore)
	if !ok {
		return notifications.Cursor{}, notifications.ErrCursorNotFound
	}
	return cursorStore.RecordCursorError(ctx, report)
}

func (r *bridgeRuntime) DeliverBridge(
	ctx context.Context,
	extensionName string,
	req bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	if r == nil {
		return bridgepkg.DeliveryAck{}, bridgepkg.ErrDeliveryTransportUnavailable
	}
	transport, ok := r.extensionRuntime().(bridgepkg.DeliveryTransport)
	if !ok || transport == nil {
		return bridgepkg.DeliveryAck{}, bridgepkg.ErrDeliveryTransportUnavailable
	}
	return transport.DeliverBridge(ctx, extensionName, req)
}

func (r *bridgeRuntime) ListBridgeTargets(
	ctx context.Context,
	query bridgepkg.BridgeTargetQuery,
) (bridgepkg.BridgeTargetsResult, error) {
	if r == nil || r.Service == nil {
		return bridgepkg.BridgeTargetsResult{}, bridgepkg.ErrBridgeTargetDirectoryUnavailable
	}
	result, err := r.Service.ListBridgeTargets(ctx, query)
	if err != nil || !result.CacheStale {
		return result, err
	}
	if err := r.refreshBridgeTargetDirectoryForInstance(ctx, query.BridgeID); err != nil {
		return result, nil
	}
	return r.Service.ListBridgeTargets(ctx, query)
}

func (r *bridgeRuntime) ResolveBridgeTarget(
	ctx context.Context,
	bridgeID string,
	query string,
) (bridgepkg.ResolveBridgeTargetResult, error) {
	if r == nil || r.Service == nil {
		return bridgepkg.ResolveBridgeTargetResult{}, bridgepkg.ErrBridgeTargetDirectoryUnavailable
	}
	if err := r.refreshStaleBridgeTargetDirectory(ctx, bridgeID); err != nil {
		return bridgepkg.ResolveBridgeTargetResult{}, err
	}
	return r.Service.ResolveBridgeTarget(ctx, bridgeID, query)
}

func (r *bridgeRuntime) refreshStaleBridgeTargetDirectory(ctx context.Context, bridgeID string) error {
	result, err := r.Service.ListBridgeTargets(ctx, bridgepkg.BridgeTargetQuery{BridgeID: bridgeID})
	if err != nil || !result.CacheStale {
		return err
	}
	if err := r.refreshBridgeTargetDirectoryForInstance(ctx, bridgeID); err != nil {
		return fmt.Errorf("daemon: refresh stale bridge target directory for %q: %w", strings.TrimSpace(bridgeID), err)
	}
	return nil
}

func (r *bridgeRuntime) BridgeTargetSnapshots(
	ctx context.Context,
	extensionName string,
	req bridgepkg.BridgeTargetSnapshotRequest,
) ([]bridgepkg.BridgeTargetSnapshot, error) {
	if r == nil {
		return nil, bridgepkg.ErrBridgeTargetDirectoryUnavailable
	}
	transport, ok := r.extensionRuntime().(bridgepkg.TargetSnapshotTransport)
	if !ok || transport == nil {
		return nil, bridgepkg.ErrBridgeTargetDirectoryUnavailable
	}
	return transport.BridgeTargetSnapshots(ctx, extensionName, req)
}

func (r *bridgeRuntime) setResourceDefinitions(
	store resources.Store[bridgepkg.BridgeInstanceSpec],
	actor resources.MutationActor,
	trigger func(context.Context, resources.ResourceKind, resources.ReconcileReason) error,
) {
	if r == nil {
		return
	}
	r.resourceStore = store
	r.resourceActor = actor
	r.resourceTrigger = trigger
}

func (r *bridgeRuntime) resourceDefinitionsEnabled() bool {
	return r != nil && r.resourceStore != nil
}

func (r *bridgeRuntime) Broker() *bridgepkg.Broker {
	if r == nil {
		return nil
	}
	return r.broker
}
