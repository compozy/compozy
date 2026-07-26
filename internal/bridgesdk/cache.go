package bridgesdk

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

// InstanceCache keeps the provider-owned managed-instance snapshot locally,
// preserving launch-time bound secret material across Host API syncs.
type InstanceCache struct {
	mu             sync.RWMutex
	runtimeVersion string
	purpose        subprocess.BridgeRuntimePurpose
	provider       string
	platform       string
	allowedMethods []string
	managed        map[string]subprocess.InitializeBridgeManagedInstance
}

// NewInstanceCache constructs a cache seeded from the negotiated bridge runtime.
func NewInstanceCache(runtime *subprocess.InitializeBridgeRuntime) *InstanceCache {
	cache := &InstanceCache{
		managed: make(map[string]subprocess.InitializeBridgeManagedInstance),
	}
	cache.Reset(runtime)
	return cache
}

// Reset replaces the managed-instance snapshot with the provided runtime grant.
func (c *InstanceCache) Reset(runtime *subprocess.InitializeBridgeRuntime) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.managed = make(map[string]subprocess.InitializeBridgeManagedInstance)
	c.runtimeVersion = ""
	c.purpose = ""
	c.provider = ""
	c.platform = ""
	c.allowedMethods = nil

	if runtime == nil {
		return
	}

	cloned := subprocess.CloneInitializeBridgeRuntime(runtime)
	if cloned == nil {
		return
	}

	c.runtimeVersion = cloned.RuntimeVersion
	c.purpose = cloned.Purpose
	c.provider = cloned.Provider
	c.platform = cloned.Platform
	c.allowedMethods = append([]string(nil), cloned.AllowedMethods...)
	for _, managed := range cloned.ManagedInstances {
		c.managed[strings.TrimSpace(managed.Instance.ID)] = managed
	}
}

// Snapshot returns the current managed-runtime snapshot.
func (c *InstanceCache) Snapshot() *subprocess.InitializeBridgeRuntime {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	runtime := &subprocess.InitializeBridgeRuntime{
		RuntimeVersion:   c.runtimeVersion,
		Purpose:          c.purpose,
		Provider:         c.provider,
		Platform:         c.platform,
		AllowedMethods:   append([]string(nil), c.allowedMethods...),
		ManagedInstances: make([]subprocess.InitializeBridgeManagedInstance, 0, len(c.managed)),
	}
	for _, id := range c.idsLocked() {
		runtime.ManagedInstances = append(runtime.ManagedInstances, cloneManagedInstance(c.managed[id]))
	}
	return runtime
}

// Get returns one managed instance snapshot by id.
func (c *InstanceCache) Get(id string) (*subprocess.InitializeBridgeManagedInstance, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	managed, ok := c.managed[strings.TrimSpace(id)]
	if !ok {
		return nil, false
	}
	cloned := cloneManagedInstance(managed)
	return &cloned, true
}

// List returns every managed instance snapshot in stable id order.
func (c *InstanceCache) List() []subprocess.InitializeBridgeManagedInstance {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make([]subprocess.InitializeBridgeManagedInstance, 0, len(c.managed))
	for _, id := range c.idsLocked() {
		items = append(items, cloneManagedInstance(c.managed[id]))
	}
	return items
}

// BoundSecretValue returns one launch-time bound secret value for the managed instance.
func (c *InstanceCache) BoundSecretValue(instanceID string, bindingName string) (string, bool) {
	if c == nil {
		return "", false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	managed, ok := c.managed[strings.TrimSpace(instanceID)]
	if !ok {
		return "", false
	}
	trimmedName := strings.TrimSpace(bindingName)
	for _, secret := range managed.BoundSecrets {
		if strings.TrimSpace(secret.BindingName) != trimmedName {
			continue
		}
		return secret.Value, true
	}
	return "", false
}

// Sync refreshes the provider-owned instance state from the Host API while preserving
// launch-time bound secrets for instances that were already hydrated at initialize time.
func (c *InstanceCache) Sync(
	ctx context.Context,
	host *HostAPIClient,
) ([]subprocess.InitializeBridgeManagedInstance, error) {
	if c == nil {
		return nil, errors.New("bridgesdk: instance cache is required")
	}
	if host == nil {
		return nil, errors.New("bridgesdk: host api client is required")
	}

	instances, err := host.ListBridgeInstances(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	next := make(map[string]subprocess.InitializeBridgeManagedInstance, len(instances))
	for _, instance := range instances {
		managed := subprocess.InitializeBridgeManagedInstance{Instance: instance}
		if existing, ok := c.managed[strings.TrimSpace(instance.ID)]; ok {
			managed.BoundSecrets = append([]subprocess.InitializeBridgeBoundSecret(nil), existing.BoundSecrets...)
		}
		next[strings.TrimSpace(instance.ID)] = managed
	}
	c.managed = next

	items := make([]subprocess.InitializeBridgeManagedInstance, 0, len(c.managed))
	for _, id := range c.idsLocked() {
		items = append(items, cloneManagedInstance(c.managed[id]))
	}
	return items, nil
}

func (c *InstanceCache) idsLocked() []string {
	ids := make([]string, 0, len(c.managed))
	for id := range c.managed {
		ids = append(ids, id)
	}
	slicesSortStrings(ids)
	return ids
}

func cloneManagedInstance(src subprocess.InitializeBridgeManagedInstance) subprocess.InitializeBridgeManagedInstance {
	cloned := src
	cloned.Instance = cloneBridgeInstance(cloned.Instance)
	cloned.BoundSecrets = append([]subprocess.InitializeBridgeBoundSecret(nil), cloned.BoundSecrets...)
	return cloned
}

func cloneBridgeInstance(instance bridgepkg.BridgeInstance) bridgepkg.BridgeInstance {
	cloned := instance
	if len(cloned.ProviderConfig) > 0 {
		cloned.ProviderConfig = append([]byte(nil), cloned.ProviderConfig...)
	}
	if len(cloned.DeliveryDefaults) > 0 {
		cloned.DeliveryDefaults = append([]byte(nil), cloned.DeliveryDefaults...)
	}
	if cloned.Degradation != nil {
		degradation := *cloned.Degradation
		cloned.Degradation = &degradation
	}
	return cloned
}

func slicesSortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	sort.Strings(values)
}
