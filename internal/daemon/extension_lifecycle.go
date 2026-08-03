package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/api/contract"
)

// extensionLifecycleCoordinator serializes complete mutations for one extension
// name. The exclusive gate covers install flows whose authored name is not known
// until their marketplace artifact has been verified.
type extensionLifecycleCoordinator struct {
	gate    *lifecycleRWGate
	mu      sync.Mutex
	entries map[string]*extensionLifecycleEntry
}

type extensionLifecycleEntry struct {
	refs int
	lock chan struct{}
}

func newExtensionLifecycleCoordinator() *extensionLifecycleCoordinator {
	return &extensionLifecycleCoordinator{
		gate:    &lifecycleRWGate{},
		entries: make(map[string]*extensionLifecycleEntry),
	}
}

func (c *extensionLifecycleCoordinator) withName(
	ctx context.Context,
	name string,
	fn func() error,
) error {
	if c == nil {
		return errors.New("daemon: extension lifecycle coordinator is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("daemon: extension lifecycle name is required")
	}
	if fn == nil {
		return errors.New("daemon: extension lifecycle mutation is required")
	}
	if ctx == nil {
		return errors.New("daemon: extension lifecycle context is required")
	}
	unlockGate, err := c.gate.lock(ctx, true)
	if err != nil {
		return fmt.Errorf("daemon: wait for shared extension lifecycle: %w", err)
	}
	defer unlockGate()
	entry := c.retain(name)
	defer c.release(name, entry)
	select {
	case <-ctx.Done():
		return fmt.Errorf("daemon: wait for extension lifecycle %q: %w", name, ctx.Err())
	case <-entry.lock:
	}
	defer func() { entry.lock <- struct{}{} }()
	return fn()
}

func (c *extensionLifecycleCoordinator) withNames(
	ctx context.Context,
	names []string,
	fn func() error,
) error {
	if c == nil {
		return errors.New("daemon: extension lifecycle coordinator is required")
	}
	if fn == nil {
		return errors.New("daemon: extension lifecycle mutation is required")
	}
	if ctx == nil {
		return errors.New("daemon: extension lifecycle context is required")
	}
	normalized := normalizeLifecycleNames(names)
	if len(normalized) == 0 {
		return fn()
	}
	unlockGate, err := c.gate.lock(ctx, true)
	if err != nil {
		return fmt.Errorf("daemon: wait for shared extension lifecycle: %w", err)
	}
	defer unlockGate()
	entries := make([]*extensionLifecycleEntry, 0, len(normalized))
	for _, name := range normalized {
		entries = append(entries, c.retain(name))
	}
	defer func() {
		for idx, name := range normalized {
			c.release(name, entries[idx])
		}
	}()
	acquired := 0
	defer func() {
		for idx := acquired - 1; idx >= 0; idx-- {
			entries[idx].lock <- struct{}{}
		}
	}()
	for idx, entry := range entries {
		select {
		case <-ctx.Done():
			return fmt.Errorf("daemon: wait for extension lifecycle %q: %w", normalized[idx], ctx.Err())
		case <-entry.lock:
			acquired++
		}
	}
	return fn()
}

func (c *extensionLifecycleCoordinator) exclusive(ctx context.Context, fn func() error) error {
	if c == nil {
		return errors.New("daemon: extension lifecycle coordinator is required")
	}
	if fn == nil {
		return errors.New("daemon: extension lifecycle mutation is required")
	}
	if ctx == nil {
		return errors.New("daemon: extension lifecycle context is required")
	}
	unlockGate, err := c.gate.lock(ctx, false)
	if err != nil {
		return fmt.Errorf("daemon: wait for exclusive extension lifecycle: %w", err)
	}
	defer unlockGate()
	return fn()
}

func (c *extensionLifecycleCoordinator) retain(name string) *extensionLifecycleEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]*extensionLifecycleEntry)
	}
	entry := c.entries[name]
	if entry == nil {
		entry = &extensionLifecycleEntry{lock: make(chan struct{}, 1)}
		entry.lock <- struct{}{}
		c.entries[name] = entry
	}
	entry.refs++
	return entry
}

func (c *extensionLifecycleCoordinator) release(name string, entry *extensionLifecycleEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(c.entries, name)
	}
}

func normalizeLifecycleNames(names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		result = append(result, name)
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func (s *daemonExtensionService) lifecycleUpdateNames(req contract.UpdateExtensionsRequest) ([]string, error) {
	if !req.All {
		return normalizeLifecycleNames(req.Names), nil
	}
	infos, err := s.registry.List()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(infos))
	for _, info := range infos {
		names = append(names, info.Name)
	}
	return normalizeLifecycleNames(names), nil
}
