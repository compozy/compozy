package services

import (
	"context"
)

// SignalDispatcher defines the interface for dispatching signals
type SignalDispatcher interface {
	DispatchSignal(ctx context.Context, signalName string, payload map[string]any, correlationID string) error
}

// MockSignalDispatcher implements SignalDispatcher for testing
type MockSignalDispatcher struct {
	Calls         []SignalCall
	DispatchError error
}

type SignalCall struct {
	SignalName    string
	Payload       map[string]any
	CorrelationID string
}

func NewMockSignalDispatcher() *MockSignalDispatcher {
	return &MockSignalDispatcher{}
}

func (m *MockSignalDispatcher) DispatchSignal(
	_ context.Context,
	signalName string,
	payload map[string]any,
	correlationID string,
) error {
	if m.DispatchError != nil {
		return m.DispatchError
	}
	m.Calls = append(m.Calls, SignalCall{
		SignalName:    signalName,
		Payload:       payload,
		CorrelationID: correlationID,
	})
	return nil
}

// TemporalSignalDispatcher will be implemented in the worker package to avoid circular imports
type TemporalSignalDispatcher interface {
	SignalDispatcher
}
