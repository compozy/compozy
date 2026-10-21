package providerusage

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/jonboulle/clockwork"
)

// Start reconciles the complete live profile list and schedules startup within ten seconds.
func (s *Manager) Start(ctx context.Context, profileIDs []string) error {
	if err := s.removeProfiles(ctx, profileIDs); err != nil {
		return err
	}
	for _, profile := range profileIDs {
		canceled, err := s.resolveTargets(ctx, profile, nil)
		if err != nil {
			return err
		}
		if err := joinFlights(ctx, canceled); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped || !s.enabled {
		return nil
	}
	s.started = true
	s.startScheduleLocked(context.WithoutCancel(ctx))
	return nil
}

func (s *Manager) removeProfiles(ctx context.Context, live []string) error {
	s.mu.Lock()
	var removed []string
	var canceled []*flight
	for id, p := range s.profiles {
		if slices.Contains(live, id) {
			continue
		}
		removed = append(removed, id)
		for _, state := range p.targets {
			if state.flight != nil {
				state.flight.cancel()
				canceled = append(canceled, state.flight)
			}
		}
		for ch := range p.subscribers {
			delete(p.subscribers, ch)
			close(ch)
		}
		delete(s.profiles, id)
	}
	s.mu.Unlock()
	for _, id := range removed {
		for _, collector := range s.options.Collectors {
			if cache, ok := collector.(CredentialCache); ok {
				cache.ClearCredentials(id)
			}
		}
	}
	return joinFlights(ctx, canceled)
}

type scheduleRun struct {
	timer   clockwork.Timer
	initial bool
}

// startScheduleLocked gives each scheduler run its own timer.
func (s *Manager) startScheduleLocked(ctx context.Context) {
	var seed [8]byte
	rand.Read(seed[:])
	jitter := time.Duration(binary.LittleEndian.Uint64(seed[:]) % uint64(10*time.Second))
	run := &scheduleRun{timer: s.options.Clock.NewTimer(jitter), initial: true}
	s.scheduler = run
	s.wg.Add(1)
	go s.schedule(ctx, s.root, run)
}

func (s *Manager) schedule(ctx context.Context, root context.Context, run *scheduleRun) {
	defer s.wg.Done()
	defer run.timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-root.Done():
			return
		case <-run.timer.Chan():
		}
		s.mu.Lock()
		run.initial = false
		run.timer.Reset(s.options.RefreshInterval)
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
		s.startScheduleLocked(context.WithoutCancel(ctx))
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

// SetRefreshInterval reschedules from now; intervals outside [1m, 1h] are ignored.
// Startup jitter, error outcomes, and active cooldown deadlines are preserved.
func (s *Manager) SetRefreshInterval(interval time.Duration) {
	if interval < time.Minute || interval > time.Hour {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.options.RefreshInterval == interval {
		return
	}
	s.options.RefreshInterval = interval
	if s.enabled && s.scheduler != nil && !s.scheduler.initial {
		s.scheduler.timer.Reset(interval)
	}
	now := s.options.Clock.Now()
	for _, p := range s.profiles {
		changed := false
		for _, row := range p.targets {
			if row.snapshot.MeasuredAt == nil {
				continue
			}
			next := now.Add(interval)
			if row.cooldown != nil && row.cooldown.until.After(next) {
				next = row.cooldown.until
			}
			row.snapshot.NextRefreshAt = &next
			if row.snapshot.Error == nil {
				row.snapshot.Status = StatusOK
				if now.Sub(*row.snapshot.MeasuredAt) > 3*interval {
					row.snapshot.Status = StatusStale
				}
			}
			changed = true
		}
		if changed {
			s.changedLocked(p)
		}
	}
}

// resolve cancels ineligible flights without making readers wait for their cleanup.
func (s *Manager) resolve(ctx context.Context, profile string) error {
	_, err := s.resolveTargets(ctx, profile, nil)
	return err
}

func joinFlights(ctx context.Context, canceled []*flight) error {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for _, f := range canceled {
		select {
		case <-f.done:
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return context.DeadlineExceeded
		}
	}
	return nil
}
