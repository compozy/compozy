package shared

import (
	"context"
	"sync"
	"testing"

	"github.com/compozy/compozy/engine/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHandler is a test implementation of TaskResponseHandler
type MockHandler struct {
	taskType task.Type
}

func (m *MockHandler) Type() task.Type {
	return m.taskType
}

func (m *MockHandler) HandleResponse(_ context.Context, _ *ResponseInput) (*ResponseOutput, error) {
	return &ResponseOutput{
		Response: "mock response",
		State:    &task.State{},
	}, nil
}

func TestResponseHandlerFactory_ConcurrentAccess(t *testing.T) {
	t.Run("Should handle concurrent handler registration safely", func(t *testing.T) {
		// This test should be run with -race flag
		// go test -race -run TestResponseHandlerFactory_ConcurrentAccess
		factory := NewResponseHandlerFactory()

		var wg sync.WaitGroup
		numRegistrations := 10
		wg.Add(numRegistrations)

		// Task types to register
		taskTypes := []task.Type{
			task.TaskTypeBasic,
			task.TaskTypeCollection,
			task.TaskTypeParallel,
			task.TaskTypeComposite,
			task.TaskTypeRouter,
			task.TaskTypeWait,
			task.TaskTypeSignal,
			task.TaskTypeAggregate,
			task.TaskTypeMemory,
			"custom-type",
		}

		// Register handlers concurrently
		for i := range numRegistrations {
			go func(index int) {
				defer wg.Done()
				taskType := taskTypes[index%len(taskTypes)]
				handler := &MockHandler{taskType: taskType}
				err := factory.RegisterHandler(taskType, handler)
				// Some registrations might fail due to duplicate types
				if err != nil {
					assert.Contains(t, err.Error(), "already registered")
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("Should handle concurrent reads safely", func(t *testing.T) {
		factory := NewResponseHandlerFactory()

		// Pre-register some handlers
		handlers := map[task.Type]TaskResponseHandler{
			task.TaskTypeBasic:      &MockHandler{taskType: task.TaskTypeBasic},
			task.TaskTypeCollection: &MockHandler{taskType: task.TaskTypeCollection},
			task.TaskTypeParallel:   &MockHandler{taskType: task.TaskTypeParallel},
		}

		for taskType, handler := range handlers {
			err := factory.RegisterHandler(taskType, handler)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		numReaders := 20
		wg.Add(numReaders)

		// Read handlers concurrently
		for i := range numReaders {
			go func(index int) {
				defer wg.Done()
				taskTypes := []task.Type{task.TaskTypeBasic, task.TaskTypeCollection, task.TaskTypeParallel}
				taskType := taskTypes[index%len(taskTypes)]
				handler, err := factory.GetHandler(taskType)
				assert.NoError(t, err)
				assert.NotNil(t, handler)
				assert.Equal(t, taskType, handler.Type())
			}(i)
		}

		wg.Wait()
	})

	t.Run("Should handle mixed read/write operations safely", func(t *testing.T) {
		factory := NewResponseHandlerFactory()

		// Pre-register basic handler
		basicHandler := &MockHandler{taskType: task.TaskTypeBasic}
		err := factory.RegisterHandler(task.TaskTypeBasic, basicHandler)
		require.NoError(t, err)

		var wg sync.WaitGroup
		numOperations := 30
		wg.Add(numOperations)

		// Mix of readers and writers
		for i := range numOperations {
			if i%3 == 0 {
				// Writer - try to register new handler
				go func(index int) {
					defer wg.Done()
					taskTypes := []task.Type{
						task.TaskTypeCollection,
						task.TaskTypeParallel,
						task.TaskTypeComposite,
						task.TaskTypeRouter,
					}
					taskType := taskTypes[index%len(taskTypes)]
					handler := &MockHandler{taskType: taskType}
					_ = factory.RegisterHandler(taskType, handler)
				}(i)
			} else {
				// Reader - get existing handler
				go func() {
					defer wg.Done()
					handler, err := factory.GetHandler(task.TaskTypeBasic)
					assert.NoError(t, err)
					assert.NotNil(t, handler)
				}()
			}
		}

		wg.Wait()
	})

	t.Run("Should validate task types concurrently", func(t *testing.T) {
		factory := NewResponseHandlerFactory()

		var wg sync.WaitGroup
		numValidations := 20
		wg.Add(numValidations)

		// Test various task types including invalid ones
		testTypes := []task.Type{
			task.TaskTypeBasic,
			task.TaskTypeCollection,
			"invalid-type",
			task.TaskTypeParallel,
			"another-invalid",
			task.TaskTypeMemory,
		}

		for i := range numValidations {
			go func(index int) {
				defer wg.Done()
				taskType := testTypes[index%len(testTypes)]
				_, err := factory.GetHandler(taskType)
				// Validation should happen before checking if handler exists
				if taskType == "invalid-type" || taskType == "another-invalid" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "unsupported task type")
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("Should handle RegisterAllHandlers concurrently with GetHandler", func(_ *testing.T) {
		factory := NewResponseHandlerFactory()

		var wg sync.WaitGroup

		// Create handlers for RegisterAllHandlers
		handlers := map[task.Type]TaskResponseHandler{
			task.TaskTypeBasic:      &MockHandler{taskType: task.TaskTypeBasic},
			task.TaskTypeCollection: &MockHandler{taskType: task.TaskTypeCollection},
			task.TaskTypeParallel:   &MockHandler{taskType: task.TaskTypeParallel},
			task.TaskTypeComposite:  &MockHandler{taskType: task.TaskTypeComposite},
			task.TaskTypeRouter:     &MockHandler{taskType: task.TaskTypeRouter},
			task.TaskTypeWait:       &MockHandler{taskType: task.TaskTypeWait},
			task.TaskTypeSignal:     &MockHandler{taskType: task.TaskTypeSignal},
			task.TaskTypeAggregate:  &MockHandler{taskType: task.TaskTypeAggregate},
		}

		// Concurrent RegisterAllHandlers
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := factory.RegisterAllHandlers(
				handlers[task.TaskTypeBasic],
				handlers[task.TaskTypeCollection],
				handlers[task.TaskTypeParallel],
				handlers[task.TaskTypeComposite],
				handlers[task.TaskTypeRouter],
				handlers[task.TaskTypeWait],
				handlers[task.TaskTypeSignal],
				handlers[task.TaskTypeAggregate],
			)
			// May fail if handlers already registered
			_ = err
		}()

		// Concurrent reads while registration is happening
		numReaders := 10
		wg.Add(numReaders)
		for range numReaders {
			go func() {
				defer wg.Done()
				// Try to get various handlers
				for taskType := range handlers {
					_, _ = factory.GetHandler(taskType)
				}
			}()
		}

		wg.Wait()
	})
}
