package redact

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const minDynamicSecretLength = 8

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

// RegisterDynamicSecret includes runtime-resolved material in subsequent redaction calls.
// The returned cleanup is idempotent and removes one registration reference.
func RegisterDynamicSecret(value string) func() {
	secret := strings.TrimSpace(value)
	if len(secret) < minDynamicSecretLength {
		return func() {}
	}
	dynamicSecrets.register(secret)

	var once sync.Once
	return func() {
		once.Do(func() {
			dynamicSecrets.unregister(secret)
		})
	}
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
