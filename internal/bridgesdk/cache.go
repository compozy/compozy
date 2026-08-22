package bridgesdk

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"
	"github.com/compozy/compozy/internal/subprocess"
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
	managed        map[instanceCacheKey]subprocess.InitializeBridgeManagedInstance
}

type instanceCacheKey struct {
	ProfileID  string
	InstanceID string
}

// NewInstanceCache constructs a cache seeded from the negotiated bridge runtime.
func NewInstanceCache(runtime *subprocess.InitializeBridgeRuntime) *InstanceCache {
	cache := &InstanceCache{
		managed: make(map[instanceCacheKey]subprocess.InitializeBridgeManagedInstance),
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

	c.managed = make(map[instanceCacheKey]subprocess.InitializeBridgeManagedInstance)
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
		c.managed[managedInstanceCacheKey(managed.Instance)] = managed
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
	for _, key := range c.keysLocked() {
		runtime.ManagedInstances = append(runtime.ManagedInstances, cloneManagedInstance(c.managed[key]))
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

	key, ok := c.uniqueInstanceKeyLocked(id)
	if !ok {
		return nil, false
	}
	managed, ok := c.managed[key]
	if !ok {
		return nil, false
	}
	cloned := cloneManagedInstance(managed)
	return &cloned, true
}

// GetForProfile returns one managed instance owned by the exact profile and id.
func (c *InstanceCache) GetForProfile(
	profileID string,
	instanceID string,
) (*subprocess.InitializeBridgeManagedInstance, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	managed, ok := c.managed[instanceCacheKey{
		ProfileID:  strings.TrimSpace(profileID),
		InstanceID: strings.TrimSpace(instanceID),
	}]
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
	for _, key := range c.keysLocked() {
		items = append(items, cloneManagedInstance(c.managed[key]))
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

	key, ok := c.uniqueInstanceKeyLocked(instanceID)
	if !ok {
		return "", false
	}
	return boundSecretValue(c.managed[key], bindingName)
}

// BoundSecretValueForProfile returns one secret only for the exact profile-owned instance.
func (c *InstanceCache) BoundSecretValueForProfile(
	profileID string,
	instanceID string,
	bindingName string,
) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	managed, ok := c.managed[instanceCacheKey{
		ProfileID:  strings.TrimSpace(profileID),
		InstanceID: strings.TrimSpace(instanceID),
	}]
	if !ok {
		return "", false
	}
	return boundSecretValue(managed, bindingName)
}

func boundSecretValue(
	managed subprocess.InitializeBridgeManagedInstance,
	bindingName string,
) (string, bool) {
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

	next := make(map[instanceCacheKey]subprocess.InitializeBridgeManagedInstance, len(instances))
	for _, instance := range instances {
		managed := subprocess.InitializeBridgeManagedInstance{Instance: instance}
		key := managedInstanceCacheKey(instance)
		if existing, ok := c.managed[key]; ok {
			managed.BoundSecrets = append([]subprocess.InitializeBridgeBoundSecret(nil), existing.BoundSecrets...)
		}
		next[key] = managed
	}
	c.managed = next

	items := make([]subprocess.InitializeBridgeManagedInstance, 0, len(c.managed))
	for _, key := range c.keysLocked() {
		items = append(items, cloneManagedInstance(c.managed[key]))
	}
	return items, nil
}

func (c *InstanceCache) keysLocked() []instanceCacheKey {
	keys := make([]instanceCacheKey, 0, len(c.managed))
	for key := range c.managed {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool {
		if keys[left].ProfileID == keys[right].ProfileID {
			return keys[left].InstanceID < keys[right].InstanceID
		}
		return keys[left].ProfileID < keys[right].ProfileID
	})
	return keys
}

func (c *InstanceCache) uniqueInstanceKeyLocked(instanceID string) (instanceCacheKey, bool) {
	trimmedID := strings.TrimSpace(instanceID)
	var match instanceCacheKey
	found := false
	for key := range c.managed {
		if key.InstanceID != trimmedID {
			continue
		}
		if found {
			return instanceCacheKey{}, false
		}
		match = key
		found = true
	}
	return match, found
}

func managedInstanceCacheKey(instance bridgepkg.BridgeInstance) instanceCacheKey {
	return instanceCacheKey{
		ProfileID:  strings.TrimSpace(instance.ProfileID),
		InstanceID: strings.TrimSpace(instance.ID),
	}
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
