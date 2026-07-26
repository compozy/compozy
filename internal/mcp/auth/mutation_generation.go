package auth

import "sync"

// WithMutationGeneration shares token mutation ordering across Service instances.
func WithMutationGeneration(generation *MutationGeneration) ServiceOption {
	return func(service *Service) {
		service.generation = generation
	}
}

// MutationGeneration coordinates durable token mutations across Service instances.
type MutationGeneration struct {
	mu      sync.Mutex
	targets map[string]*targetMutationGeneration
}

type targetMutationGeneration struct {
	mu    sync.Mutex
	value uint64
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

// Advance supersedes every mutation that started in an earlier generation.
func (g *MutationGeneration) Advance(target Target) (uint64, error) {
	generation, err := g.forTarget(target)
	if err != nil {
		return 0, err
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	generation.value++
	return generation.value, nil
}

// CommitIfCurrent runs commit atomically with respect to generation changes.
func (g *MutationGeneration) CommitIfCurrent(
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
	generation.value++
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
	generation := &targetMutationGeneration{}
	g.targets[key] = generation
	return generation, nil
}
