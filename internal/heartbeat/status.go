package heartbeat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
)

const diagnosticSessionHealthUnsupported = "session_health_unsupported"

// StatusService composes policy, wake state, and optional session health reads.
type StatusService interface {
	Inspect(ctx context.Context, req InspectRequest) (InspectResult, error)
	Status(ctx context.Context, req StatusRequest) (StatusResult, error)
}

// StatusStore provides persisted Heartbeat snapshots and wake state for status reads.
type StatusStore interface {
	FindHeartbeatSnapshotByDigest(
		ctx context.Context,
		workspaceID string,
		agentName string,
		digest string,
	) (Snapshot, bool, error)
	GetHeartbeatWakeState(
		ctx context.Context,
		workspaceID string,
		agentName string,
		sessionID string,
	) (WakeState, error)
	ListHeartbeatWakeState(ctx context.Context, query WakeStateListQuery) ([]WakeState, error)
}

// SessionHealthReader provides metadata-only session health for status reads.
type SessionHealthReader interface {
	GetSessionHealth(ctx context.Context, sessionID string) (SessionHealth, error)
}

// PolicyResolver resolves non-filesystem Heartbeat policies, such as package-owned agent resources.
type PolicyResolver interface {
	ResolveHeartbeatPolicy(ctx context.Context, target AuthoringTarget) (ResolvedPolicy, bool, error)
}

// InspectRequest identifies the policy source to inspect.
type InspectRequest struct {
	Target AuthoringTarget
}

// InspectResult returns the resolved policy and matching stored snapshot, when present.
type InspectResult struct {
	AgentName string
	Policy    ResolvedPolicy
	Snapshot  *Snapshot
}

// StatusRequest identifies the policy and optional session health to compose.
type StatusRequest struct {
	Target               AuthoringTarget
	SessionID            string
	IncludeSessionHealth bool
}

// StatusResult is the transport-neutral heartbeat status payload.
type StatusResult struct {
	AgentName        string
	SourcePath       string
	Enabled          bool
	Present          bool
	Active           bool
	Valid            bool
	Digest           string
	ConfigDigest     string
	SnapshotID       string
	Summary          string
	Preferences      Preferences
	Diagnostics      []Diagnostic
	ConfigProvenance ConfigProvenance
	Policy           StatusData
	WakeState        *WakeState
	SessionHealth    *SessionHealth
}

// StatusError carries deterministic redacted diagnostics for status composition failures.
type StatusError struct {
	Code        string
	Diagnostics []Diagnostic
	cause       error
}

func (e *StatusError) Error() string {
	if e == nil {
		return "heartbeat status error"
	}
	if len(e.Diagnostics) > 0 && strings.TrimSpace(e.Diagnostics[0].Message) != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Diagnostics[0].Message)
	}
	return e.Code
}

// Unwrap exposes the sentinel cause for errors.Is callers.
func (e *StatusError) Unwrap() error {
	if e == nil || e.cause == nil {
		return ErrInvalid
	}
	return e.cause
}

// ManagedHeartbeatStatusService composes read-only Heartbeat policy status.
type ManagedHeartbeatStatusService struct {
	store          StatusStore
	healthReader   SessionHealthReader
	policyResolver PolicyResolver
}

var _ StatusService = (*ManagedHeartbeatStatusService)(nil)

// StatusOption customizes managed Heartbeat status service dependencies.
type StatusOption func(*ManagedHeartbeatStatusService)

// WithHeartbeatStatusSessionHealthReader injects the session health read model.
func WithHeartbeatStatusSessionHealthReader(reader SessionHealthReader) StatusOption {
	return func(service *ManagedHeartbeatStatusService) {
		service.healthReader = reader
	}
}

// WithHeartbeatStatusPolicyResolver injects an agent-aware policy resolver.
func WithHeartbeatStatusPolicyResolver(resolver PolicyResolver) StatusOption {
	return func(service *ManagedHeartbeatStatusService) {
		service.policyResolver = resolver
	}
}

// NewManagedHeartbeatStatusService creates the managed Heartbeat status service.
func NewManagedHeartbeatStatusService(
	persistence StatusStore,
	options ...StatusOption,
) (*ManagedHeartbeatStatusService, error) {
	if persistence == nil {
		return nil, errors.New("heartbeat: status store is required")
	}
	service := &ManagedHeartbeatStatusService{store: persistence}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

// Inspect resolves the current policy source and attaches the persisted snapshot if it exists.
func (s *ManagedHeartbeatStatusService) Inspect(
	ctx context.Context,
	req InspectRequest,
) (InspectResult, error) {
	result, _, err := s.inspectResolved(ctx, req.Target)
	if err != nil {
		return InspectResult{}, err
	}
	return result, nil
}

// Status composes current policy status with wake state and requested session health.
func (s *ManagedHeartbeatStatusService) Status(
	ctx context.Context,
	req StatusRequest,
) (StatusResult, error) {
	inspect, target, err := s.inspectResolved(ctx, req.Target)
	if err != nil {
		return StatusResult{}, err
	}
	wakeState, err := s.wakeStateForStatus(ctx, target, req.SessionID)
	if err != nil {
		return StatusResult{}, err
	}
	sessionHealth, err := s.sessionHealthForStatus(ctx, req)
	if err != nil {
		return StatusResult{}, err
	}
	return statusResultFromInspect(&inspect, wakeState, sessionHealth), nil
}

func (s *ManagedHeartbeatStatusService) inspectResolved(
	ctx context.Context,
	target AuthoringTarget,
) (InspectResult, resolvedAuthoringTarget, error) {
	resolved, policy, err := s.resolvePolicy(ctx, target)
	if err != nil {
		return InspectResult{}, resolvedAuthoringTarget{}, err
	}
	snapshot, err := s.snapshotForPolicy(ctx, resolved, &policy)
	if err != nil {
		return InspectResult{}, resolvedAuthoringTarget{}, err
	}
	return InspectResult{
		AgentName: resolved.agentName,
		Policy:    policy,
		Snapshot:  snapshot,
	}, resolved, nil
}

func (s *ManagedHeartbeatStatusService) resolvePolicy(
	ctx context.Context,
	target AuthoringTarget,
) (resolvedAuthoringTarget, ResolvedPolicy, error) {
	if s != nil && s.policyResolver != nil {
		policy, ok, err := s.policyResolver.ResolveHeartbeatPolicy(ctx, target)
		if err != nil {
			return resolvedAuthoringTarget{}, ResolvedPolicy{}, err
		}
		if ok {
			resolved, resolveErr := resolveStatusTargetForPolicy(target)
			if resolveErr != nil {
				return resolvedAuthoringTarget{}, ResolvedPolicy{}, resolveErr
			}
			return resolved, policy, nil
		}
	}
	resolved, err := resolveStatusTarget(ctx, target)
	if err != nil {
		return resolvedAuthoringTarget{}, ResolvedPolicy{}, err
	}
	policy, err := resolvePolicyForStatus(ctx, resolved)
	if err != nil {
		return resolvedAuthoringTarget{}, ResolvedPolicy{}, err
	}
	return resolved, policy, nil
}

func resolveStatusTarget(ctx context.Context, target AuthoringTarget) (resolvedAuthoringTarget, error) {
	service := ManagedHeartbeatAuthoringService{}
	return service.resolveTarget(ctx, target)
}

func resolveStatusTargetForPolicy(target AuthoringTarget) (resolvedAuthoringTarget, error) {
	if target.Config == (aghconfig.HeartbeatConfig{}) {
		target.Config = aghconfig.DefaultHeartbeatConfig()
	}
	normalized, err := normalizeAuthoringTarget(target)
	if err != nil {
		return resolvedAuthoringTarget{}, err
	}
	return resolvedAuthoringTarget{
		workspaceID:   normalized.workspaceID,
		workspaceRoot: normalized.workspaceRoot,
		agentName:     normalized.agentName,
		agentPath:     normalized.agentPath,
		config:        normalized.config,
	}, nil
}

func resolvePolicyForStatus(ctx context.Context, target resolvedAuthoringTarget) (ResolvedPolicy, error) {
	policy, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err == nil {
		return policy, nil
	}
	if errors.Is(err, ErrInvalid) {
		return policy, nil
	}
	return ResolvedPolicy{}, fmt.Errorf("heartbeat: resolve policy status: %w", err)
}

func (s *ManagedHeartbeatStatusService) snapshotForPolicy(
	ctx context.Context,
	target resolvedAuthoringTarget,
	policy *ResolvedPolicy,
) (*Snapshot, error) {
	if policy == nil || !policy.Valid || !policy.Present || strings.TrimSpace(policy.Digest) == "" {
		return nil, nil
	}
	snapshot, ok, err := s.store.FindHeartbeatSnapshotByDigest(
		ctx,
		target.workspaceID,
		target.agentName,
		policy.Digest,
	)
	if err != nil {
		return nil, fmt.Errorf("heartbeat: find status snapshot: %w", err)
	}
	if !ok {
		return nil, nil
	}
	return &snapshot, nil
}

func (s *ManagedHeartbeatStatusService) wakeStateForStatus(
	ctx context.Context,
	target resolvedAuthoringTarget,
	sessionID string,
) (*WakeState, error) {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID != "" {
		state, err := s.store.GetHeartbeatWakeState(ctx, target.workspaceID, target.agentName, trimmedSessionID)
		if err != nil {
			if errors.Is(err, ErrWakeStateNotFound) {
				return nil, nil
			}
			return nil, fmt.Errorf("heartbeat: get wake state: %w", err)
		}
		return &state, nil
	}
	states, err := s.store.ListHeartbeatWakeState(ctx, WakeStateListQuery{
		WorkspaceID: target.workspaceID,
		AgentName:   target.agentName,
		Limit:       1,
	})
	if err != nil {
		return nil, fmt.Errorf("heartbeat: list wake state: %w", err)
	}
	if len(states) == 0 {
		return nil, nil
	}
	return &states[0], nil
}

func (s *ManagedHeartbeatStatusService) sessionHealthForStatus(
	ctx context.Context,
	req StatusRequest,
) (*SessionHealth, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	if !req.IncludeSessionHealth {
		return nil, nil
	}
	if sessionID == "" {
		return nil, statusDiagnosticError(
			"session_not_found",
			"session_id is required when session health is requested",
			nil,
			ErrSessionHealthNotFound,
		)
	}
	if s.healthReader == nil {
		return nil, statusDiagnosticError(
			"session_not_found",
			"session health reader is not configured",
			nil,
			ErrSessionHealthNotFound,
		)
	}
	health, err := s.healthReader.GetSessionHealth(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionHealthNotFound) {
			return nil, statusDiagnosticError(
				"session_not_found",
				"session health was not found",
				err,
				ErrSessionHealthNotFound,
			)
		}
		return nil, fmt.Errorf("heartbeat: get session health: %w", err)
	}
	if err := health.Validate(); err != nil {
		return nil, statusDiagnosticError(
			diagnosticSessionHealthUnsupported,
			"session health contains unsupported state",
			err,
			ErrInvalidSessionHealth,
		)
	}
	return &health, nil
}

func statusResultFromInspect(
	inspect *InspectResult,
	wakeState *WakeState,
	sessionHealth *SessionHealth,
) StatusResult {
	if inspect == nil {
		return StatusResult{WakeState: wakeState, SessionHealth: sessionHealth}
	}
	policy := inspect.Policy.Status
	snapshotID := ""
	if inspect.Snapshot != nil {
		snapshotID = inspect.Snapshot.ID
	}
	return StatusResult{
		AgentName:        inspect.AgentName,
		SourcePath:       policy.SourcePath,
		Enabled:          policy.Enabled,
		Present:          policy.Present,
		Active:           policy.Active,
		Valid:            policy.Valid,
		Digest:           policy.Digest,
		ConfigDigest:     policy.ConfigDigest,
		SnapshotID:       snapshotID,
		Summary:          policy.Summary,
		Preferences:      policy.Preferences,
		Diagnostics:      cloneDiagnostics(policy.Diagnostics),
		ConfigProvenance: policy.ConfigProvenance,
		Policy:           policy,
		WakeState:        wakeState,
		SessionHealth:    sessionHealth,
	}
}

func statusDiagnosticError(code string, message string, err error, cause error) error {
	if err != nil {
		message = firstNonEmpty(message, err.Error()) + ": " + err.Error()
	}
	diagnostic := Diagnostic{
		Code:       code,
		Severity:   diagnosticError,
		Message:    diagnostics.RedactAndBound(message, 300),
		SourcePath: FileName,
	}
	return &StatusError{
		Code:        code,
		Diagnostics: []Diagnostic{diagnostic},
		cause:       cause,
	}
}
