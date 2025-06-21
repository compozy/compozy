package services

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/compozy/compozy/engine/task"
)

// MockConfigStore is a mock implementation of ConfigStore
type MockConfigStore struct {
	mock.Mock
}

// NewMockConfigStore creates a new mock config store
func NewMockConfigStore() *MockConfigStore {
	return &MockConfigStore{}
}

// Save mocks the Save method
func (m *MockConfigStore) Save(ctx context.Context, taskExecID string, config *task.Config) error {
	args := m.Called(ctx, taskExecID, config)
	return args.Error(0)
}

// Get mocks the Get method
func (m *MockConfigStore) Get(ctx context.Context, taskExecID string) (*task.Config, error) {
	args := m.Called(ctx, taskExecID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*task.Config), args.Error(1) //nolint:errcheck,forcetypeassert // Mock type assertion is safe
}

// Delete mocks the Delete method
func (m *MockConfigStore) Delete(ctx context.Context, taskExecID string) error {
	args := m.Called(ctx, taskExecID)
	return args.Error(0)
}

// SaveMetadata mocks the SaveMetadata method
func (m *MockConfigStore) SaveMetadata(ctx context.Context, key string, data []byte) error {
	args := m.Called(ctx, key, data)
	return args.Error(0)
}

// GetMetadata mocks the GetMetadata method
func (m *MockConfigStore) GetMetadata(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1) //nolint:errcheck,forcetypeassert // Mock type assertion is safe
}

// DeleteMetadata mocks the DeleteMetadata method
func (m *MockConfigStore) DeleteMetadata(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

// Close mocks the Close method
func (m *MockConfigStore) Close() error {
	args := m.Called()
	return args.Error(0)
}
