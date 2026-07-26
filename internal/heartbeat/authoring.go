package heartbeat

import (
	"context"

	"errors"
	"fmt"
	"os"

	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/fileutil"
	"github.com/compozy/agh/internal/store"
)

const (
	authoringEnabledKey             = "enabled"
	authoringHeartbeatPathEscapeKey = "heartbeat_path_escape"
	authoringVersionKey             = "version"
)

const (
	diagnosticHeartbeatConflict        = "heartbeat_conflict"
	diagnosticHeartbeatNoPolicy        = "heartbeat_no_policy"
	diagnosticHeartbeatAgentNotFound   = "agent_not_found"
	diagnosticHeartbeatPathError       = "heartbeat_path_error"
	diagnosticHeartbeatRevisionMissing = "revision_not_found"
)

var (
	// ErrAuthoringConflict reports a stale or missing expected digest.
	ErrAuthoringConflict = errors.New("heartbeat: authoring conflict")
	// ErrAuthoringAgentNotFound reports a target agent that cannot be resolved.
	ErrAuthoringAgentNotFound = errors.New("heartbeat: authoring agent not found")
	// ErrAuthoringPathRejected reports a managed HEARTBEAT.md path that is unsafe to mutate.
	ErrAuthoringPathRejected = errors.New("heartbeat: authoring path rejected")
	// ErrAuthoringNoPolicy reports a mutation request for an absent HEARTBEAT.md.
	ErrAuthoringNoPolicy = errors.New("heartbeat: authored policy missing")
)

// AuthoringService is the only managed mutation boundary for HEARTBEAT.md.
type AuthoringService interface {
	Validate(ctx context.Context, req ValidateRequest) (ValidateResult, error)
	Put(ctx context.Context, req PutRequest) (MutationResult, error)
	Delete(ctx context.Context, req DeleteRequest) (MutationResult, error)
	History(ctx context.Context, req HistoryRequest) (HistoryResult, error)
	Rollback(ctx context.Context, req RollbackRequest) (MutationResult, error)
}

// AuthoringTarget identifies the workspace and agent whose HEARTBEAT.md is managed.
type AuthoringTarget struct {
	WorkspaceID   string
	WorkspaceRoot string
	AgentName     string
	AgentPath     string
	Config        aghconfig.HeartbeatConfig
}

// AuthoringIdentity records actor metadata for revision rows.
type AuthoringIdentity struct {
	Kind string
	Ref  string
}

// ValidateRequest validates either the current HEARTBEAT.md or the provided body.
type ValidateRequest struct {
	Target AuthoringTarget
	Body   *string
}

// ValidateResult is the transport-neutral validation response.
type ValidateResult struct {
	Policy ResolvedPolicy
}

// PutRequest creates or updates HEARTBEAT.md through managed authoring.
type PutRequest struct {
	Target         AuthoringTarget
	Body           string
	ExpectedDigest string
	Actor          AuthoringIdentity
}

// DeleteRequest removes HEARTBEAT.md through managed authoring.
type DeleteRequest struct {
	Target         AuthoringTarget
	ExpectedDigest string
	Actor          AuthoringIdentity
}

// HistoryRequest lists managed HEARTBEAT.md authoring revisions.
type HistoryRequest struct {
	Target AuthoringTarget
	Limit  int
}

// RollbackRequest restores a prior revision or snapshot body through managed authoring.
type RollbackRequest struct {
	Target         AuthoringTarget
	RevisionID     string
	TargetDigest   string
	ExpectedDigest string
	Actor          AuthoringIdentity
}

// MutationResult returns the resolved post-mutation state and audit row.
type MutationResult struct {
	Policy   ResolvedPolicy
	Snapshot Snapshot
	Revision Revision
}

// HistoryResult returns managed authoring history in newest-first order.
type HistoryResult struct {
	Revisions []Revision
}

// AuthoringError carries deterministic redacted diagnostics for authoring failures.
type AuthoringError struct {
	Code        string
	Diagnostics []Diagnostic
	cause       error
}

func (e *AuthoringError) Error() string {
	if e == nil {
		return "heartbeat authoring error"
	}
	if len(e.Diagnostics) > 0 && strings.TrimSpace(e.Diagnostics[0].Message) != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Diagnostics[0].Message)
	}
	return e.Code
}

// Unwrap exposes the sentinel cause for errors.Is callers.
func (e *AuthoringError) Unwrap() error {
	if e == nil || e.cause == nil {
		return ErrInvalid
	}
	return e.cause
}

// ManagedHeartbeatAuthoringService coordinates validation, filesystem mutation, and revision storage.
type ManagedHeartbeatAuthoringService struct {
	store AuthoringStore
	now   func() time.Time
	newID func(prefix string) string
	mode  os.FileMode
	mu    sync.Mutex
}

var _ AuthoringService = (*ManagedHeartbeatAuthoringService)(nil)

// AuthoringOption customizes managed Heartbeat authoring service dependencies.
type AuthoringOption func(*ManagedHeartbeatAuthoringService)

// WithHeartbeatAuthoringClock injects deterministic timestamps.
func WithHeartbeatAuthoringClock(clock func() time.Time) AuthoringOption {
	return func(service *ManagedHeartbeatAuthoringService) {
		if clock != nil {
			service.now = clock
		}
	}
}

// WithHeartbeatAuthoringIDGenerator injects deterministic snapshot and revision ids.
func WithHeartbeatAuthoringIDGenerator(generator func(prefix string) string) AuthoringOption {
	return func(service *ManagedHeartbeatAuthoringService) {
		if generator != nil {
			service.newID = generator
		}
	}
}

// NewManagedHeartbeatAuthoringService creates the managed HEARTBEAT.md authoring service.
func NewManagedHeartbeatAuthoringService(
	persistence AuthoringStore,
	options ...AuthoringOption,
) (*ManagedHeartbeatAuthoringService, error) {
	if persistence == nil {
		return nil, errors.New("heartbeat: authoring store is required")
	}
	service := &ManagedHeartbeatAuthoringService{
		store: persistence,
		now: func() time.Time {
			return time.Now().UTC()
		},
		newID: store.NewID,
		mode:  0o644,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

// Validate validates current or proposed HEARTBEAT.md content without mutating files or storage.
func (s *ManagedHeartbeatAuthoringService) Validate(
	ctx context.Context,
	req ValidateRequest,
) (ValidateResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return ValidateResult{}, err
	}
	if req.Body != nil {
		resolved, err := Parse(ctx, ParseRequest{
			SourcePath:    target.heartbeatPath,
			WorkspaceRoot: target.workspaceRoot,
			Content:       []byte(*req.Body),
			Config:        target.config,
		})
		if err != nil {
			return ValidateResult{Policy: resolved}, authoringInvalidError(resolved.Diagnostics)
		}
		return ValidateResult{Policy: resolved}, nil
	}
	resolved, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err != nil {
		return ValidateResult{Policy: resolved}, authoringInvalidError(resolved.Diagnostics)
	}
	return ValidateResult{Policy: resolved}, nil
}

// Put creates or updates HEARTBEAT.md through CAS-protected managed authoring.
func (s *ManagedHeartbeatAuthoringService) Put(ctx context.Context, req PutRequest) (MutationResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentHeartbeatForMutation(ctx, target)
	if err != nil {
		return MutationResult{}, err
	}
	if err := validateExpectedDigest(&current, req.ExpectedDigest, target.sourcePath); err != nil {
		return MutationResult{}, err
	}
	proposed, err := Parse(ctx, ParseRequest{
		SourcePath:    target.heartbeatPath,
		WorkspaceRoot: target.workspaceRoot,
		Content:       []byte(req.Body),
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Policy: proposed}, authoringInvalidError(proposed.Diagnostics)
	}
	if err := s.verifyUnchangedHeartbeat(ctx, target, &current); err != nil {
		return MutationResult{}, err
	}
	if err := fileutil.AtomicWriteFile(target.heartbeatPath, []byte(req.Body), s.mode); err != nil {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticHeartbeatPathError,
			target.sourcePath,
			"HEARTBEAT.md could not be written",
			err,
			ErrAuthoringPathRejected,
		)
	}
	return s.persistPostWrite(ctx, target, current.Digest, RevisionOperationWrite, req.Body, req.Actor)
}

// Delete removes HEARTBEAT.md through CAS-protected managed authoring.
func (s *ManagedHeartbeatAuthoringService) Delete(
	ctx context.Context,
	req DeleteRequest,
) (MutationResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentHeartbeatForMutation(ctx, target)
	if err != nil {
		return MutationResult{}, err
	}
	if !current.Present {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticHeartbeatNoPolicy,
			target.sourcePath,
			"HEARTBEAT.md is not present",
			nil,
			ErrAuthoringNoPolicy,
		)
	}
	if err := validateExpectedDigest(&current, req.ExpectedDigest, target.sourcePath); err != nil {
		return MutationResult{}, err
	}
	if err := s.verifyUnchangedHeartbeat(ctx, target, &current); err != nil {
		return MutationResult{}, err
	}
	if err := fileutil.AtomicRemoveFile(target.heartbeatPath); err != nil {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticHeartbeatPathError,
			target.sourcePath,
			"HEARTBEAT.md could not be deleted",
			err,
			ErrAuthoringPathRejected,
		)
	}
	return s.persistPostDelete(ctx, target, current.Digest, RevisionOperationDelete, req.Actor)
}

// History lists managed HEARTBEAT.md revision history.
func (s *ManagedHeartbeatAuthoringService) History(
	ctx context.Context,
	req HistoryRequest,
) (HistoryResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return HistoryResult{}, err
	}
	revisions, err := s.store.ListHeartbeatRevisions(ctx, RevisionListQuery{
		WorkspaceID: target.workspaceID,
		AgentName:   target.agentName,
		Limit:       req.Limit,
	})
	if err != nil {
		return HistoryResult{}, fmt.Errorf("heartbeat: list authoring history: %w", err)
	}
	return HistoryResult{Revisions: revisions}, nil
}

// Rollback restores a prior revision or snapshot body through the same validation and CAS path as Put.
func (s *ManagedHeartbeatAuthoringService) Rollback(
	ctx context.Context,
	req RollbackRequest,
) (MutationResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentHeartbeatForMutation(ctx, target)
	if err != nil {
		return MutationResult{}, err
	}
	if err := validateExpectedDigest(&current, req.ExpectedDigest, target.sourcePath); err != nil {
		return MutationResult{}, err
	}
	rollback, err := s.rollbackTarget(ctx, target, req)
	if err != nil {
		return MutationResult{}, err
	}
	if rollback.absent {
		if err := s.verifyUnchangedHeartbeat(ctx, target, &current); err != nil {
			return MutationResult{}, err
		}
		if err := fileutil.AtomicRemoveFile(target.heartbeatPath); err != nil {
			return MutationResult{}, authoringDiagnosticError(
				diagnosticHeartbeatPathError,
				target.sourcePath,
				"HEARTBEAT.md could not be rolled back",
				err,
				ErrAuthoringPathRejected,
			)
		}
		return s.persistPostDelete(ctx, target, current.Digest, RevisionOperationRollback, req.Actor)
	}
	proposed, err := Parse(ctx, ParseRequest{
		SourcePath:    target.heartbeatPath,
		WorkspaceRoot: target.workspaceRoot,
		Content:       []byte(rollback.body),
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Policy: proposed}, authoringInvalidError(proposed.Diagnostics)
	}
	if err := s.verifyUnchangedHeartbeat(ctx, target, &current); err != nil {
		return MutationResult{}, err
	}
	if err := fileutil.AtomicWriteFile(target.heartbeatPath, []byte(rollback.body), s.mode); err != nil {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticHeartbeatPathError,
			target.sourcePath,
			"HEARTBEAT.md could not be rolled back",
			err,
			ErrAuthoringPathRejected,
		)
	}
	return s.persistPostWrite(ctx, target, current.Digest, RevisionOperationRollback, rollback.body, req.Actor)
}
