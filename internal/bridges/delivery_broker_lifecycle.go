package bridges

import (
	"context"
)

// SetTransport swaps the negotiated extension-delivery transport used by the broker.
func (b *Broker) SetTransport(transport DeliveryTransport) {
	if b == nil {
		return
	}
	b.mu.Lock()
	if b.phase != brokerPhaseRunning {
		b.mu.Unlock()
		return
	}
	b.transport = transport
	for _, route := range b.routes {
		b.signalRoute(route)
	}
	b.mu.Unlock()
}

// Close closes mutable admission, stops workers, joins admitted work, and flushes terminal metrics.
func (b *Broker) Close() {
	if b == nil {
		return
	}
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.phase = brokerPhaseClosing
		cancel := b.cancel
		b.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		b.wg.Wait()
		if b.ledgerStore != nil {
			flushCtx, cancelFlush := context.WithTimeout(context.Background(), b.requestTimeout)
			if err := b.flushDirtyDeliveryMetrics(flushCtx, false); err != nil {
				b.setDeliveryMetricPersistenceError(err)
			}
			cancelFlush()
		}
		b.mu.Lock()
		b.phase = brokerPhaseClosed
		close(b.closed)
		b.mu.Unlock()
	})
	<-b.closed
}

func (b *Broker) beginMutableOperation(ctx context.Context) (context.Context, func(), error) {
	b.mu.Lock()
	if b.phase != brokerPhaseRunning {
		b.mu.Unlock()
		return nil, nil, ErrBrokerClosed
	}
	b.wg.Add(1)
	lifecycleCtx := b.lifecycleCtx
	b.mu.Unlock()

	operationCtx, cancelOperation := context.WithCancel(ctx)
	stopLifecycleCancel := context.AfterFunc(lifecycleCtx, cancelOperation)
	return operationCtx, func() {
		stopLifecycleCancel()
		cancelOperation()
		b.wg.Done()
	}, nil
}
