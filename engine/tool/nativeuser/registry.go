package nativeuser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Handler defines the signature required for Go-native tool handlers.
// The input and config maps are copies derived from the tool configuration to
// prevent mutation of shared state.
type Handler func(ctx context.Context, input map[string]any, cfg map[string]any) (map[string]any, error)

// Definition represents a registered native tool handler.
type Definition struct {
	ID      string
	Handler Handler
}

var (
	registry sync.Map
	// ErrInvalidID indicates an empty tool identifier was provided.
	ErrInvalidID = errors.New("native tool id is required")
	// ErrNilHandler indicates a nil handler was provided during registration.
	ErrNilHandler = errors.New("native tool handler is required")
	// ErrAlreadyRegistered indicates a handler has already been registered for the given ID.
	ErrAlreadyRegistered = errors.New("native tool handler already registered")
)

// Register stores a native tool handler for the given ID. IDs are case-sensitive after trimming.
func Register(id string, handler Handler) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ErrInvalidID
	}
	if handler == nil {
		return ErrNilHandler
	}
	definition := Definition{ID: trimmed, Handler: handler}
	if _, loaded := registry.LoadOrStore(trimmed, definition); loaded {
		return fmt.Errorf("%w: %s", ErrAlreadyRegistered, trimmed)
	}
	return nil
}

// Lookup retrieves a registered native handler definition by ID.
func Lookup(id string) (Definition, bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return Definition{}, false
	}
	value, ok := registry.Load(trimmed)
	if !ok {
		return Definition{}, false
	}
	def, ok := value.(Definition)
	if !ok {
		return Definition{}, false
	}
	return def, true
}

// IDs returns the set of registered native tool identifiers.
func IDs() []string {
	ids := make([]string, 0)
	registry.Range(func(key, _ any) bool {
		if id, ok := key.(string); ok {
			ids = append(ids, id)
		}
		return true
	})
	return ids
}

// Reset clears the registry. This helper is intended for tests.
func Reset() {
	registry.Range(func(key, _ any) bool {
		registry.Delete(key)
		return true
	})
}
