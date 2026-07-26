package memory

import (
	"fmt"

	"time"
)

func (s *Service) acquireLock() (time.Time, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pending {
		return time.Time{}, false, nil
	}

	priorMtime, ok, err := s.lock.TryAcquire()
	if err != nil || !ok {
		return priorMtime, ok, err
	}

	s.pending = true
	s.priorMtime = priorMtime
	return priorMtime, true, nil
}

func (s *Service) ensureLock() (time.Time, error) {
	s.mu.Lock()
	if s.pending {
		priorMtime := s.priorMtime
		s.mu.Unlock()
		return priorMtime, nil
	}
	s.mu.Unlock()

	priorMtime, ok, err := s.acquireLock()
	if err != nil {
		return time.Time{}, err
	}
	if !ok {
		return time.Time{}, ErrLockUnavailable
	}

	return priorMtime, nil
}

func (s *Service) completeRun(success bool, priorMtime time.Time) error {
	var err error
	if success {
		err = s.lock.Release()
	} else {
		err = s.lock.Rollback(priorMtime)
	}

	s.mu.Lock()
	s.pending = false
	s.priorMtime = time.Time{}
	s.mu.Unlock()

	if err != nil {
		if success {
			return fmt.Errorf("memory: release consolidation lock: %w", err)
		}
		return fmt.Errorf("memory: rollback consolidation lock: %w", err)
	}

	return nil
}

func withNow(now func() time.Time) Option {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

func withSessionCounter(counter func(time.Time) (int, error)) Option {
	return func(service *Service) {
		if counter != nil {
			service.countSessionsSince = counter
		}
	}
}

func withLock(lock consolidationLocker) Option {
	return func(service *Service) {
		if lock != nil {
			service.lock = lock
		}
	}
}
