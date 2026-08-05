package redact

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	minDynamicSecretLength  = 8
	minRequiredSecretLength = 1
)

var dynamicSecrets = newDynamicSecretRegistry()

type dynamicSecretRegistry struct {
	mu       sync.Mutex
	values   map[string]int
	snapshot atomic.Value
}

func newDynamicSecretRegistry() *dynamicSecretRegistry {
	registry := &dynamicSecretRegistry{values: make(map[string]int)}
	registry.snapshot.Store([]string(nil))
	return registry
}

// RegisterDynamicSecret registers heuristic runtime material and returns an idempotent cleanup.
func RegisterDynamicSecret(value string) func() {
	return registerSecret(value, minDynamicSecretLength)
}

// RegisterRequiredSecret registers nonblank caller-classified secret material and returns an idempotent cleanup.
func RegisterRequiredSecret(value string) func() {
	return registerSecret(value, minRequiredSecretLength)
}

func registerSecret(value string, minimumLength int) func() {
	secret := strings.TrimSpace(value)
	if len(secret) < minimumLength {
		return func() {}
	}
	dynamicSecrets.register(secret)

	return sync.OnceFunc(func() {
		dynamicSecrets.unregister(secret)
	})
}

func (r *dynamicSecretRegistry) register(secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[secret]++
	r.storeSnapshotLocked()
}

func (r *dynamicSecretRegistry) unregister(secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values[secret] <= 1 {
		delete(r.values, secret)
	} else {
		r.values[secret]--
	}
	r.storeSnapshotLocked()
}

func (r *dynamicSecretRegistry) storeSnapshotLocked() {
	secrets := make([]string, 0, len(r.values))
	for secret := range r.values {
		secrets = append(secrets, secret)
	}
	sort.Slice(secrets, func(i int, j int) bool {
		if len(secrets[i]) == len(secrets[j]) {
			return secrets[i] < secrets[j]
		}
		return len(secrets[i]) > len(secrets[j])
	})
	r.snapshot.Store(secrets)
}

func (r *dynamicSecretRegistry) valuesSnapshot() []string {
	secrets, ok := r.snapshot.Load().([]string)
	if !ok {
		return nil
	}
	return secrets
}
