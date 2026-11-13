package session

import "context"

// WithTransportReadinessGate holds provider resumes that start off the boot path until the daemon
// reports that its HTTP and UDS listeners are up. A provider's hosted MCP proxy is a client of
// those listeners; starting one earlier makes it miss its bind window. Managers built without the
// option (tests, headless runs) never wait.
func WithTransportReadinessGate() Option {
	return func(manager *Manager) {
		manager.transportsListening = make(chan struct{})
	}
}

// MarkTransportsListening releases every resume held by the transport readiness gate.
func (m *Manager) MarkTransportsListening() {
	if m == nil || m.transportsListening == nil {
		return
	}
	m.transportsListeningOnce.Do(func() { close(m.transportsListening) })
}

// waitTransportsListening blocks until the listeners are up or ctx ends.
func (m *Manager) waitTransportsListening(ctx context.Context) error {
	if m == nil || m.transportsListening == nil {
		return nil
	}
	select {
	case <-m.transportsListening:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
