package auth

import (
	"sync"
	"sync/atomic"
	"time"
)

// WithMutationGeneration shares token mutation ordering across Service instances.
func WithMutationGeneration(generation *MutationGeneration) ServiceOption {
	return func(service *Service) {
		service.generation = generation
	}
}

// MutationGeneration coordinates durable token mutations across Service instances.
type MutationGeneration struct {
	revision atomic.Uint64
	mu       sync.Mutex
	targets  map[string]*targetMutationGeneration
}

type targetMutationGeneration struct {
	mu                sync.Mutex
	value             uint64
	revision          uint64
	refreshGate       chan struct{}
	refreshGeneration uint64
	refreshAttempts   int
	refreshRetryAt    time.Time
	remoteRejected    bool
}

// NewMutationGeneration constructs a process-wide token mutation coordinator.
func NewMutationGeneration() *MutationGeneration {
	return &MutationGeneration{targets: make(map[string]*targetMutationGeneration)}
}

// Snapshot returns the current generation for one normalized target.
func (g *MutationGeneration) Snapshot(target Target) (uint64, error) {
	generation, err := g.forTarget(target)
	if err != nil {
		return 0, err
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	return generation.value, nil
}

// Revision changes on every committed credential mutation without superseding open logins.
func (g *MutationGeneration) Revision(target Target) (uint64, error) {
	generation, err := g.forTarget(target)
	if err != nil {
		return 0, err
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	return generation.revision, nil
}

// RejectIfCurrent records a 401 for the observed credential without affecting a replacement login.
func (g *MutationGeneration) RejectIfCurrent(target Target, revision uint64) error {
	generation, err := g.forTarget(target)
	if err != nil {
		return err
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	if generation.revision == revision && !generation.remoteRejected {
		generation.remoteRejected = true
		generation.revision++
		g.revision.Add(1)
	}
	return nil
}

func (g *MutationGeneration) remotelyRejected(target Target) (bool, error) {
	generation, err := g.forTarget(target)
	if err != nil {
		return false, err
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	return generation.remoteRejected, nil
}

// Advance supersedes every mutation that started in an earlier generation.
func (g *MutationGeneration) Advance(target Target) (uint64, error) {
	generation, err := g.forTarget(target)
	if err != nil {
		return 0, err
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	generation.value++
	generation.revision++
	g.revision.Add(1)
	return generation.value, nil
}

func (g *MutationGeneration) commitIfCurrent(
	target Target,
	expected uint64,
	replacesAccess bool,
	commit func() error,
) (bool, error) {
	generation, err := g.forTarget(target)
	if err != nil {
		return false, err
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	if generation.value != expected {
		return false, nil
	}
	if err := commit(); err != nil {
		return false, err
	}
	if replacesAccess {
		generation.remoteRejected = false
	}
	generation.revision++
	g.revision.Add(1)
	return true, nil
}

// CommitAndAdvanceIfCurrent commits one replacement token and atomically
// supersedes every mutation that observed the replaced generation.
func (g *MutationGeneration) CommitAndAdvanceIfCurrent(
	target Target,
	expected uint64,
	commit func() error,
) (bool, error) {
	generation, err := g.forTarget(target)
	if err != nil {
		return false, err
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	if generation.value != expected {
		return false, nil
	}
	if err := commit(); err != nil {
		return false, err
	}
	generation.remoteRejected = false
	generation.value++
	generation.revision++
	g.revision.Add(1)
	return true, nil
}

func (g *MutationGeneration) forTarget(target Target) (*targetMutationGeneration, error) {
	key, err := target.Key()
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrMutationGenerationUnavailable
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.targets == nil {
		g.targets = make(map[string]*targetMutationGeneration)
	}
	if generation := g.targets[key]; generation != nil {
		return generation, nil
	}
	generation := &targetMutationGeneration{refreshGate: make(chan struct{}, 1)}
	g.targets[key] = generation
	return generation, nil
}

// GlobalRevision changes on any committed credential mutation across normalized targets.
func (g *MutationGeneration) GlobalRevision() uint64 {
	if g == nil {
		return 0
	}
	return g.revision.Load()
}
