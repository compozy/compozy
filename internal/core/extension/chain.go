package extensions

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// ExtensionCaller abstracts a transport capable of issuing one JSON-RPC-style
// request to an extension subprocess.
type ExtensionCaller interface {
	Call(ctx context.Context, method string, params any, result any) error
}

// ExtensionState captures the runtime lifecycle phase relevant to dispatcher and
// Host API routing.
type ExtensionState string

const (
	// ExtensionStateLoaded identifies a session that exists but is not ready yet.
	ExtensionStateLoaded ExtensionState = "loaded"
	// ExtensionStateInitializing identifies a session that is handshaking.
	ExtensionStateInitializing ExtensionState = "initializing"
	// ExtensionStateReady identifies a session that can accept operational calls.
	ExtensionStateReady ExtensionState = "ready"
	// ExtensionStateDraining identifies a session that is shutting down.
	ExtensionStateDraining ExtensionState = "draining"
	// ExtensionStateDegraded identifies a session that stays alive with partial service.
	ExtensionStateDegraded ExtensionState = "degraded"
	// ExtensionStateStopped identifies a session that is no longer serving requests.
	ExtensionStateStopped ExtensionState = "stopped"
)

// RuntimeExtension stores the runtime metadata required by the dispatcher and
// Host API router for one extension session.
type RuntimeExtension struct {
	Name         string
	Ref          Ref
	Manifest     *Manifest
	ManifestPath string
	ExtensionDir string
	Caller       ExtensionCaller
	Capabilities CapabilityChecker

	mu               sync.RWMutex
	state            ExtensionState
	shutdownDeadline time.Duration
}

// State reports the current runtime state.
func (e *RuntimeExtension) State() ExtensionState {
	if e == nil {
		return ExtensionStateStopped
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.state
}

// SetState updates the current runtime state.
func (e *RuntimeExtension) SetState(state ExtensionState) {
	if e == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.state = state
}

// ShutdownDeadline reports the currently configured graceful-shutdown deadline.
func (e *RuntimeExtension) ShutdownDeadline() time.Duration {
	if e == nil {
		return 0
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.shutdownDeadline > 0 {
		return e.shutdownDeadline
	}
	if e.Manifest != nil && e.Manifest.Subprocess != nil {
		return e.Manifest.Subprocess.ShutdownTimeout
	}
	return 0
}

// SetShutdownDeadline overrides the shutdown deadline used for router gating.
func (e *RuntimeExtension) SetShutdownDeadline(deadline time.Duration) {
	if e == nil {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.shutdownDeadline = deadline
}

func (e *RuntimeExtension) normalizedName() string {
	if e == nil {
		return ""
	}

	if name := strings.TrimSpace(e.Name); name != "" {
		return name
	}
	if name := strings.TrimSpace(e.Ref.Name); name != "" {
		return name
	}
	if e.Manifest != nil {
		return strings.TrimSpace(e.Manifest.Extension.Name)
	}
	return ""
}

// Registry stores runtime extensions by name for dispatcher and router lookup.
type Registry struct {
	mu         sync.RWMutex
	extensions map[string]*RuntimeExtension
}

// NewRegistry constructs a runtime registry from the provided entries.
func NewRegistry(extensions ...*RuntimeExtension) (*Registry, error) {
	registry := &Registry{
		extensions: make(map[string]*RuntimeExtension, len(extensions)),
	}
	for _, extension := range extensions {
		if err := registry.Add(extension); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// Add inserts one runtime extension into the registry.
func (r *Registry) Add(extension *RuntimeExtension) error {
	if r == nil {
		return fmt.Errorf("add runtime extension: registry is nil")
	}
	if extension == nil {
		return fmt.Errorf("add runtime extension: extension is nil")
	}

	name := extension.normalizedName()
	if name == "" {
		return fmt.Errorf("add runtime extension: extension name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.extensions == nil {
		r.extensions = make(map[string]*RuntimeExtension)
	}
	if _, exists := r.extensions[name]; exists {
		return fmt.Errorf("add runtime extension: duplicate extension %q", name)
	}

	extension.Name = name
	r.extensions[name] = extension
	return nil
}

// Get looks up one runtime extension by name.
func (r *Registry) Get(name string) (*RuntimeExtension, bool) {
	if r == nil {
		return nil, false
	}

	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	extension, ok := r.extensions[normalized]
	return extension, ok
}

// Extensions returns all registered runtime extensions in deterministic name order.
func (r *Registry) Extensions() []*RuntimeExtension {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	extensions := make([]*RuntimeExtension, 0, len(r.extensions))
	for _, extension := range r.extensions {
		extensions = append(extensions, extension)
	}

	slices.SortFunc(extensions, func(left, right *RuntimeExtension) int {
		return strings.Compare(left.normalizedName(), right.normalizedName())
	})
	return extensions
}

type hookChainEntry struct {
	extension *RuntimeExtension
	hook      HookDeclaration
}

func buildHookChains(registry *Registry) map[HookName][]hookChainEntry {
	chains := make(map[HookName][]hookChainEntry)
	if registry == nil {
		return chains
	}

	for _, extension := range registry.Extensions() {
		if extension == nil || extension.Manifest == nil {
			continue
		}

		for _, hook := range extension.Manifest.Hooks {
			event := HookName(strings.TrimSpace(string(hook.Event)))
			if event == "" {
				continue
			}
			chains[event] = append(chains[event], hookChainEntry{
				extension: extension,
				hook:      hook,
			})
		}
	}

	for event := range chains {
		slices.SortFunc(chains[event], func(left, right hookChainEntry) int {
			if left.hook.Priority != right.hook.Priority {
				if left.hook.Priority < right.hook.Priority {
					return -1
				}
				return 1
			}
			return strings.Compare(left.extension.normalizedName(), right.extension.normalizedName())
		})
	}

	return chains
}
