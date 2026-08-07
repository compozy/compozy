package gateway

import (
	"errors"
	"io"
	"sync"
	"time"
)

type pairingEntry struct {
	expiresAt time.Time
	source    PairingSource
	spent     bool
}

// PairingStore is a bounded, process-local set of single-use pairing artifacts.
type PairingStore struct {
	mu         sync.Mutex
	entries    map[string]pairingEntry
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
) (*PairingStore, error) {
	if maxPending <= 0 {
		return nil, errors.New("gateway: pairing max pending must be positive")
	}
	if ttl <= 0 {
		return nil, errors.New("gateway: pairing TTL must be positive")
	}
	return &PairingStore{
		entries: make(map[string]pairingEntry), maxPending: maxPending,
		ttl: ttl, now: now, entropy: entropy,
	}, nil
}

func (p *PairingStore) Mint(source PairingSource) (PairingArtifact, error) {
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
		return PairingArtifact{}, err
	}
	expiresAt := now.Add(p.ttl)
	p.entries[hashCredential(artifact)] = pairingEntry{expiresAt: expiresAt, source: source}
	return PairingArtifact{Artifact: artifact, ExpiresAt: expiresAt}, nil
}

func (p *PairingStore) evictSpentUntilBelowLimit() {
	for digest, entry := range p.entries {
		if len(p.entries) < p.maxPending {
			return
		}
		if entry.spent {
			delete(p.entries, digest)
		}
	}
}

func (p *PairingStore) pendingCount() int {
	pending := 0
	for _, entry := range p.entries {
		if !entry.spent {
			pending++
		}
	}
	return pending
}

// Redeem holds the single-use decision through the durable device write.
func (p *PairingStore) Redeem(artifact string, issue func(PairingSource) error) error {
	if !validOpaqueSecret(artifact, pairingArtifactPrefix) {
		return ErrPairingInvalid
	}
	digest := hashCredential(artifact)
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.entries[digest]
	if !ok {
		return ErrPairingInvalid
	}
	if !p.now().Before(entry.expiresAt) {
		delete(p.entries, digest)
		return ErrPairingExpired
	}
	if entry.spent {
		return ErrPairingSpent
	}
	if err := issue(entry.source); err != nil {
		return err
	}
	entry.spent = true
	p.entries[digest] = entry
	return nil
}

func (p *PairingStore) sweepExpired(now time.Time) {
	for digest, entry := range p.entries {
		if !now.Before(entry.expiresAt) {
			delete(p.entries, digest)
		}
	}
}
