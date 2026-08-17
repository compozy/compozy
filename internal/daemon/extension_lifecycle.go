package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

// extensionLifecycleCoordinator serializes complete mutations for one extension
// instance while allowing unrelated instances to proceed independently.
type extensionLifecycleCoordinator struct {
	mu      sync.Mutex
	changed chan struct{}
	entries map[string]*extensionLifecycleEntry
}

type extensionLifecycleEntry struct {
	refs int
	lock chan struct{}
}

func newExtensionLifecycleCoordinator() *extensionLifecycleCoordinator {
	return &extensionLifecycleCoordinator{
		changed: make(chan struct{}),
		entries: make(map[string]*extensionLifecycleEntry),
	}
}

func (c *extensionLifecycleCoordinator) withName(
	ctx context.Context,
	name string,
	fn func() error,
) error {
	return c.withInstance(ctx, extensionpkg.GlobalInstanceKey(name), fn)
}

func (c *extensionLifecycleCoordinator) withInstance(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	fn func() error,
) error {
	if c == nil {
		return errors.New("daemon: extension lifecycle coordinator is required")
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	if fn == nil {
		return errors.New("daemon: extension lifecycle mutation is required")
	}
	if ctx == nil {
		return errors.New("daemon: extension lifecycle context is required")
	}
	identity := lifecycleInstanceIdentity(key)
	entry := c.retain(identity)
	defer c.release(identity, entry)
	select {
	case <-ctx.Done():
		return fmt.Errorf("daemon: wait for extension lifecycle %q: %w", identity, ctx.Err())
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
	keys := make([]extensionpkg.InstanceKey, 0, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) != "" {
			keys = append(keys, extensionpkg.GlobalInstanceKey(name))
		}
	}
	return c.withInstances(ctx, keys, fn)
}

func (c *extensionLifecycleCoordinator) withInstances(
	ctx context.Context,
	keys []extensionpkg.InstanceKey,
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
	identities, err := normalizeLifecycleInstances(keys)
	if err != nil {
		return err
	}
	if len(identities) == 0 {
		return fn()
	}
	entries := make([]*extensionLifecycleEntry, 0, len(identities))
	for _, identity := range identities {
		entries = append(entries, c.retain(identity))
	}
	defer func() {
		for idx, identity := range identities {
			c.release(identity, entries[idx])
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
			return fmt.Errorf("daemon: wait for extension lifecycle %q: %w", identities[idx], ctx.Err())
		case <-entry.lock:
			acquired++
		}
	}
	return fn()
}

func normalizeLifecycleInstances(keys []extensionpkg.InstanceKey) ([]string, error) {
	identities := make([]string, 0, len(keys))
	for _, key := range keys {
		key = key.Normalize()
		if err := key.Validate(); err != nil {
			return nil, err
		}
		identities = append(identities, lifecycleInstanceIdentity(key))
	}
	slices.Sort(identities)
	return slices.Compact(identities), nil
}

func lifecycleInstanceIdentity(key extensionpkg.InstanceKey) string {
	key = key.Normalize()
	if key.WorkspaceID == "" {
		return key.Name
	}
	return key.Name + "\x00" + key.WorkspaceID
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
	c.notifyLocked()
	return entry
}

func (c *extensionLifecycleCoordinator) release(name string, entry *extensionLifecycleEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(c.entries, name)
	}
	c.notifyLocked()
}

func (c *extensionLifecycleCoordinator) notifyLocked() {
	if c.changed == nil {
		c.changed = make(chan struct{})
	}
	close(c.changed)
	c.changed = make(chan struct{})
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
