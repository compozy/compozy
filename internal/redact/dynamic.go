package redact

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
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
	mu           sync.Mutex
	values       map[string]secretRegistration
	registration uint64
	snapshot     atomic.Pointer[SecretSnapshot]
}

type secretRegistration struct {
	references int
	sequence   uint64
}

// SecretSnapshot pairs the registered secrets with their reversible encodings,
// derived once per change because redaction reads them for every string.
type SecretSnapshot struct {
	fingerprint [sha256.Size]byte
	values      []string
	encodings   []string
}

func newDynamicSecretRegistry() *dynamicSecretRegistry {
	registry := &dynamicSecretRegistry{values: make(map[string]secretRegistration)}
	registry.snapshot.Store(&SecretSnapshot{fingerprint: sha256.Sum256(nil)})
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
	entry := r.values[secret]
	if entry.references > 0 {
		entry.references++
		r.values[secret] = entry
		return
	}
	r.registration++
	r.values[secret] = secretRegistration{references: 1, sequence: r.registration}
	r.storeSnapshotLocked()
}

func (r *dynamicSecretRegistry) unregister(secret string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, present := r.values[secret]
	if !present {
		return
	}
	if entry.references > 1 {
		entry.references--
		r.values[secret] = entry
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
	// An unrelated temporary registration can disappear without changing the lifetime of any retained secret.
	registrations := make([]byte, 0, len(secrets)*8)
	for _, secret := range secrets {
		registrations = binary.LittleEndian.AppendUint64(registrations, r.values[secret].sequence)
	}
	r.snapshot.Store(&SecretSnapshot{
		values: secrets, encodings: secretEncodings(secrets),
		fingerprint: sha256.Sum256(registrations),
	})
}

func (r *dynamicSecretRegistry) valuesSnapshot() []string {
	return r.snapshot.Load().values
}

func (r *dynamicSecretRegistry) encodingsSnapshot() []string {
	return r.snapshot.Load().encodings
}

// RegisteredSecretSnapshot captures one immutable set of registered secrets for redaction.
func RegisteredSecretSnapshot() *SecretSnapshot { return dynamicSecrets.snapshot.Load() }

// SecretFingerprint identifies the current continuous registrations without exposing their values.
func SecretFingerprint() [sha256.Size]byte { return RegisteredSecretSnapshot().fingerprint }
