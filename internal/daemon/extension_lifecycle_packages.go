package daemon

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"golang.org/x/sync/semaphore"
)

const exclusiveExtensionPackageWeight = int64(math.MaxInt64)

type extensionPackageLock struct {
	semaphore *semaphore.Weighted
	refs      int
}

func (c *extensionLifecycleCoordinator) withPackageMutation(
	ctx context.Context,
	names []string,
	fn func() error,
) error {
	keys := make([]extensionpkg.InstanceKey, 0, len(names))
	for _, name := range normalizeLifecycleNames(names) {
		keys = append(keys, extensionpkg.GlobalInstanceKey(name))
	}
	return c.withMutation(ctx, keys, exclusiveExtensionPackageWeight, fn)
}

func (c *extensionLifecycleCoordinator) withPackages(
	ctx context.Context,
	identities []string,
	weight int64,
	fn func() error,
) error {
	names := make([]string, 0, len(identities))
	for _, identity := range identities {
		name, _, _ := strings.Cut(identity, "\x00")
		names = append(names, name)
	}
	names = normalizeLifecycleNames(names)
	acquired := make([]*extensionPackageLock, 0, len(names))
	defer func() {
		for i, entry := range slices.Backward(acquired) {
			entry.semaphore.Release(weight)
			c.releasePackage(names[i], entry)
		}
	}()
	for _, name := range names {
		entry := c.retainPackage(name)
		if err := entry.semaphore.Acquire(ctx, weight); err != nil {
			c.releasePackage(name, entry)
			return fmt.Errorf("daemon: wait for extension package %q: %w", name, err)
		}
		acquired = append(acquired, entry)
	}
	return fn()
}

func (c *extensionLifecycleCoordinator) retainPackage(name string) *extensionPackageLock {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.packages == nil {
		c.packages = make(map[string]*extensionPackageLock)
	}
	entry := c.packages[name]
	if entry == nil {
		entry = &extensionPackageLock{semaphore: semaphore.NewWeighted(exclusiveExtensionPackageWeight)}
		c.packages[name] = entry
	}
	entry.refs++
	c.notifyLocked()
	return entry
}

func (c *extensionLifecycleCoordinator) releasePackage(name string, entry *extensionPackageLock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(c.packages, name)
	}
	c.notifyLocked()
}
