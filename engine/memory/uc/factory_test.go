package uc

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/memory/service"
	"github.com/compozy/compozy/engine/memory/testutil"
	"github.com/compozy/compozy/engine/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockMemoryOperationsService is a mock implementation of service.MemoryOperationsService
type mockMemoryOperationsService struct {
	mock.Mock
}

func (m *mockMemoryOperationsService) Read(
	ctx context.Context,
	req *service.ReadRequest,
) (*service.ReadResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.ReadResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMemoryOperationsService) ReadPaginated(
	ctx context.Context,
	req *service.ReadPaginatedRequest,
) (*service.ReadPaginatedResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.ReadPaginatedResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMemoryOperationsService) Write(
	ctx context.Context,
	req *service.WriteRequest,
) (*service.WriteResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.WriteResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMemoryOperationsService) Append(
	ctx context.Context,
	req *service.AppendRequest,
) (*service.AppendResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.AppendResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMemoryOperationsService) Delete(
	ctx context.Context,
	req *service.DeleteRequest,
) (*service.DeleteResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.DeleteResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMemoryOperationsService) Flush(
	ctx context.Context,
	req *service.FlushRequest,
) (*service.FlushResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.FlushResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMemoryOperationsService) Clear(
	ctx context.Context,
	req *service.ClearRequest,
) (*service.ClearResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.ClearResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMemoryOperationsService) Health(
	ctx context.Context,
	req *service.HealthRequest,
) (*service.HealthResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.HealthResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMemoryOperationsService) Stats(
	ctx context.Context,
	req *service.StatsRequest,
) (*service.StatsResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*service.StatsResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestFactory(t *testing.T) {
	t.Run("Should create factory with provided service", func(t *testing.T) {
		// Setup
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		mockService := &mockMemoryOperationsService{}
		factory, err := NewFactory(setup.Manager, nil, mockService)
		require.NoError(t, err)

		assert.NotNil(t, factory)
		assert.Equal(t, mockService, factory.memoryService)
	})

	t.Run("Should create factory with default service when nil provided", func(t *testing.T) {
		// Setup
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		factory, err := NewFactory(setup.Manager, nil, nil)
		require.NoError(t, err)

		assert.NotNil(t, factory)
		assert.NotNil(t, factory.memoryService)
	})

	t.Run("Should create use cases with injected service", func(t *testing.T) {
		// Setup
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		mockService := &mockMemoryOperationsService{}
		factory, err := NewFactory(setup.Manager, &worker.Worker{}, mockService)
		require.NoError(t, err)

		// Test creating various use cases
		readUC := factory.CreateReadMemory()
		assert.NotNil(t, readUC)
		assert.Equal(t, mockService, readUC.Service)

		writeUC := factory.CreateWriteMemory("test_ref", "test_key", &WriteMemoryInput{})
		assert.NotNil(t, writeUC)
		assert.Equal(t, mockService, writeUC.service)

		appendUC := factory.CreateAppendMemory("test_ref", "test_key", &AppendMemoryInput{})
		assert.NotNil(t, appendUC)
		assert.Equal(t, mockService, appendUC.service)

		statsUC, err := factory.CreateStatsMemory()
		require.NoError(t, err)
		assert.NotNil(t, statsUC)
		assert.Equal(t, mockService, statsUC.service)
	})
}

func TestDependencyInjection(t *testing.T) {
	t.Run("Should use injected service in flush memory", func(t *testing.T) {
		// Setup
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()

		mockService := &mockMemoryOperationsService{}
		expectedResponse := &service.FlushResponse{
			Success:          true,
			SummaryGenerated: false,
			MessageCount:     10,
			TokenCount:       100,
		}

		// Set expectation
		mockService.On("Flush", mock.Anything, mock.Anything).Return(expectedResponse, nil)

		// Create flush memory UC with injected service
		flushUC, err := NewFlushMemory(
			setup.Manager,
			"test_ref",
			"test_key",
			&FlushMemoryInput{},
			mockService,
		)
		require.NoError(t, err)

		// Execute
		ctx := context.Background()
		result, err := flushUC.Execute(ctx)

		// Verify
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 10, result.MessageCount)
		assert.Equal(t, 100, result.TokenCount)

		// Verify mock was called
		mockService.AssertExpectations(t)
	})
}
