package deadentity

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

type transitionPublication struct {
	revision           uint64
	predecessor        *transitionPublication
	done               chan struct{}
	skippedPredecessor bool
}

func (s *Service) reserveTransitionPublicationLocked(
	key store.DeadEntityKey,
	operation *entityOperation,
) *transitionPublication {
	publication := &transitionPublication{
		revision: operation.revision,
		done:     make(chan struct{}),
	}

	s.mu.Lock()
	if predecessor := s.publicationTails[key]; predecessor != nil {
		publication.predecessor = predecessor
		if publication.revision <= predecessor.revision {
			publication.revision = predecessor.revision + 1
		}
	}
	s.publicationTails[key] = publication
	s.mu.Unlock()
	return publication
}

func (s *Service) publishTransition(
	ctx context.Context,
	publication *transitionPublication,
	entity store.DeadEntity,
	marked bool,
) error {
	defer s.completeTransitionPublication(entity.DeadEntityKey, publication)
	if err := publication.waitForPredecessors(ctx); err != nil {
		publication.skippedPredecessor = true
		return err
	}
	return s.emitTransition(ctx, entity, marked)
}

func (p *transitionPublication) waitForPredecessors(ctx context.Context) error {
	for predecessor := p.predecessor; predecessor != nil; predecessor = predecessor.predecessor {
		select {
		case <-predecessor.done:
			if !predecessor.skippedPredecessor {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *Service) completeTransitionPublication(
	key store.DeadEntityKey,
	publication *transitionPublication,
) {
	close(publication.done)
	s.mu.Lock()
	if transitionPublicationChainComplete(s.publicationTails[key]) {
		delete(s.publicationTails, key)
	}
	s.mu.Unlock()
}

func transitionPublicationChainComplete(publication *transitionPublication) bool {
	for publication != nil {
		select {
		case <-publication.done:
			if !publication.skippedPredecessor {
				return true
			}
			publication = publication.predecessor
		default:
			return false
		}
	}
	return true
}
