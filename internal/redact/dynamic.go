package redact

import (
	"cmp"
	"crypto/sha256"
	"slices"
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
	snapshot atomic.Pointer[secretSnapshot]
}

// secretSnapshot pairs the registered secrets with their reversible encodings,
// derived once per change because redaction reads them for every string.
type secretSnapshot struct {
	fingerprint [sha256.Size]byte
	values      []string
	encodings   []string
}

func newDynamicSecretRegistry() *dynamicSecretRegistry {
	registry := &dynamicSecretRegistry{values: make(map[string]int)}
	registry.snapshot.Store(&secretSnapshot{})
	return registry
}

// ContainsRegisteredSecret reports whether value contains runtime-registered secret material.
func ContainsRegisteredSecret(value string) bool {
	for _, secret := range dynamicSecrets.valuesSnapshot() {
		if strings.Contains(value, secret) {
			return true
		}
	}
	return false
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
	if r.values[secret] == 1 {
		r.storeSnapshotLocked()
	}
}

func (r *dynamicSecretRegistry) unregister(secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values[secret] > 1 {
		r.values[secret]--
		return
	}
	if r.values[secret] == 0 {
		return
	}
	delete(r.values, secret)
	r.storeSnapshotLocked()
}

func (r *dynamicSecretRegistry) storeSnapshotLocked() {
	secrets := make([]string, 0, len(r.values))
	for secret := range r.values {
		secrets = append(secrets, secret)
	}
	slices.SortFunc(secrets, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a, b))
	})
	r.snapshot.Store(&secretSnapshot{
		values: secrets, encodings: secretEncodings(secrets),
		fingerprint: sha256.Sum256([]byte(strings.Join(secrets, "\x00"))),
	})
}

func (r *dynamicSecretRegistry) valuesSnapshot() []string {
	return r.snapshot.Load().values
}

func (r *dynamicSecretRegistry) encodingsSnapshot() []string {
	return r.snapshot.Load().encodings
}

// SecretFingerprint identifies the effective registered secret set without exposing it.
func SecretFingerprint() [sha256.Size]byte { return dynamicSecrets.snapshot.Load().fingerprint }
