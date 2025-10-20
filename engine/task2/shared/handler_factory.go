package shared

import (
	"fmt"
	"sort"
	"sync"

	"github.com/compozy/compozy/engine/task"
)

// ResponseHandlerFactory creates response handlers based on task type
type ResponseHandlerFactory struct {
	mu       sync.RWMutex
	handlers map[task.Type]TaskResponseHandler
}

// NewResponseHandlerFactory creates a new handler factory
func NewResponseHandlerFactory() *ResponseHandlerFactory {
	return &ResponseHandlerFactory{
		handlers: make(map[task.Type]TaskResponseHandler),
	}
}

// RegisterHandler registers a handler for a specific task type
func (f *ResponseHandlerFactory) RegisterHandler(taskType task.Type, handler TaskResponseHandler) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Validate handler type matches
	if handler.Type() != taskType {
		return fmt.Errorf("handler type %s does not match registration type %s", handler.Type(), taskType)
	}
	// Check for duplicate registration
	if _, exists := f.handlers[taskType]; exists {
		return fmt.Errorf("handler for task type %s already registered", taskType)
	}
	f.handlers[taskType] = handler
	return nil
}

// GetHandler returns the appropriate handler for a task type
func (f *ResponseHandlerFactory) GetHandler(taskType task.Type) (TaskResponseHandler, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	// Validate task type
	if err := f.validateTaskType(taskType); err != nil {
		return nil, err
	}
	handler, exists := f.handlers[taskType]
	if !exists {
		return nil, fmt.Errorf("no handler registered for task type %s", taskType)
	}
	return handler, nil
}

// GetHandlerForConfig returns the appropriate handler for a task config
func (f *ResponseHandlerFactory) GetHandlerForConfig(config *task.Config) (TaskResponseHandler, error) {
	if config == nil {
		return nil, fmt.Errorf("task config cannot be nil")
	}
	return f.GetHandler(config.Type)
}

// validateTaskType validates that the task type is supported
func (f *ResponseHandlerFactory) validateTaskType(taskType task.Type) error {
	switch taskType {
	case task.TaskTypeBasic,
		task.TaskTypeCollection,
		task.TaskTypeParallel,
		task.TaskTypeComposite,
		task.TaskTypeRouter,
		task.TaskTypeWait,
		task.TaskTypeSignal,
		task.TaskTypeAggregate,
		task.TaskTypeMemory:
		return nil
	default:
		return fmt.Errorf("unsupported task type: %s", taskType)
	}
}

// RegisterAllHandlers registers all task-specific handlers
// This is a convenience method for registering all handlers at once
func (f *ResponseHandlerFactory) RegisterAllHandlers(
	basicHandler TaskResponseHandler,
	collectionHandler TaskResponseHandler,
	parallelHandler TaskResponseHandler,
	compositeHandler TaskResponseHandler,
	routerHandler TaskResponseHandler,
	waitHandler TaskResponseHandler,
	signalHandler TaskResponseHandler,
	aggregateHandler TaskResponseHandler,
) error {
	// Validate that all handlers are non-nil
	if basicHandler == nil || collectionHandler == nil || parallelHandler == nil ||
		compositeHandler == nil || routerHandler == nil || waitHandler == nil ||
		signalHandler == nil || aggregateHandler == nil {
		return fmt.Errorf("all handlers must be non-nil")
	}
	// Register each handler with type validation
	handlers := []struct {
		taskType task.Type
		handler  TaskResponseHandler
	}{
		{task.TaskTypeBasic, basicHandler},
		{task.TaskTypeCollection, collectionHandler},
		{task.TaskTypeParallel, parallelHandler},
		{task.TaskTypeComposite, compositeHandler},
		{task.TaskTypeRouter, routerHandler},
		{task.TaskTypeWait, waitHandler},
		{task.TaskTypeSignal, signalHandler},
		{task.TaskTypeAggregate, aggregateHandler},
	}
	for _, h := range handlers {
		if err := f.RegisterHandler(h.taskType, h.handler); err != nil {
			return fmt.Errorf("failed to register %s handler: %w", h.taskType, err)
		}
	}
	return nil
}

// HasHandler checks if a handler is registered for a task type
func (f *ResponseHandlerFactory) HasHandler(taskType task.Type) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, exists := f.handlers[taskType]
	return exists
}

// ListRegisteredTypes returns all registered task types
func (f *ResponseHandlerFactory) ListRegisteredTypes() []task.Type {
	f.mu.RLock()
	defer f.mu.RUnlock()
	types := make([]task.Type, 0, len(f.handlers))
	for t := range f.handlers {
		types = append(types, t)
	}
	// Sort for deterministic order
	sort.Slice(types, func(i, j int) bool {
		return string(types[i]) < string(types[j])
	})
	return types
}
