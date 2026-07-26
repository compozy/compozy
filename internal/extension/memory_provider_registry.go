package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/store"
)

const (
	memoryProviderRegistryBuiltinKey = "builtin"
)

const (
	defaultMemoryProviderName      = "local"
	memoryProviderCollisionEvent   = eventspkg.MemoryProviderCollision
	memoryProviderNameCollision    = "provider_name"
	memoryProviderToolCollision    = "tool_name"
	memoryProviderReservedToolName = "reserved_tool_name"
	memoryProviderCollisionSummary = "memory provider collision"
)

var (
	// ErrMemoryProviderNotFound reports that no registered memory provider matched a lookup.
	ErrMemoryProviderNotFound = errors.New("extension: memory provider not found")
	// ErrMemoryProviderCollision reports a deterministic memory provider registration collision.
	ErrMemoryProviderCollision = errors.New("extension: memory provider collision")
)

// MemoryProviderRegistration describes one registered memory provider implementation.
type MemoryProviderRegistration struct {
	Name          string
	Version       string
	ExtensionName string
	Provider      memcontract.MemoryProvider
	ToolNames     []string
	Bundled       bool
}

// MemoryProviderCollisionError describes a rejected provider registration.
type MemoryProviderCollisionError struct {
	Name              string
	ExistingExtension string
	IncomingExtension string
	Reason            string
	ToolName          string
}

// MemoryProviderNotFoundError describes a missing provider lookup.
type MemoryProviderNotFoundError struct {
	Name string
}

// MemoryProviderRegistryOption customizes MemoryProviderRegistry.
type MemoryProviderRegistryOption func(*MemoryProviderRegistry)

// MemoryProviderRegistry owns MemoryProvider registration and workspace selection.
type MemoryProviderRegistry struct {
	mu            sync.RWMutex
	providers     map[string]MemoryProviderRegistration
	active        map[string]string
	toolOwners    map[string]string
	reservedTools map[string]string
	eventWriter   memoryProviderEventWriter
	now           func() time.Time
}

type memoryProviderEventWriter interface {
	WriteEventSummary(ctx context.Context, summary store.EventSummary) error
}

type memoryProviderCollisionPayload struct {
	Provider          string    `json:"provider"`
	ExistingExtension string    `json:"existing_extension,omitempty"`
	IncomingExtension string    `json:"incoming_extension,omitempty"`
	Reason            string    `json:"reason"`
	ToolName          string    `json:"tool_name,omitempty"`
	OccurredAt        time.Time `json:"occurred_at"`
}

// WithMemoryProviderEventSummaryStore records provider collisions into observability.
func WithMemoryProviderEventSummaryStore(writer memoryProviderEventWriter) MemoryProviderRegistryOption {
	return func(registry *MemoryProviderRegistry) {
		registry.eventWriter = writer
	}
}

// WithMemoryProviderReservedTools reserves built-in tool names against provider claims.
func WithMemoryProviderReservedTools(names ...string) MemoryProviderRegistryOption {
	return func(registry *MemoryProviderRegistry) {
		for _, name := range names {
			normalized := normalizeMemoryProviderToolName(name)
			if normalized == "" {
				continue
			}
			registry.reservedTools[normalized] = memoryProviderRegistryBuiltinKey
		}
	}
}

// WithMemoryProviderRegistryClock injects a deterministic event timestamp.
func WithMemoryProviderRegistryClock(now func() time.Time) MemoryProviderRegistryOption {
	return func(registry *MemoryProviderRegistry) {
		if now != nil {
			registry.now = now
		}
	}
}

// NewMemoryProviderRegistry constructs an in-memory provider registry.
func NewMemoryProviderRegistry(opts ...MemoryProviderRegistryOption) *MemoryProviderRegistry {
	registry := &MemoryProviderRegistry{
		providers:     map[string]MemoryProviderRegistration{},
		active:        map[string]string{},
		toolOwners:    map[string]string{},
		reservedTools: map[string]string{},
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(registry)
		}
	}
	return registry
}

// Register adds one provider unless its name or tool names collide.
func (r *MemoryProviderRegistry) Register(ctx context.Context, registration MemoryProviderRegistration) error {
	if err := r.checkContext(ctx); err != nil {
		return err
	}
	normalized, err := normalizeMemoryProviderName(registration.Name)
	if err != nil {
		return err
	}
	if registration.Provider == nil {
		return errors.New("extension: memory provider implementation is required")
	}
	next := normalizeMemoryProviderRegistration(registration, normalized)

	r.mu.Lock()
	if existing, ok := r.providers[normalized]; ok {
		collision := MemoryProviderCollisionError{
			Name:              normalized,
			ExistingExtension: existing.ExtensionName,
			IncomingExtension: next.ExtensionName,
			Reason:            memoryProviderNameCollision,
		}
		r.mu.Unlock()
		return r.collisionError(ctx, collision)
	}
	if collision, ok := r.firstToolCollisionLocked(next); ok {
		r.mu.Unlock()
		return r.collisionError(ctx, collision)
	}

	r.providers[normalized] = next
	for _, toolName := range next.ToolNames {
		r.toolOwners[toolName] = normalized
	}
	r.mu.Unlock()
	return nil
}

// SetActive selects one registered provider for a workspace.
func (r *MemoryProviderRegistry) SetActive(ctx context.Context, workspaceID string, name string) error {
	if err := r.checkContext(ctx); err != nil {
		return err
	}
	normalized, err := normalizeMemoryProviderName(name)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.providers[normalized]; !ok {
		return &MemoryProviderNotFoundError{Name: normalized}
	}
	r.active[normalizeMemoryProviderWorkspace(workspaceID)] = normalized
	return nil
}

// Select returns the requested provider, or the active/default provider for a workspace.
func (r *MemoryProviderRegistry) Select(
	ctx context.Context,
	workspaceID string,
	name string,
) (MemoryProviderRegistration, error) {
	if err := r.checkContext(ctx); err != nil {
		return MemoryProviderRegistration{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	target := strings.TrimSpace(name)
	if target == "" {
		target = r.active[normalizeMemoryProviderWorkspace(workspaceID)]
	}
	if target == "" {
		target = defaultMemoryProviderName
	}
	normalized, err := normalizeMemoryProviderName(target)
	if err != nil {
		return MemoryProviderRegistration{}, err
	}
	registration, ok := r.providers[normalized]
	if !ok {
		return MemoryProviderRegistration{}, &MemoryProviderNotFoundError{Name: normalized}
	}
	return cloneMemoryProviderRegistration(registration), nil
}

// List returns registered providers ordered by canonical name.
func (r *MemoryProviderRegistry) List() []MemoryProviderRegistration {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	slices.Sort(names)
	registrations := make([]MemoryProviderRegistration, 0, len(names))
	for _, name := range names {
		registrations = append(registrations, cloneMemoryProviderRegistration(r.providers[name]))
	}
	return registrations
}

func (r *MemoryProviderRegistry) firstToolCollisionLocked(
	registration MemoryProviderRegistration,
) (MemoryProviderCollisionError, bool) {
	for _, toolName := range registration.ToolNames {
		if owner, ok := r.reservedTools[toolName]; ok {
			return MemoryProviderCollisionError{
				Name:              registration.Name,
				ExistingExtension: owner,
				IncomingExtension: registration.ExtensionName,
				Reason:            memoryProviderReservedToolName,
				ToolName:          toolName,
			}, true
		}
		if owner, ok := r.toolOwners[toolName]; ok {
			existing := r.providers[owner]
			return MemoryProviderCollisionError{
				Name:              registration.Name,
				ExistingExtension: existing.ExtensionName,
				IncomingExtension: registration.ExtensionName,
				Reason:            memoryProviderToolCollision,
				ToolName:          toolName,
			}, true
		}
	}
	return MemoryProviderCollisionError{}, false
}

func (r *MemoryProviderRegistry) collisionError(
	ctx context.Context,
	collision MemoryProviderCollisionError,
) error {
	err := &collision
	if recordErr := r.recordCollision(ctx, collision); recordErr != nil {
		return errors.Join(err, fmt.Errorf("extension: record memory provider collision: %w", recordErr))
	}
	return err
}

func (r *MemoryProviderRegistry) recordCollision(
	ctx context.Context,
	collision MemoryProviderCollisionError,
) error {
	if r.eventWriter == nil {
		return nil
	}
	occurredAt := r.now().UTC()
	payload := memoryProviderCollisionPayload{
		Provider:          collision.Name,
		ExistingExtension: collision.ExistingExtension,
		IncomingExtension: collision.IncomingExtension,
		Reason:            collision.Reason,
		ToolName:          collision.ToolName,
		OccurredAt:        occurredAt,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("extension: encode memory provider collision: %w", err)
	}
	return r.eventWriter.WriteEventSummary(ctx, store.EventSummary{
		Type:      memoryProviderCollisionEvent,
		Content:   content,
		Summary:   memoryProviderCollisionSummary,
		Timestamp: occurredAt,
	})
}

func (r *MemoryProviderRegistry) checkContext(ctx context.Context) error {
	if r == nil {
		return errors.New("extension: memory provider registry is required")
	}
	if ctx == nil {
		return errors.New("extension: memory provider registry context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("extension: memory provider registry context: %w", err)
	}
	return nil
}

func normalizeMemoryProviderRegistration(
	registration MemoryProviderRegistration,
	name string,
) MemoryProviderRegistration {
	return MemoryProviderRegistration{
		Name:          name,
		Version:       strings.TrimSpace(registration.Version),
		ExtensionName: strings.TrimSpace(registration.ExtensionName),
		Provider:      registration.Provider,
		ToolNames:     normalizeMemoryProviderToolNames(registration.ToolNames),
		Bundled:       registration.Bundled,
	}
}

func normalizeMemoryProviderName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("extension: memory provider name is required")
	}
	return strings.ToLower(trimmed), nil
}

func normalizeMemoryProviderToolNames(names []string) []string {
	normalized := make(map[string]struct{}, len(names))
	for _, name := range names {
		toolName := normalizeMemoryProviderToolName(name)
		if toolName == "" {
			continue
		}
		normalized[toolName] = struct{}{}
	}
	out := make([]string, 0, len(normalized))
	for name := range normalized {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func normalizeMemoryProviderToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeMemoryProviderWorkspace(workspaceID string) string {
	return strings.TrimSpace(workspaceID)
}

func cloneMemoryProviderRegistration(
	registration MemoryProviderRegistration,
) MemoryProviderRegistration {
	return MemoryProviderRegistration{
		Name:          registration.Name,
		Version:       registration.Version,
		ExtensionName: registration.ExtensionName,
		Provider:      registration.Provider,
		ToolNames:     append([]string(nil), registration.ToolNames...),
		Bundled:       registration.Bundled,
	}
}

// Error returns the provider collision message.
func (e *MemoryProviderCollisionError) Error() string {
	if e == nil {
		return ErrMemoryProviderCollision.Error()
	}
	if strings.TrimSpace(e.ToolName) != "" {
		return fmt.Sprintf(
			"%s: %s %q for provider %q",
			ErrMemoryProviderCollision,
			strings.TrimSpace(e.Reason),
			strings.TrimSpace(e.ToolName),
			strings.TrimSpace(e.Name),
		)
	}
	return fmt.Sprintf(
		"%s: %s for provider %q",
		ErrMemoryProviderCollision,
		strings.TrimSpace(e.Reason),
		strings.TrimSpace(e.Name),
	)
}

// Is matches sentinel errors for provider collisions.
func (e *MemoryProviderCollisionError) Is(target error) bool {
	return target == ErrMemoryProviderCollision
}

// Error returns the provider lookup message.
func (e *MemoryProviderNotFoundError) Error() string {
	if e == nil || strings.TrimSpace(e.Name) == "" {
		return ErrMemoryProviderNotFound.Error()
	}
	return fmt.Sprintf("%s: %s", ErrMemoryProviderNotFound, strings.TrimSpace(e.Name))
}

// Is matches sentinel errors for missing providers.
func (e *MemoryProviderNotFoundError) Is(target error) bool {
	return target == ErrMemoryProviderNotFound
}
