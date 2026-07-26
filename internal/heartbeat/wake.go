package heartbeat

import (
	"context"

	"errors"

	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	// SyntheticReasonHeartbeatWake marks daemon-owned prompts emitted by the Heartbeat wake service.
	SyntheticReasonHeartbeatWake = "agent_heartbeat_wake"
)

var (
	// ErrSyntheticPromptBusy reports that the session prompt gate rejected a no-queue wake.
	ErrSyntheticPromptBusy = errors.New("heartbeat: synthetic prompt busy")
)

// WakeService evaluates Heartbeat policy and session health before issuing advisory wakes.
type WakeService interface {
	Wake(ctx context.Context, req WakeRequest) (WakeDecision, error)
	WakeMany(ctx context.Context, requests []WakeRequest) ([]WakeDecision, error)
}

// WakeStore provides persisted Heartbeat policy, wake state, and wake audit rows.
type WakeStore interface {
	GetLatestValidHeartbeatSnapshot(ctx context.Context, workspaceID string, agentName string) (Snapshot, error)
	GetHeartbeatWakeState(
		ctx context.Context,
		workspaceID string,
		agentName string,
		sessionID string,
	) (WakeState, error)
	UpsertHeartbeatWakeState(ctx context.Context, state WakeState) (WakeState, error)
	AppendHeartbeatWakeEvent(ctx context.Context, event WakeEvent) (WakeEvent, error)
	RecordHeartbeatWakeDecision(ctx context.Context, event WakeEvent, state WakeState) (WakeEvent, WakeState, error)
}

// SyntheticWakePrompter injects one daemon-owned synthetic wake prompt through the session path.
type SyntheticWakePrompter interface {
	PromptHeartbeatWake(ctx context.Context, req SyntheticWakePromptRequest) (SyntheticWakePromptResult, error)
}

// WakeRequest identifies one advisory Heartbeat wake decision.
type WakeRequest struct {
	WorkspaceID          string
	AgentName            string
	SessionID            string
	Source               WakeSource
	DryRun               bool
	SyntheticCorrelation WakeSyntheticCorrelation
}

// WakeDecision reports the auditable result of one Heartbeat wake decision.
type WakeDecision struct {
	WakeEventID       string
	Result            WakeResult
	Reason            WakeReason
	PolicySnapshotID  string
	PolicyDigest      string
	ConfigDigest      string
	SyntheticPromptID string
	Diagnostics       []Diagnostic
}

// WakeSyntheticCorrelation carries optional claimed-run lineage for synthetic
// wake prompts emitted through the heartbeat path.
type WakeSyntheticCorrelation struct {
	TaskID               string
	TaskRunID            string
	WorkflowID           string
	ClaimTokenHash       string
	CoordinatorSessionID string
}

// Normalize returns a trimmed copy of the wake synthetic correlation fields.
func (c WakeSyntheticCorrelation) Normalize() WakeSyntheticCorrelation {
	return WakeSyntheticCorrelation{
		TaskID:               strings.TrimSpace(c.TaskID),
		TaskRunID:            strings.TrimSpace(c.TaskRunID),
		WorkflowID:           strings.TrimSpace(c.WorkflowID),
		ClaimTokenHash:       strings.TrimSpace(c.ClaimTokenHash),
		CoordinatorSessionID: strings.TrimSpace(c.CoordinatorSessionID),
	}
}

// SyntheticWakePromptRequest carries the prompt input and stable Heartbeat correlation metadata.
type SyntheticWakePromptRequest struct {
	SessionID            string
	Message              string
	TurnID               string
	WakeEventID          string
	PolicySnapshotID     string
	PolicyDigest         string
	ConfigDigest         string
	Summary              string
	SyntheticCorrelation WakeSyntheticCorrelation
}

// SyntheticWakePromptResult reports the session prompt turn selected for a sent wake.
type SyntheticWakePromptResult struct {
	SyntheticPromptID string
}

// ManagedWakeService is the production Heartbeat wake decision engine.
type ManagedWakeService struct {
	store        WakeStore
	healthReader SessionHealthReader
	prompter     SyntheticWakePrompter
	config       aghconfig.HeartbeatConfig
	now          func() time.Time
	newID        func(prefix string) string
	mu           sync.Mutex
}

var _ WakeService = (*ManagedWakeService)(nil)

// WakeOption customizes the managed Heartbeat wake service.
type WakeOption func(*ManagedWakeService)

// WithWakeClock injects deterministic time for wake decisions.
func WithWakeClock(clock func() time.Time) WakeOption {
	return func(service *ManagedWakeService) {
		if clock != nil {
			service.now = clock
		}
	}
}

// WithWakeIDGenerator injects deterministic wake event and prompt turn IDs.
func WithWakeIDGenerator(generator func(prefix string) string) WakeOption {
	return func(service *ManagedWakeService) {
		if generator != nil {
			service.newID = generator
		}
	}
}

// NewManagedWakeService creates the production Heartbeat wake service.
func NewManagedWakeService(
	store WakeStore,
	healthReader SessionHealthReader,
	prompter SyntheticWakePrompter,
	config aghconfig.HeartbeatConfig,
	options ...WakeOption,
) (*ManagedWakeService, error) {
	if store == nil {
		return nil, errors.New("heartbeat: wake store is required")
	}
	if healthReader == nil {
		return nil, errors.New("heartbeat: session health reader is required")
	}
	if prompter == nil {
		return nil, errors.New("heartbeat: synthetic wake prompter is required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	service := &ManagedWakeService{
		store:        store,
		healthReader: healthReader,
		prompter:     prompter,
		config:       config,
		now:          time.Now,
		newID:        defaultWakeID,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

// Wake evaluates one policy/health decision and emits a synthetic prompt only when eligible.
func (s *ManagedWakeService) Wake(ctx context.Context, req WakeRequest) (WakeDecision, error) {
	if ctx == nil {
		return WakeDecision{}, errors.New("heartbeat: wake context is required")
	}
	normalized, err := normalizeWakeRequest(req)
	if err != nil {
		return WakeDecision{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wakeLocked(ctx, normalized)
}

// WakeMany evaluates a scheduler cycle and applies the configured max wake bound.
func (s *ManagedWakeService) WakeMany(ctx context.Context, requests []WakeRequest) ([]WakeDecision, error) {
	if ctx == nil {
		return nil, errors.New("heartbeat: wake context is required")
	}
	decisions := make([]WakeDecision, 0, len(requests))
	sent := 0
	var errs []error
	for _, req := range requests {
		if sent >= s.config.MaxWakesPerCycle {
			decision, err := s.recordRateLimited(ctx, req)
			if err != nil {
				errs = append(errs, err)
				decisions = append(decisions, s.failedWakeManyDecision(err))
				continue
			}
			decisions = append(decisions, decision)
			continue
		}
		decision, err := s.Wake(ctx, req)
		if err != nil {
			errs = append(errs, err)
			decisions = append(decisions, s.failedWakeManyDecision(err))
			continue
		}
		if decision.Result == WakeResultSent {
			sent++
		}
		decisions = append(decisions, decision)
	}
	return decisions, errors.Join(errs...)
}

func (s *ManagedWakeService) failedWakeManyDecision(err error) WakeDecision {
	decision := s.newDecision(WakeResultFailed, WakeReasonSyntheticPromptFailed, "", "", "")
	if err != nil {
		decision.Diagnostics = []Diagnostic{sanitizeWakeDiagnostic(Diagnostic{
			Code:     "heartbeat_wake_error",
			Severity: diagnosticError,
			Message:  err.Error(),
		})}
	}
	return decision
}
