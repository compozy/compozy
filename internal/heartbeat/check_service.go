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
	"github.com/compozy/compozy/internal/diagnostics"
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
	BeforeCheck(context.Context, *CheckTarget, string) (string, error)
	Collect(context.Context, string, CollectorScope, []string, string, string) (CheckDigest, error)
}
type TriageOutcome struct {
	Result, Reason, Detail string
	DeniedCalls            int64
	Report                 *agentinstance.ReportInput
}
type TriageDispatcher interface {
	Dispatch(context.Context, *CheckTarget, CheckDigest, func(string) error) (TriageOutcome, error)
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

func NewCheckService(
	ctx context.Context,
	storage InstanceWakeStore,
	environment CheckEnvironment,
	dispatcher TriageDispatcher,
	config CheckConfig,
	now func() time.Time,
) (*CheckService, error) {
	if ctx == nil || storage == nil || environment == nil || dispatcher == nil || now == nil || config.Interval <= 0 ||
		config.MaxInterval <= 0 ||
		config.Retention <= 0 ||
		config.Timeout <= 0 ||
		config.MaxConcurrent < 1 ||
		config.BatchSize < 1 {
		return nil, errors.New("heartbeat: incomplete check service configuration")
	}
	lifetime, cancel := context.WithCancel(ctx)
	return &CheckService{
		store:       storage,
		environment: environment,
		dispatcher:  dispatcher,
		config:      config,
		now:         now,
		ctx:         lifetime,
		cancel:      cancel,
		pending:     map[string]FinishRequest{},
	}, nil
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
	target, err := s.environment.ResolveCheck(ctx, state.InstanceID)
	if err != nil {
		return CheckResult{}, s.rescheduleCheckFailure(ctx, state, manual || dryRun, s.config.Interval, err)
	}
	if manual && !dryRun && target.Instance.State == "paused" {
		return CheckResult{}, agentinstance.ErrPaused
	}
	inflight, err := s.store.Inflight(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	if state.OccurrenceID != "" || len(inflight) >= s.config.MaxConcurrent {
		return CheckResult{Gate: "no_slot"}, nil
	}
	return s.checkTarget(ctx, state, &target, manual, dryRun)
}

func (s *CheckService) checkTarget(
	ctx context.Context,
	state InstanceWakeState,
	target *CheckTarget,
	manual, dryRun bool,
) (CheckResult, error) {
	now := s.now()
	cadence := s.checkInterval(&target.Policy)
	if skipped, err := s.skipInitialCheck(ctx, state, target, manual, cadence, now); skipped {
		return CheckResult{Gate: "cooldown_active"}, err
	}
	skip := func(reason, detail string) (CheckResult, error) {
		if manual || dryRun {
			return CheckResult{Gate: reason}, nil
		}
		_, err := s.store.Skip(
			ctx,
			SkipRequest{
				InstanceID:  state.InstanceID,
				ExpectedDue: state.NextWakeAt,
				Reason:      reason,
				Detail:      diagnostics.RedactAndBound(detail, 300),
				Now:         now,
				NextWakeAt:  NextCheckTime(now, s.checkBackoff(cadence, state), state.InstanceID),
				ExpiresAt:   now.Add(s.config.Retention),
			},
		)
		return CheckResult{Gate: reason}, err
	}
	gate, err := s.checkGate(target, dryRun, now)
	if err != nil {
		return CheckResult{}, err
	}
	if gate != "" {
		return skip(gate, "")
	}
	source := "timer"
	if manual {
		source = "instance_check"
	}
	if !dryRun {
		reason, err := s.environment.BeforeCheck(ctx, target, source)
		if err != nil {
			return CheckResult{}, s.rescheduleCheckFailure(ctx, state, manual || dryRun, cadence, err)
		}
		if reason != "" {
			return skip("hook_cancelled", reason) //nolint:misspell // Matches the persisted wake reason.
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
	digest, err := s.environment.Collect(
		ctx,
		state.InstanceID,
		target.Scope,
		target.Policy.Preferences.Context.Include,
		target.Policy.Digest,
		target.Policy.GuidanceMarkdown,
	)
	if err != nil {
		return CheckResult{}, s.rescheduleCheckFailure(ctx, state, manual || dryRun, cadence, err)
	}
	if gate := s.checkDigestGate(target, &digest, state, manual, now); gate != "" {
		return skip(gate, "")
	}
	if dryRun {
		return CheckResult{Gate: "would_run", Digest: &digest}, nil
	}
	return s.admitCheck(ctx, target, digest, AdmitRequest{
		InstanceID: state.InstanceID, Now: now,
		NextWakeAt:  NextCheckTime(now, cadence, state.InstanceID),
		ManualCheck: manual, MaxConcurrent: s.config.MaxConcurrent,
	}, source)
}

func NextCheckTime(now time.Time, cadence time.Duration, id string) time.Time {
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

func (s *CheckService) rescheduleCheckFailure(
	ctx context.Context,
	state InstanceWakeState,
	passive bool,
	cadence time.Duration,
	cause error,
) error {
	if passive {
		return cause
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.config.Timeout)
	defer cancel()
	now := s.now()
	_, err := s.store.Skip(cleanupCtx, SkipRequest{
		InstanceID:  state.InstanceID,
		ExpectedDue: state.NextWakeAt,
		Reason:      "triage_failed",
		Detail:      "The check could not be prepared; retry scheduled.",
		Now:         now,
		NextWakeAt:  NextCheckTime(now, s.checkBackoff(cadence, state), state.InstanceID),
		ExpiresAt:   now.Add(s.config.Retention),
	})
	return errors.Join(cause, err)
}

func (s *CheckService) checkGate(target *CheckTarget, dryRun bool, now time.Time) (string, error) {
	if !s.config.Enabled {
		return "heartbeat_disabled", nil
	}
	if (!dryRun && target.Instance.State != "active") || target.Instance.ArchivedAt != nil ||
		!target.Instance.ChecksEnabled {
		return "heartbeat_disabled", nil
	}
	if target.Gate != "" {
		return target.Gate, nil
	}
	if !target.Policy.Enabled {
		return "policy_disabled", nil
	}
	if allowed, err := target.Policy.Preferences.AllowsAt(now); err != nil {
		return "", err
	} else if !allowed {
		return "quiet_window", nil
	}
	if target.MainPrompting {
		return "session_prompt_active", nil
	}
	return "", nil
}

func (s *CheckService) checkDigestGate(
	target *CheckTarget,
	digest *CheckDigest,
	state InstanceWakeState,
	manual bool,
	now time.Time,
) string {
	if len(digest.Items) == 0 && strings.TrimSpace(target.Policy.GuidanceMarkdown) == "" {
		return "digest_empty"
	}
	if !manual && digest.Fingerprint == state.LastDigest &&
		now.Sub(target.Scope.LastCompletedAt) < s.config.MaxInterval {
		return "digest_unchanged"
	}
	return ""
}

func (s *CheckService) skipInitialCheck(
	ctx context.Context,
	state InstanceWakeState,
	target *CheckTarget,
	manual bool,
	cadence time.Duration,
	now time.Time,
) (bool, error) {
	if manual || state.NextWakeAt != "" || state.LastWakeAt != "" {
		return false, nil
	}
	firstCheck := target.Instance.CreatedAt.Add(cadence)
	if !firstCheck.After(now) {
		return false, nil
	}
	_, err := s.store.Skip(ctx, SkipRequest{
		InstanceID:  state.InstanceID,
		ExpectedDue: state.NextWakeAt,
		Reason:      "cooldown_active",
		Now:         now,
		NextWakeAt:  firstCheck,
		ExpiresAt:   now.Add(s.config.Retention),
	})
	return true, err
}
