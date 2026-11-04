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
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
)

type CheckTarget struct {
	DefinitionDir, RootDir string
	UntrustedScripts       bool
	Instance               agentinstance.Instance
	Policy                 ResolvedPolicy
	Scope                  CollectorScope
	Gate                   string
	MainPrompting          bool
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
	Sources                                                compozyconfig.HeartbeatSourcesConfig
	UnattendedTurnLimit                                    int64
	QuietBackoffCap                                        time.Duration
	FatigueLimit                                           int
	Enabled                                                bool
	Interval, MinInterval, MaxInterval, Retention, Timeout time.Duration
	MaxConcurrent, BatchSize                               int
}
type CheckResult struct {
	Sources      []SourceOutcome `json:"sources,omitempty"`
	Gate         string          `json:"gate,omitempty"`
	Digest       *CheckDigest    `json:"digest,omitempty"`
	OccurrenceID string          `json:"occurrence_id,omitempty"`
}
type checkRun struct {
	occurrence string
	cancel     context.CancelFunc
}

type CheckService struct {
	sourcePollCounts   map[string]int64
	sourcePollDuration int64
	collecting         map[string]context.CancelFunc
	pollSem            chan struct{}
	recoveryMu         sync.Mutex
	afterPause         func(context.Context, agentinstance.Instance) error
	running            map[string]checkRun
	store              InstanceWakeStore
	environment        CheckEnvironment
	dispatcher         TriageDispatcher
	config             CheckConfig
	now                func() time.Time
	ctx                context.Context
	cancel             context.CancelFunc
	mu                 sync.Mutex
	ready, closed      bool
	pending            map[string]FinishRequest
	wg                 sync.WaitGroup
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
		config.BatchSize < 1 || config.UnattendedTurnLimit < 0 {
		return nil, errors.New("heartbeat: incomplete check service configuration")
	}
	if config.MinInterval <= 0 {
		config.MinInterval = compozyconfig.DefaultHeartbeatConfig().MinInterval
	}
	if config.Sources.IsZero() {
		config.Sources = compozyconfig.DefaultHeartbeatSourcesConfig()
	}
	if err := config.Sources.Validate(); err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithCancel(ctx)
	return &CheckService{
		collecting: map[string]context.CancelFunc{}, pollSem: make(chan struct{}, config.Sources.MaxConcurrentPolls),
		store:       storage,
		environment: environment,
		dispatcher:  dispatcher,
		config:      config,
		now:         now,
		ctx:         lifetime,
		cancel:      cancel,
		pending:     map[string]FinishRequest{},
		running:     map[string]checkRun{},
	}, nil
}
func (s *CheckService) Shutdown(context.Context) error {
	s.mu.Lock()
	s.closed = true
	for _, cancel := range s.collecting {
		cancel()
	}
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
	_, collecting := s.collecting[state.InstanceID]
	s.mu.Unlock()
	if collecting && !dryRun {
		return CheckResult{Gate: checkSourcePollInFlight}, nil
	}
	if !ready {
		return CheckResult{}, errors.New("heartbeat: checks are not running")
	}
	target, err := s.environment.ResolveCheck(ctx, state.InstanceID)
	if err != nil {
		target.Instance.ID = state.InstanceID
		return CheckResult{}, s.rescheduleCheckFailure(
			ctx,
			state,
			manual || dryRun,
			s.config.Interval,
			s.hasSourcesAfterResolveFailure(ctx, &target),
			err,
		)
	}
	if manual && !dryRun && target.Instance.State == "paused" {
		return CheckResult{}, agentinstance.ErrPaused
	}
	inflight, err := s.store.Inflight(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	if state.OccurrenceID != "" || len(inflight) >= s.config.MaxConcurrent {
		return CheckResult{Gate: checkNoSlot}, nil
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
		if dryRun || (manual && reason != checkHookCancelled) {
			return CheckResult{Gate: reason}, nil
		}
		_, err := s.store.Skip(
			ctx,
			SkipRequest{
				Deferred:    state.Deferred,
				ManualCheck: manual,
				InstanceID:  state.InstanceID,
				ExpectedDue: state.NextWakeAt,
				Reason:      reason,
				Detail:      diagnostics.RedactAndBound(detail, 300),
				Now:         now,
				NextWakeAt: NextCheckTime(
					now,
					s.checkBackoff(cadence, state, len(target.Policy.Preferences.Context.Sources) > 0),
					state.InstanceID,
				),
				ExpiresAt: now.Add(s.config.Retention),
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
	source := checkTimerSource
	if manual {
		source = checkManualSource
	}
	if !dryRun && len(target.Policy.Preferences.Context.Sources) == 0 {
		if env, ok := s.environment.(interface {
			ReconcileSourcePolicy(context.Context, *CheckTarget) error
		}); ok {
			if err := env.ReconcileSourcePolicy(ctx, target); err != nil {
				return CheckResult{}, s.rescheduleCheckFailure(ctx, state, manual, cadence, false, err)
			}
		}
		reason, err := s.environment.BeforeCheck(ctx, target, source)
		if err != nil {
			return CheckResult{}, s.rescheduleCheckFailure(
				ctx,
				state,
				manual || dryRun,
				cadence,
				len(target.Policy.Preferences.Context.Sources) > 0,
				err,
			)
		}
		if reason != "" {
			return skip(checkHookCancelled, reason)
		}
	}
	if len(target.Policy.Preferences.Context.Sources) > 0 {
		return s.collectSources(ctx, state, target, manual, dryRun)
	}
	return s.collectTarget(ctx, state, target, manual, dryRun, PollBatch{})
}

func (s *CheckService) collectTarget(
	ctx context.Context,
	state InstanceWakeState,
	target *CheckTarget,
	manual, dryRun bool,
	batch PollBatch,
) (CheckResult, error) {
	now := s.now()
	cadence := s.checkInterval(&target.Policy)
	var err error
	source := checkTimerSource
	if manual {
		source = checkManualSource
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
		return CheckResult{}, s.rescheduleCheckFailure(
			ctx,
			state,
			manual || dryRun,
			cadence,
			len(target.Policy.Preferences.Context.Sources) > 0,
			err,
		)
	}
	if len(batch.Sources) > 0 {
		if err := AppendSourceDigest(&digest, batch, s.config.Sources.FailureThreshold); err != nil {
			return CheckResult{}, err
		}
	}
	if gate := s.checkDigestGate(target, &digest, state, manual, now); gate != "" {
		return s.skipCollectedTarget(ctx, state, target, manual, dryRun, batch, gate)
	}
	if dryRun {
		return CheckResult{Gate: "would_run", Digest: &digest, Sources: batch.Outcomes()}, nil
	}
	if batch.HasMore() {
		cadence = s.config.MinInterval
	}
	return s.admitCheck(ctx, target, digest, AdmitRequest{
		ExpectedDue: state.NextWakeAt, Sources: batch.Sources, InstanceID: state.InstanceID, Now: now,
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
		if err := s.finish(ctx, req); err != nil {
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
	hasSources bool,
	cause error,
) error {
	if passive {
		return cause
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.config.Timeout)
	defer cancel()
	now := s.now()
	_, err := s.store.Skip(cleanupCtx, SkipRequest{
		Deferred:    state.Deferred,
		InstanceID:  state.InstanceID,
		ExpectedDue: state.NextWakeAt,
		Reason:      "triage_failed",
		Detail:      "The check could not be prepared; retry scheduled.",
		Now:         now,
		NextWakeAt:  NextCheckTime(now, s.checkBackoff(cadence, state, hasSources), state.InstanceID),
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
	hasSources := len(target.Policy.Preferences.Context.Sources) > 0
	if len(digest.Items) == 0 && (hasSources || strings.TrimSpace(target.Policy.GuidanceMarkdown) == "") {
		return "digest_empty"
	}
	if (hasSources || !manual) && digest.Fingerprint == state.LastDigest &&
		(hasSources || now.Sub(target.Scope.LastCompletedAt) < s.config.MaxInterval) {
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
		Deferred:    state.Deferred,
		InstanceID:  state.InstanceID,
		ExpectedDue: state.NextWakeAt,
		Reason:      "cooldown_active",
		Now:         now,
		NextWakeAt:  firstCheck,
		ExpiresAt:   now.Add(s.config.Retention),
	})
	return true, err
}

func (s *CheckService) skipCollectedTarget(
	ctx context.Context,
	state InstanceWakeState,
	target *CheckTarget,
	manual, dryRun bool,
	batch PollBatch,
	reason string,
) (CheckResult, error) {
	now := s.now()
	cadence := s.checkInterval(&target.Policy)
	result := CheckResult{Gate: reason, Sources: batch.Outcomes()}
	if dryRun {
		return result, nil
	}
	if manual && len(batch.Sources) == 0 {
		return result, nil
	}
	delay := s.checkBackoff(cadence, state, len(target.Policy.Preferences.Context.Sources) > 0)
	if batch.HasMore() {
		delay = s.config.MinInterval
	}
	_, err := s.store.Skip(
		ctx,
		SkipRequest{
			Deferred:    state.Deferred,
			ManualCheck: manual,
			InstanceID:  state.InstanceID,
			ExpectedDue: state.NextWakeAt,
			Reason:      reason,
			Now:         now,
			NextWakeAt:  NextCheckTime(now, delay, state.InstanceID),
			ExpiresAt:   now.Add(s.config.Retention),
			Sources:     batch.Sources,
		},
	)
	return result, err
}
