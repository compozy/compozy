package gateway

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/remotehost"
)

const pairingManualCodeAttempts = 8

type pairingEntry struct {
	expiresAt time.Time
	source    PairingSource
	spent     bool
}

// pairingStore is a bounded, process-local set of single-use pairing artifacts.
type pairingStore struct {
	mu sync.Mutex
	// entries is keyed by the hashed opaque artifact digest.
	entries map[string]pairingEntry
	// manual maps a hashed 6-digit code to the artifact digest of the same entry.
	manual     map[string]string
	maxPending int
	ttl        time.Duration
	now        func() time.Time
	entropy    io.Reader
}

func newPairingStore(
	maxPending int,
	ttl time.Duration,
	now func() time.Time,
	entropy io.Reader,
) (*pairingStore, error) {
	if maxPending <= 0 {
		return nil, errors.New("gateway: pairing max pending must be positive")
	}
	if ttl <= 0 {
		return nil, errors.New("gateway: pairing TTL must be positive")
	}
	return &pairingStore{
		entries: make(map[string]pairingEntry), manual: make(map[string]string),
		maxPending: maxPending, ttl: ttl, now: now, entropy: entropy,
	}, nil
}

func (p *pairingStore) reconfigure(maxPending int, ttl time.Duration) error {
	if maxPending <= 0 {
		return errors.New("gateway: pairing max pending must be positive")
	}
	if ttl <= 0 {
		return errors.New("gateway: pairing TTL must be positive")
	}
	p.mu.Lock()
	p.sweepExpired(p.now())
	p.maxPending = maxPending
	p.ttl = ttl
	p.mu.Unlock()
	return nil
}

func (p *pairingStore) Mint(source PairingSource) (PairingArtifact, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	p.sweepExpired(now)
	p.evictSpentUntilBelowLimit()
	if p.pendingCount() >= p.maxPending {
		return PairingArtifact{}, ErrPairingLimit
	}
	artifact, err := generatePairingArtifact(p.entropy)
	if err != nil {
		return PairingArtifact{}, fmt.Errorf("gateway: generate pairing artifact: %w", err)
	}
	manualCode, err := p.mintManualCodeLocked()
	if err != nil {
		return PairingArtifact{}, err
	}
	expiresAt := now.Add(p.ttl)
	digest := remotehost.HashCredential(artifact)
	p.entries[digest] = pairingEntry{expiresAt: expiresAt, source: source}
	p.manual[remotehost.HashCredential(manualCode)] = digest
	return PairingArtifact{
		Artifact: artifact, ManualCode: manualCode, ExpiresAt: expiresAt,
	}, nil
}

func (p *pairingStore) mintManualCodeLocked() (string, error) {
	for range pairingManualCodeAttempts {
		code, err := generatePairingManualCode(p.entropy)
		if err != nil {
			return "", err
		}
		if _, collides := p.manual[remotehost.HashCredential(code)]; collides {
			continue
		}
		return code, nil
	}
	return "", errors.New("gateway: pairing manual code collided with every pending code")
}

func (p *pairingStore) evictSpentUntilBelowLimit() {
	for digest, entry := range p.entries {
		if len(p.entries) < p.maxPending {
			return
		}
		if entry.spent {
			p.deleteEntryLocked(digest)
		}
	}
}

func (p *pairingStore) pendingCount() int {
	pending := 0
	for _, entry := range p.entries {
		if !entry.spent {
			pending++
		}
	}
	return pending
}

// Redeem holds the single-use decision through the durable device write.
func (p *pairingStore) Redeem(artifact, manualCode string, issue func(PairingSource) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	digest, ok := p.resolveLocked(artifact, manualCode)
	if !ok {
		return ErrPairingInvalid
	}
	entry, ok := p.entries[digest]
	if !ok {
		return ErrPairingInvalid
	}
	if !p.now().Before(entry.expiresAt) {
		p.deleteEntryLocked(digest)
		return ErrPairingExpired
	}
	if entry.spent {
		return ErrPairingSpent
	}
	if err := issue(entry.source); err != nil {
		return fmt.Errorf("gateway: issue paired device: %w", err)
	}
	entry.spent = true
	p.entries[digest] = entry
	return nil
}

// resolveLocked accepts the opaque artifact or the hand-typed code for one pending entry.
func (p *pairingStore) resolveLocked(artifact, manualCode string) (string, bool) {
	if candidate := strings.TrimSpace(artifact); remotehost.ValidOpaqueSecret(candidate, pairingArtifactPrefix) {
		return remotehost.HashCredential(candidate), true
	}
	normalized := NormalizePairingManualCode(manualCode)
	if !validPairingManualCode(normalized) {
		return "", false
	}
	digest, exists := p.manual[remotehost.HashCredential(normalized)]
	return digest, exists
}

func (p *pairingStore) sweepExpired(now time.Time) {
	for digest, entry := range p.entries {
		if !now.Before(entry.expiresAt) {
			p.deleteEntryLocked(digest)
		}
	}
}

// deleteEntryLocked keeps both indexes in step whenever an entry leaves the store.
func (p *pairingStore) deleteEntryLocked(digest string) {
	delete(p.entries, digest)
	maps.DeleteFunc(p.manual, func(_ string, artifactDigest string) bool {
		return artifactDigest == digest
	})
}
