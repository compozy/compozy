package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// ChallengePathPrefix is mounted only on private and public tier listeners.
const ChallengePathPrefix = "/.well-known/compozy/gateway-challenge/"

// ChallengeResolver resolves an active nonce without exposing mutation methods to transports.
type ChallengeResolver interface {
	Resolve(Tier, string) (string, bool)
}

type activeChallenge struct {
	id    string
	path  string
	nonce string
}

// ChallengeRegistry owns the single active verification challenge for each tier listener.
type ChallengeRegistry struct {
	mu     sync.RWMutex
	random io.Reader
	active map[Tier]activeChallenge
}

// NewChallengeRegistry constructs a cryptographically random tier challenge registry.
func NewChallengeRegistry() *ChallengeRegistry {
	return &ChallengeRegistry{random: rand.Reader, active: make(map[Tier]activeChallenge)}
}

// Begin replaces the current challenge for one tier and returns an ownership-safe cleanup.
func (r *ChallengeRegistry) Begin(tier Tier) (path string, nonce string, cleanup func(), err error) {
	if r == nil {
		return "", "", nil, errors.New("gateway: challenge registry is required")
	}
	if err := tier.Validate(); err != nil {
		return "", "", nil, err
	}
	idBytes := make([]byte, 16)
	if _, err := io.ReadFull(r.random, idBytes); err != nil {
		return "", "", nil, fmt.Errorf("gateway: generate challenge id: %w", err)
	}
	nonceBytes := make([]byte, 32)
	if _, err := io.ReadFull(r.random, nonceBytes); err != nil {
		return "", "", nil, fmt.Errorf("gateway: generate challenge nonce: %w", err)
	}
	id := hex.EncodeToString(idBytes)
	challenge := activeChallenge{
		id: id, path: ChallengePathPrefix + id, nonce: hex.EncodeToString(nonceBytes),
	}
	r.mu.Lock()
	r.active[tier] = challenge
	r.mu.Unlock()
	var once sync.Once
	return challenge.path, challenge.nonce, func() {
		once.Do(func() { r.clear(tier, id) })
	}, nil
}

// Resolve returns the exact nonce only for the active path on the assigned tier.
func (r *ChallengeRegistry) Resolve(tier Tier, path string) (string, bool) {
	if r == nil || !strings.HasPrefix(path, ChallengePathPrefix) {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	challenge, ok := r.active[tier]
	if !ok || challenge.path != path {
		return "", false
	}
	return challenge.nonce, true
}

var _ ChallengeResolver = (*ChallengeRegistry)(nil)

func (r *ChallengeRegistry) clear(tier Tier, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.active[tier]
	if ok && challenge.id == id {
		delete(r.active, tier)
	}
}
