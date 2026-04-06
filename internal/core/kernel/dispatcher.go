package kernel

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
)

// Handler executes one typed command and returns its typed result.
type Handler[C any, R any] interface {
	Handle(ctx context.Context, cmd C) (R, error)
}

// Dispatcher stores typed handlers keyed by command type.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[reflect.Type]any
}

// UnregisteredHandlerError reports a command type with no registered handler.
type UnregisteredHandlerError struct {
	CommandType reflect.Type
}

// Error implements error.
func (e *UnregisteredHandlerError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("kernel: no handler registered for %s", formatType(e.CommandType))
}

// HandlerTypeMismatchError reports a command dispatched with an incompatible
// result type for the registered handler.
type HandlerTypeMismatchError struct {
	CommandType       reflect.Type
	ResultType        reflect.Type
	ActualHandlerType reflect.Type
}

// Error implements error.
func (e *HandlerTypeMismatchError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf(
		"kernel: handler type mismatch for %s with result %s (registered %s)",
		formatType(e.CommandType),
		formatType(e.ResultType),
		formatType(e.ActualHandlerType),
	)
}

// NewDispatcher constructs an empty typed dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[reflect.Type]any),
	}
}

// Register stores one handler for the command type C.
func Register[C any, R any](d *Dispatcher, h Handler[C, R]) {
	commandType := reflect.TypeFor[C]()

	d.mu.Lock()
	d.handlers[commandType] = h
	d.mu.Unlock()
}

// Dispatch routes cmd to its registered handler and returns the typed result.
func Dispatch[C any, R any](ctx context.Context, d *Dispatcher, cmd C) (R, error) {
	var zero R

	commandType := reflect.TypeFor[C]()
	if d == nil {
		return zero, &UnregisteredHandlerError{CommandType: commandType}
	}

	d.mu.RLock()
	h, ok := d.handlers[commandType]
	d.mu.RUnlock()
	if !ok {
		return zero, &UnregisteredHandlerError{CommandType: commandType}
	}

	typed, ok := h.(Handler[C, R])
	if !ok {
		return zero, &HandlerTypeMismatchError{
			CommandType:       commandType,
			ResultType:        reflect.TypeFor[R](),
			ActualHandlerType: reflect.TypeOf(h),
		}
	}
	return typed.Handle(ctx, cmd)
}

func registeredCommandTypes(d *Dispatcher) []reflect.Type {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	types := make([]reflect.Type, 0, len(d.handlers))
	for commandType := range d.handlers {
		types = append(types, commandType)
	}
	sort.Slice(types, func(i, j int) bool {
		return formatType(types[i]) < formatType(types[j])
	})
	return types
}

func formatType(t reflect.Type) string {
	if t == nil {
		return "<nil>"
	}
	return t.String()
}
