package heartbeat

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

type CheckTarget struct {
	Instance      agentinstance.Instance
	Policy        ResolvedPolicy
	Scope         CollectorScope
	Gate          string
	MainPrompting bool
}
type CheckEnvironment interface {
	ResolveCheck(context.Context, string) (CheckTarget, error)
	BeforeCheck(context.Context, CheckTarget, string) (string, error)
	Collect(context.Context, string, CollectorScope, []string, string, string) (CheckDigest, error)
}
type TriageOutcome struct {
	Result, Reason, Detail string
	DeniedCalls            int64
	Report                 *agentinstance.ReportInput
}
type TriageDispatcher interface {
	Dispatch(context.Context, CheckTarget, CheckDigest, func(string) error) (TriageOutcome, error)
	Stop(context.Context, string) error
}
type CheckConfig struct {
	QuietBackoffCap                           time.Duration
	FatigueLimit                              int
	Enabled                                   bool
	Interval, MaxInterval, Retention, Timeout time.Duration
	MaxConcurrent, BatchSize                  int
}
type CheckResult struct {
	Gate         string       `json:"gate,omitempty"`
	Digest       *CheckDigest `json:"digest,omitempty"`
	OccurrenceID string       `json:"occurrence_id,omitempty"`
}
type CheckService struct {
	store         InstanceWakeStore
	environment   CheckEnvironment
	dispatcher    TriageDispatcher
	config        CheckConfig
	now           func() time.Time
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	ready, closed bool
	pending       map[string]FinishRequest
	wg            sync.WaitGroup
}

func NewCheckService(ctx context.Context, storage InstanceWakeStore, environment CheckEnvironment, dispatcher TriageDispatcher, config CheckConfig, now func() time.Time) (*CheckService, error) {
	if ctx == nil || storage == nil || environment == nil || dispatcher == nil || now == nil || config.Interval <= 0 || config.MaxInterval <= 0 || config.Retention <= 0 || config.Timeout <= 0 || config.MaxConcurrent < 1 || config.BatchSize < 1 {
		return nil, errors.New("heartbeat: incomplete check service configuration")
	}
	lifetime, cancel := context.WithCancel(ctx)
	return &CheckService{store: storage, environment: environment, dispatcher: dispatcher, config: config, now: now, ctx: lifetime, cancel: cancel, pending: map[string]FinishRequest{}}, nil
}
func (s *CheckService) Shutdown(context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	return nil
}
func (s *CheckService) Tick(ctx context.Context) error {
	if err := s.flush(ctx); err != nil {
		return err
	}
	due, err := s.store.Due(ctx, s.now(), s.config.BatchSize)
	if err != nil {
		return err
	}
	var failures []error
	for _, state := range due {
		if _, err := s.check(ctx, state, false, false); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
func (s *CheckService) Check(ctx context.Context, id string, dryRun bool) (CheckResult, error) {
	state, err := s.store.WakeState(ctx, id)
	if err != nil {
		return CheckResult{}, err
	}
	return s.check(ctx, state, true, dryRun)
}
func (s *CheckService) check(ctx context.Context, state InstanceWakeState, manual, dryRun bool) (CheckResult, error) {
	s.mu.Lock()
	ready := s.ready && !s.closed
	s.mu.Unlock()
	if !ready {
		return CheckResult{}, errors.New("heartbeat: checks are not running")
	}
	inflight, err := s.store.Inflight(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	if state.OccurrenceID != "" || len(inflight) >= s.config.MaxConcurrent {
		return CheckResult{Gate: "no_slot"}, nil
	}
	target, err := s.environment.ResolveCheck(ctx, state.InstanceID)
	if err != nil {
		return CheckResult{}, err
	}
	now := s.now()
	cadence := max(s.config.Interval, target.Policy.Preferences.MinInterval)
	skip := func(reason string) (CheckResult, error) {
		if manual || dryRun {
			return CheckResult{Gate: reason}, nil
		}
		_, err := s.store.Skip(ctx, SkipRequest{InstanceID: state.InstanceID, ExpectedDue: state.NextWakeAt, Reason: reason, Now: now, NextWakeAt: nextCheckTime(now, cadence, state.InstanceID), ExpiresAt: now.Add(s.config.Retention)})
		return CheckResult{Gate: reason}, err
	}
	if !s.config.Enabled {
		return skip("heartbeat_disabled")
	}
	if (!dryRun && target.Instance.State != "active") || target.Instance.ArchivedAt != nil || !target.Instance.ChecksEnabled {
		return skip("heartbeat_disabled")
	}
	if target.Gate != "" {
		return skip(target.Gate)
	}
	if !target.Policy.Enabled {
		return skip("policy_disabled")
	}
	if allowed, err := target.Policy.Preferences.AllowsAt(now); err != nil {
		return CheckResult{}, err
	} else if !allowed {
		return skip("quiet_window")
	}
	if target.MainPrompting {
		return skip("session_prompt_active")
	}
	source := "timer"
	if manual {
		source = "instance_check"
	}
	if !dryRun {
		reason, err := s.environment.BeforeCheck(ctx, target, source)
		if err != nil {
			return CheckResult{}, err
		}
		if reason != "" {
			return skip("hook_cancelled")
		}
	}
	target.Scope.Now = now
	target.Scope.AutomationCursor = state.AutomationCursor
	if state.LastDigestAt != "" {
		target.Scope.LastCompletedAt, err = store.ParseTimestamp(state.LastDigestAt)
		if err != nil {
			return CheckResult{}, err
		}
	}
	digest, err := s.environment.Collect(ctx, state.InstanceID, target.Scope, target.Policy.Preferences.Context.Include, target.Policy.Digest, target.Policy.GuidanceMarkdown)
	if err != nil {
		return CheckResult{}, err
	}
	if len(digest.Items) == 0 && strings.TrimSpace(target.Policy.GuidanceMarkdown) == "" {
		return skip("digest_empty")
	}
	if !manual && digest.Fingerprint == state.LastDigest && now.Sub(target.Scope.LastCompletedAt) < s.config.MaxInterval {
		return skip("digest_unchanged")
	}
	if dryRun {
		return CheckResult{Gate: "would_run", Digest: &digest}, nil
	}
	occurrence, err := store.NewID(source)
	if err != nil {
		return CheckResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return CheckResult{}, errors.New("heartbeat: checks are stopped")
	}
	admitted, err := s.store.Admit(ctx, AdmitRequest{InstanceID: state.InstanceID, OccurrenceID: occurrence, Now: now, NextWakeAt: nextCheckTime(now, cadence, state.InstanceID), ManualCheck: manual, MaxConcurrent: s.config.MaxConcurrent})
	if err != nil {
		return CheckResult{}, err
	}
	if !admitted {
		return CheckResult{Gate: "no_slot"}, nil
	}
	s.wg.Go(func() { s.dispatch(target, digest, occurrence, source) })
	return CheckResult{OccurrenceID: occurrence}, nil
}
func nextCheckTime(now time.Time, cadence time.Duration, id string) time.Time {
	hash := sha256.Sum256([]byte(id))
	return now.Add(cadence + time.Duration(binary.LittleEndian.Uint64(hash[:8])%1000)*(cadence/10000))
}
func (s *CheckService) flush(ctx context.Context) error {
	s.mu.Lock()
	pending := make([]FinishRequest, 0, len(s.pending))
	for _, req := range s.pending {
		pending = append(pending, req)
	}
	s.mu.Unlock()
	for _, req := range pending {
		if _, err := s.store.Finish(ctx, req); err != nil {
			return fmt.Errorf("finish proactive check: %w", err)
		}
		s.mu.Lock()
		delete(s.pending, req.OccurrenceID)
		s.mu.Unlock()
	}
	return nil
}
