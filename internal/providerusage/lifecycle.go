package providerusage

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

// Start schedules all known profiles, with startup collection jitter bounded by ten seconds.
func (s *Manager) Start(ctx context.Context, profileIDs []string) error {
	for _, profile := range profileIDs {
		if err := s.resolve(ctx, profile); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped || !s.enabled {
		return nil
	}
	s.started = true
	s.wg.Add(1)
	go s.schedule(ctx, s.root)
	return nil
}

func (s *Manager) schedule(ctx context.Context, root context.Context) {
	defer s.wg.Done()
	var seed [8]byte
	rand.Read(seed[:])
	jitter := time.Duration(binary.LittleEndian.Uint64(seed[:]) % uint64(10*time.Second))
	timer := s.options.Clock.NewTimer(jitter)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-root.Done():
			return
		case <-timer.Chan():
		}
		s.mu.Lock()
		profiles := make([]string, 0, len(s.profiles))
		for profile := range s.profiles {
			profiles = append(profiles, profile)
		}
		s.mu.Unlock()
		for _, profile := range profiles {
			if err := s.resolve(ctx, profile); err != nil {
				continue
			}
			if _, err := s.startRefresh(
				RefreshRequest{ProfileID: profile, Intent: IntentBackground},
			); err != nil &&
				ctx.Err() == nil {
				s.options.Logger.Debug("provider_usage.schedule", "error_code", AsUsageError(err, "").Code)
			}
		}

		timer.Reset(s.options.RefreshInterval)
	}
}

func (s *Manager) Shutdown(ctx context.Context) error {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	s.mu.Lock()
	s.stopped = true
	s.enabled = false
	s.cancel()
	s.dropLocked()
	s.mu.Unlock()
	joinErr := s.join(ctx)
	s.closeOnce.Do(func() {
		for _, collector := range s.options.Collectors {
			if closer, ok := collector.(io.Closer); ok {
				s.closeErr = errors.Join(s.closeErr, closer.Close())
			}
		}
	})
	return errors.Join(joinErr, s.closeErr)
}

// SetEnabled drops all cached values and joins canceled flights before returning on disable.
func (s *Manager) SetEnabled(ctx context.Context, enabled bool) error {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	s.mu.Lock()
	if s.stopped || s.enabled == enabled {
		s.mu.Unlock()
		return nil
	}
	s.enabled = enabled
	if !enabled {
		s.cancel()
		s.dropLocked()
		s.mu.Unlock()
		joinErr := s.join(ctx)
		for _, collector := range s.options.Collectors {
			if cache, ok := collector.(CredentialCache); ok {
				cache.ClearCredentials("")
			}
		}
		return joinErr
	}
	s.root, s.cancel = context.WithCancel(context.Background())
	if s.started {
		s.wg.Add(1)
		go s.schedule(context.WithoutCancel(ctx), s.root)
	}
	s.mu.Unlock()
	return nil
}

func (s *Manager) dropLocked() {
	for _, p := range s.profiles {
		p.targets = make(map[string]*targetState)
		p.order = nil
		s.changedLocked(p)
	}
}

func (s *Manager) join(ctx context.Context) error {
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return context.DeadlineExceeded
	}
}

// ClearCredentials invalidates a provider cache and its snapshots after consent changes.
func (s *Manager) ClearCredentials(profileID, providerID string) {
	if cache, ok := s.options.Collectors[providerID].(CredentialCache); ok {
		cache.ClearCredentials(profileID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for profile, p := range s.profiles {
		if profileID != "" && profile != profileID {
			continue
		}
		if state := p.targets[providerID]; state != nil {
			state.epoch++
			state.snapshot = initialSnapshot(state.target, s.options.Collectors[providerID])
			state.hasIdentity = false
			state.lastSuccess = time.Time{}
			state.cooldown = nil
			s.changedLocked(p)
		}
	}
}
