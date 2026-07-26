package bridges

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type pendingDeliveryMetricRecord struct {
	record   DeliveryMetricRecord
	revision uint64
}

func (b *Broker) bindDeliveryMetricIdentityLocked(delivery *activeDelivery) {
	if b == nil || delivery == nil {
		return
	}
	metrics := b.metricsLocked(delivery.bridgeInstanceID)
	if metrics == nil {
		return
	}
	if metrics.scope == "" {
		metrics.scope = delivery.routingKey.Scope
		metrics.workspaceID = delivery.routingKey.WorkspaceID
	}
}

func (b *Broker) markDeliveryMetricsDirtyLocked(metrics *instanceDeliveryMetrics) {
	if b == nil || b.ledgerStore == nil || metrics == nil {
		return
	}
	metrics.revision++
	metrics.updatedAt = b.now()
	select {
	case b.metricWakeCh <- struct{}{}:
	default:
	}
}

func (b *Broker) runDeliveryMetricPersistence() {
	defer b.wg.Done()
	for {
		select {
		case <-b.metricWakeCh:
			timer := time.NewTimer(b.retryDelay)
			select {
			case <-timer.C:
			case <-b.lifecycleCtx.Done():
				stopAndDrainTimer(timer)
				return
			}
			if err := b.flushDirtyDeliveryMetrics(b.lifecycleCtx, true); err != nil {
				b.setDeliveryMetricPersistenceError(err)
				return
			}
		case <-b.lifecycleCtx.Done():
			return
		}
	}
}

func (b *Broker) setDeliveryMetricPersistenceError(err error) {
	if b == nil || err == nil {
		return
	}
	b.mu.Lock()
	b.metricPersistErr = errors.Join(b.metricPersistErr, err)
	b.mu.Unlock()
}

func (b *Broker) flushDirtyDeliveryMetrics(ctx context.Context, respectLifecycle bool) error {
	for {
		pending, ok := b.nextPendingDeliveryMetricRecord()
		if !ok {
			return nil
		}
		if err := b.persistDeliveryMetricRecord(ctx, pending.record, respectLifecycle); err != nil {
			if isTerminalDeliveryMetricPersistenceError(err) {
				b.setDeliveryMetricPersistenceError(err)
				b.setDeliveryMetricPersistenceIssue(pending.record.BridgeInstanceID, err)
				b.markDeliveryMetricRevisionPersisted(pending)
				continue
			}
			return err
		}
		b.clearDeliveryMetricPersistenceIssue(pending.record.BridgeInstanceID)
		b.markDeliveryMetricRevisionPersisted(pending)
	}
}

func (b *Broker) setDeliveryMetricPersistenceIssue(bridgeInstanceID string, err error) {
	if b == nil || strings.TrimSpace(bridgeInstanceID) == "" || err == nil {
		return
	}
	b.mu.Lock()
	if b.metricPersistIssues == nil {
		b.metricPersistIssues = make(map[string]deliveryMetricPersistenceIssue)
	}
	b.metricPersistIssues[strings.TrimSpace(bridgeInstanceID)] = deliveryMetricPersistenceIssue{
		err: err,
		at:  b.now(),
	}
	b.mu.Unlock()
}

func (b *Broker) clearDeliveryMetricPersistenceIssue(bridgeInstanceID string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	delete(b.metricPersistIssues, strings.TrimSpace(bridgeInstanceID))
	b.mu.Unlock()
}

func isTerminalDeliveryMetricPersistenceError(err error) bool {
	return errors.Is(err, ErrDeliveryLedgerNotFound) ||
		errors.Is(err, ErrDeliveryLedgerConflict) ||
		errors.Is(err, ErrBridgeInstanceNotFound)
}

func (b *Broker) markDeliveryMetricRevisionPersisted(pending pendingDeliveryMetricRecord) {
	b.mu.Lock()
	metrics := b.metrics[pending.record.BridgeInstanceID]
	if metrics != nil && pending.revision > metrics.persistedRevision {
		metrics.persistedRevision = pending.revision
	}
	b.mu.Unlock()
}

func (b *Broker) nextPendingDeliveryMetricRecord() (pendingDeliveryMetricRecord, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for bridgeInstanceID, metrics := range b.metrics {
		if metrics == nil || metrics.revision <= metrics.persistedRevision || metrics.scope == "" {
			continue
		}
		return pendingDeliveryMetricRecord{
			record:   deliveryMetricRecordFromState(bridgeInstanceID, metrics),
			revision: metrics.revision,
		}, true
	}
	return pendingDeliveryMetricRecord{}, false
}

func deliveryMetricRecordFromState(
	bridgeInstanceID string,
	metrics *instanceDeliveryMetrics,
) DeliveryMetricRecord {
	droppedByReason := cloneDeliveryDropReasons(metrics.droppedByReason)
	droppedTotal := 0
	for _, count := range droppedByReason {
		droppedTotal += count
	}
	return DeliveryMetricRecord{
		BridgeDeliveryMetrics: BridgeDeliveryMetrics{
			BridgeInstanceID:        bridgeInstanceID,
			DeliveryDroppedTotal:    droppedTotal,
			DeliveryDroppedByReason: droppedByReason,
			DeliveryFailuresTotal:   metrics.deliveryFailuresTotal,
			LastError:               metrics.lastError,
			LastErrorAt:             metrics.lastErrorAt,
			LastSuccessAt:           metrics.lastSuccessAt,
		},
		Scope:       metrics.scope,
		WorkspaceID: metrics.workspaceID,
		UpdatedAt:   metrics.updatedAt,
	}
}

func (b *Broker) persistDeliveryMetricRecord(
	ctx context.Context,
	record DeliveryMetricRecord,
	respectLifecycle bool,
) error {
	if ctx == nil {
		return errors.New("bridges: delivery metric persistence context is required")
	}
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("bridges: persist delivery metrics %q: %w", record.BridgeInstanceID, err)
		}

		callCtx, cancel := context.WithTimeout(ctx, b.requestTimeout)
		stopLifecycleCancel := func() bool { return false }
		if respectLifecycle {
			stopLifecycleCancel = context.AfterFunc(b.lifecycleCtx, cancel)
		}
		err := b.ledgerStore.PutBridgeDeliveryMetrics(callCtx, record)
		stopLifecycleCancel()
		cancel()
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrDeliveryLedgerNotFound) ||
			errors.Is(err, ErrDeliveryLedgerConflict) ||
			errors.Is(err, ErrBridgeInstanceNotFound) {
			return fmt.Errorf("bridges: persist delivery metrics %q: %w", record.BridgeInstanceID, err)
		}

		timer := time.NewTimer(b.retryDelay)
		if err := b.waitDeliveryMetricRetry(ctx, timer, err, record.BridgeInstanceID, respectLifecycle); err != nil {
			return err
		}
	}
}

func (b *Broker) waitDeliveryMetricRetry(
	ctx context.Context,
	timer *time.Timer,
	persistErr error,
	bridgeInstanceID string,
	respectLifecycle bool,
) error {
	var lifecycleDone <-chan struct{}
	if respectLifecycle {
		lifecycleDone = b.lifecycleCtx.Done()
	}
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		stopAndDrainTimer(timer)
		return fmt.Errorf(
			"bridges: persist delivery metrics %q: %w",
			bridgeInstanceID,
			errors.Join(persistErr, ctx.Err()),
		)
	case <-lifecycleDone:
		stopAndDrainTimer(timer)
		return fmt.Errorf(
			"bridges: persist delivery metrics %q: %w",
			bridgeInstanceID,
			errors.Join(persistErr, b.lifecycleCtx.Err()),
		)
	}
}
