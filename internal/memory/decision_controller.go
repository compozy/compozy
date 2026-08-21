package memory

import (
	"context"
	"errors"
	"maps"
	"strings"
	"sync"

	memcontract "github.com/compozy/compozy/internal/memory/contract"
	"github.com/compozy/compozy/internal/memory/controller"
)

const decisionProfileMetadataKey = "__compozy_profile_id"

// DecisionControllerFactory builds the effective write controller for one
// store view. Store clones share the live factory registration.
type DecisionControllerFactory func(controller.TargetIndex) memcontract.Controller

type decisionControllerFactoryState struct {
	mu      sync.RWMutex
	factory DecisionControllerFactory
}

// SetDecisionControllerFactory installs the daemon-owned live controller
// composition used by writes, batches, and dry-run decisions.
func (s *Store) SetDecisionControllerFactory(factory DecisionControllerFactory) {
	if s == nil {
		return
	}
	if s.decisionControllerFactory == nil {
		return
	}
	s.decisionControllerFactory.mu.Lock()
	s.decisionControllerFactory.factory = factory
	s.decisionControllerFactory.mu.Unlock()
}

// DecideCandidate computes a controller decision without applying it.
func (s *Store) DecideCandidate(
	ctx context.Context,
	candidate memcontract.Candidate,
) (memcontract.Decision, error) {
	if ctx == nil {
		return memcontract.Decision{}, errors.New("memory: decide candidate context is required")
	}
	if s == nil {
		return memcontract.Decision{}, errors.New("memory: store is required")
	}
	return s.newDecisionController().Decide(ctx, s.profileScopedCandidate(candidate))
}

// profileScopedCandidate makes controller-generated WAL identities distinct
// for each durable profile. The database keeps decision IDs globally unique,
// so the profile owner must be part of the deterministic input before the WAL
// row is created rather than being inferred after a collision.
func (s *Store) profileScopedCandidate(candidate memcontract.Candidate) memcontract.Candidate {
	profileID := strings.TrimSpace(s.profileID)
	if profileID == "" {
		return candidate
	}
	metadata := make(map[string]string, len(candidate.Metadata)+1)
	maps.Copy(metadata, candidate.Metadata)
	metadata[decisionProfileMetadataKey] = profileID
	candidate.Metadata = metadata
	return candidate
}

func (s *Store) newDecisionController() memcontract.Controller {
	if s.decisionControllerFactory != nil {
		s.decisionControllerFactory.mu.RLock()
		factory := s.decisionControllerFactory.factory
		s.decisionControllerFactory.mu.RUnlock()
		if factory != nil {
			if built := factory(s); built != nil {
				return built
			}
		}
	}
	return controller.New(s)
}
