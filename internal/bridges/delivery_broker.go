package bridges

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	redactpkg "github.com/compozy/agh/internal/redact"
)

// NewBroker constructs a delivery broker with bounded per-route queues and
// background workers for negotiated extension delivery.
func NewBroker(transport DeliveryTransport, opts ...DeliveryBrokerOption) *Broker {
	broker := &Broker{
		transport:           transport,
		now:                 func() time.Time { return time.Now().UTC() },
		queueCapacity:       defaultDeliveryQueueCapacity,
		retryDelay:          defaultDeliveryRetryDelay,
		requestTimeout:      defaultDeliveryRequestTimeout,
		lifecycleCtx:        context.Background(),
		deliveries:          make(map[string]*activeDelivery),
		turnIndex:           make(map[turnIndexKey]string),
		sessionIndex:        make(map[string]map[string]struct{}),
		routes:              make(map[string]*routeWorker),
		bridgeRoutes:        make(map[string]map[string]*routeWorker),
		metrics:             make(map[string]*instanceDeliveryMetrics),
		metricPersistIssues: make(map[string]deliveryMetricPersistenceIssue),
		metricWakeCh:        make(chan struct{}, 1),
		registrationsReady:  true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(broker)
		}
	}
	if broker.now == nil {
		broker.now = func() time.Time { return time.Now().UTC() }
	}
	if broker.queueCapacity < 2 {
		broker.queueCapacity = 2
	}
	if broker.retryDelay <= 0 {
		broker.retryDelay = defaultDeliveryRetryDelay
	}
	if broker.requestTimeout <= 0 {
		broker.requestTimeout = defaultDeliveryRequestTimeout
	}
	baseCtx := broker.lifecycleCtx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	broker.lifecycleCtx, broker.cancel = context.WithCancel(baseCtx)
	if broker.ledgerStore != nil {
		broker.wg.Add(1)
		go broker.runDeliveryMetricPersistence()
	}
	return broker
}

// SetTransport swaps the negotiated extension-delivery transport used by the broker.
func (b *Broker) SetTransport(transport DeliveryTransport) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.transport = transport
	routes := make([]*routeWorker, 0, len(b.routes))
	for _, route := range b.routes {
		routes = append(routes, route)
	}
	b.mu.Unlock()
	for _, route := range routes {
		b.signalRoute(route)
	}
}

// Close stops every background route worker.
func (b *Broker) Close() {
	if b == nil {
		return
	}
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	if b.ledgerStore != nil {
		flushCtx, cancel := context.WithTimeout(context.Background(), b.requestTimeout)
		if err := b.flushDirtyDeliveryMetrics(flushCtx, false); err != nil {
			b.setDeliveryMetricPersistenceError(err)
		}
		cancel()
	}
}

// RegisterPromptDelivery binds one prompted session turn to a live delivery
// projection and optionally seeds the broker from already-persisted turn events.
func (b *Broker) RegisterPromptDelivery(
	ctx context.Context,
	reg PromptDeliveryRegistration,
) (*DeliverySnapshot, error) {
	if b == nil {
		return nil, errors.New("bridges: delivery broker is required")
	}
	if ctx == nil {
		return nil, errors.New("bridges: delivery registration context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.registrationMu.Lock()
	defer b.registrationMu.Unlock()

	normalized := reg.normalize()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	routeHash, err := normalized.RoutingKey.Hash()
	if err != nil {
		return nil, fmt.Errorf("bridges: hash delivery routing key: %w", err)
	}

	b.mu.Lock()
	if !b.registrationsReady {
		b.mu.Unlock()
		return nil, ErrDeliveryReconciliationPending
	}
	deliveryKey := newTurnIndexKey(normalized.SessionID, normalized.TurnID)
	if snapshot, found, lookupErr := b.registeredDeliveryLocked(deliveryKey); found || lookupErr != nil {
		b.mu.Unlock()
		if lookupErr != nil {
			return nil, lookupErr
		}
		return &snapshot, nil
	}

	deliveryID, err := b.reserveDeliveryIDLocked(normalized.DeliveryID)
	if err != nil {
		b.mu.Unlock()
		return nil, err
	}
	createdAt := b.now()
	delivery := newActiveDelivery(deliveryID, routeHash, normalized, createdAt)
	b.mu.Unlock()

	if err := b.createDeliveryLedgerRecord(ctx, delivery, createdAt); err != nil {
		return nil, err
	}

	b.mu.Lock()
	snapshot, routeToSignal, publishErr := b.publishPromptDeliveryLocked(
		delivery,
		deliveryKey,
		routeHash,
		normalized,
	)
	b.mu.Unlock()
	if publishErr != nil {
		if checkpointErr := b.checkpointInvalidRegistration(ctx, delivery, publishErr); checkpointErr != nil {
			return nil, errors.Join(publishErr, checkpointErr)
		}
		return nil, publishErr
	}

	if routeToSignal != nil {
		b.signalRoute(routeToSignal)
	}
	return &snapshot, nil
}

func (b *Broker) publishPromptDeliveryLocked(
	delivery *activeDelivery,
	deliveryKey turnIndexKey,
	routeHash string,
	registration PromptDeliveryRegistration,
) (DeliverySnapshot, *routeWorker, error) {
	b.deliveries[delivery.deliveryID] = delivery
	b.bindDeliveryMetricIdentityLocked(delivery)
	b.turnIndex[deliveryKey] = delivery.deliveryID
	if _, ok := b.sessionIndex[registration.SessionID]; !ok {
		b.sessionIndex[registration.SessionID] = make(map[string]struct{})
	}
	b.sessionIndex[registration.SessionID][delivery.deliveryID] = struct{}{}
	route := b.ensureRouteLocked(routeHash, registration.RoutingKey.BridgeInstanceID, registration.ExtensionName)
	var routeToSignal *routeWorker
	for _, event := range registration.SeedEvents {
		seedRoute, err := b.projectDeliveryEventLocked(delivery, event)
		if err != nil {
			b.removeDeliveryLocked(route, delivery)
			return DeliverySnapshot{}, nil, err
		}
		if seedRoute != nil {
			routeToSignal = seedRoute
		}
	}
	return cloneDeliverySnapshot(b.snapshotLocked(delivery)), routeToSignal, nil
}

func (b *Broker) registeredDeliveryLocked(
	deliveryKey turnIndexKey,
) (DeliverySnapshot, bool, error) {
	existingID, found := b.turnIndex[deliveryKey]
	if !found {
		return DeliverySnapshot{}, false, nil
	}
	existing := b.deliveries[existingID]
	if existing == nil {
		return DeliverySnapshot{}, true, ErrDeliveryNotFound
	}
	return cloneDeliverySnapshot(b.snapshotLocked(existing)), true, nil
}

func (b *Broker) reserveDeliveryIDLocked(requestedID string) (string, error) {
	if requestedID != "" {
		if _, exists := b.deliveries[requestedID]; exists {
			return "", fmt.Errorf("%w: %s", ErrDeliveryIDConflict, requestedID)
		}
		return requestedID, nil
	}
	for {
		deliveryID := newDeliveryID()
		if _, exists := b.deliveries[deliveryID]; !exists {
			return deliveryID, nil
		}
	}
}

// Deliver enqueues one already-projected delivery event for ordered extension delivery.
func (b *Broker) Deliver(ctx context.Context, evt DeliveryEvent) error {
	if b == nil {
		return errors.New("bridges: delivery broker is required")
	}
	if ctx == nil {
		return errors.New("bridges: delivery context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	normalized := evt.normalize()
	if err := normalized.Validate(); err != nil {
		return err
	}

	routeHash, err := normalized.RoutingKey.Hash()
	if err != nil {
		return fmt.Errorf("bridges: hash delivery routing key: %w", err)
	}

	b.mu.Lock()
	delivery, ok := b.deliveries[normalized.DeliveryID]
	if !ok {
		b.mu.Unlock()
		return ErrDeliveryNotFound
	}
	if delivery.routeHash != routeHash {
		b.mu.Unlock()
		return errors.New("bridges: delivery event routing key does not match registered delivery")
	}
	if delivery.final {
		b.mu.Unlock()
		return nil
	}
	route := b.ensureRouteLocked(routeHash, normalized.BridgeInstanceID, delivery.extensionName)
	err = b.enqueueEventLocked(route, delivery, normalized)
	if err != nil {
		if errors.Is(err, errProgressEventDropped) {
			b.mu.Unlock()
			return nil
		}
		b.mu.Unlock()
		return err
	}
	b.applyQueuedEventLocked(delivery, normalized)
	b.mu.Unlock()

	b.signalRoute(route)
	return nil
}

// Snapshot returns the current resumable state for one active delivery.
func (b *Broker) Snapshot(ctx context.Context, deliveryID string) (*DeliverySnapshot, error) {
	if b == nil {
		return nil, errors.New("bridges: delivery broker is required")
	}
	if ctx == nil {
		return nil, errors.New("bridges: delivery snapshot context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(deliveryID)
	if trimmed == "" {
		return nil, errors.New("bridges: delivery snapshot id is required")
	}

	b.mu.Lock()
	delivery := b.deliveries[trimmed]
	if delivery == nil {
		b.mu.Unlock()
		return nil, ErrDeliveryNotFound
	}
	snapshot := cloneDeliverySnapshot(b.snapshotLocked(delivery))
	b.mu.Unlock()
	return &snapshot, nil
}

// ProjectEvent converts one live or persisted session output event into the
// delivery-oriented stream for the registered prompt turn.
func (b *Broker) ProjectEvent(ctx context.Context, sessionID string, event DeliveryProjectionEvent) error {
	if b == nil {
		return errors.New("bridges: delivery broker is required")
	}
	if ctx == nil {
		return errors.New("bridges: delivery projection context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	sessionID = strings.TrimSpace(sessionID)
	turnID := strings.TrimSpace(event.TurnID)
	if sessionID == "" || turnID == "" {
		return nil
	}

	b.mu.Lock()
	deliveryID, ok := b.turnIndex[newTurnIndexKey(sessionID, turnID)]
	if !ok {
		b.mu.Unlock()
		return nil
	}
	delivery := b.deliveries[deliveryID]
	if delivery == nil {
		b.mu.Unlock()
		return nil
	}

	route, err := b.projectDeliveryEventLocked(delivery, event)
	if err != nil {
		b.mu.Unlock()
		return err
	}
	b.mu.Unlock()

	if route != nil {
		b.signalRoute(route)
	}
	return nil
}

// FailSession marks every unfinished delivery for the stopped session as a
// terminal error so adapters do not silently orphan bridge responses.
func (b *Broker) FailSession(ctx context.Context, sessionID string, reason string) error {
	if b == nil {
		return errors.New("bridges: delivery broker is required")
	}
	if ctx == nil {
		return errors.New("bridges: delivery fail context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = sessionStoppedDeliveryMessage
	}
	reason = redactpkg.String(reason)

	type pendingSignal struct {
		route *routeWorker
	}
	signals := make([]pendingSignal, 0)

	b.mu.Lock()
	deliveryIDs := b.sessionDeliveriesLocked(sessionID)
	for _, deliveryID := range deliveryIDs {
		delivery := b.deliveries[deliveryID]
		if delivery == nil || delivery.final {
			continue
		}

		projected := DeliveryEvent{
			DeliveryID:       delivery.deliveryID,
			BridgeInstanceID: delivery.bridgeInstanceID,
			RoutingKey:       delivery.routingKey,
			DeliveryTarget:   delivery.target,
			Seq:              delivery.latestSeq + 1,
			EventType:        DeliveryEventTypeError,
			Content:          terminalFailureContent(delivery.currentContent, reason),
			Final:            true,
			Operation:        delivery.operation,
			Reference:        cloneDeliveryReference(delivery.reference),
			Error:            &DeliveryErrorDetail{Message: reason},
			ProviderMetadata: cloneRawJSON(delivery.providerMetadata),
		}
		route := b.ensureRouteLocked(delivery.routeHash, delivery.bridgeInstanceID, delivery.extensionName)
		if err := b.enqueueEventLocked(route, delivery, projected); err != nil {
			b.mu.Unlock()
			return err
		}
		b.applyQueuedEventLocked(delivery, projected)
		signals = append(signals, pendingSignal{route: route})
	}
	b.mu.Unlock()

	for _, signal := range signals {
		b.signalRoute(signal.route)
	}
	return nil
}
