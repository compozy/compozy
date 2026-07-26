package soul

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
	authoringPathEscapeKey = "path_escape"
)

const (
	diagnosticSoulConflict    = "soul_conflict"
	diagnosticSoulMissing     = "soul_missing"
	diagnosticAgentNotFound   = "agent_not_found"
	diagnosticSoulPathError   = "soul_path_error"
	diagnosticRevisionMissing = "revision_not_found"
)

var (
	// ErrAuthoringConflict reports a stale or missing expected digest.
	ErrAuthoringConflict = errors.New("soul: authoring conflict")
	// ErrAuthoringAgentNotFound reports a target agent that cannot be resolved.
	ErrAuthoringAgentNotFound = errors.New("soul: authoring agent not found")
	// ErrAuthoringPathRejected reports a managed SOUL.md path that is unsafe to mutate.
	ErrAuthoringPathRejected = errors.New("soul: authoring path rejected")
	// ErrAuthoringMissing reports a mutation request for an absent SOUL.md.
	ErrAuthoringMissing = errors.New("soul: authored file missing")
)

// AuthoringService is the only managed mutation boundary for SOUL.md.
type AuthoringService interface {
	Validate(ctx context.Context, req ValidateRequest) (ValidateResult, error)
	Put(ctx context.Context, req PutRequest) (MutationResult, error)
	Delete(ctx context.Context, req DeleteRequest) (MutationResult, error)
	History(ctx context.Context, req HistoryRequest) (HistoryResult, error)
	Rollback(ctx context.Context, req RollbackRequest) (MutationResult, error)
}

// AuthoringTarget identifies the workspace and agent whose SOUL.md is managed.
type AuthoringTarget struct {
	WorkspaceID   string
	WorkspaceRoot string
	AgentName     string
	AgentPath     string
	Config        aghconfig.SoulConfig
	ConfigSource  string
}

// AuthoringIdentity records actor or origin metadata for revision rows.
type AuthoringIdentity struct {
	Kind string
	Ref  string
}

// ValidateRequest validates either the current SOUL.md or the provided body.
type ValidateRequest struct {
	Target AuthoringTarget
	Body   *string
}

// ValidateResult is the transport-neutral validation response.
type ValidateResult struct {
	Soul ResolvedSoul
}

// PutRequest creates or updates SOUL.md through managed authoring.
type PutRequest struct {
	Target         AuthoringTarget
	Body           string
	ExpectedDigest string
	Actor          AuthoringIdentity
	Origin         AuthoringIdentity
}

// DeleteRequest removes SOUL.md through managed authoring.
type DeleteRequest struct {
	Target         AuthoringTarget
	ExpectedDigest string
	Actor          AuthoringIdentity
	Origin         AuthoringIdentity
}

// HistoryRequest lists managed SOUL.md authoring revisions.
type HistoryRequest struct {
	Target AuthoringTarget
	Limit  int
}

// RollbackRequest restores a prior revision body through managed authoring.
type RollbackRequest struct {
	Target         AuthoringTarget
	RevisionID     string
	ExpectedDigest string
	Actor          AuthoringIdentity
	Origin         AuthoringIdentity
}

// MutationResult returns the resolved post-mutation state and audit row.
type MutationResult struct {
	Soul     ResolvedSoul
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
		return "soul authoring error"
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

// ManagedSoulAuthoringService coordinates validation, filesystem mutation, and revision storage.
type ManagedSoulAuthoringService struct {
	store AuthoringStore
	now   func() time.Time
	newID func(prefix string) string
	mode  os.FileMode
	mu    sync.Mutex
}

var _ AuthoringService = (*ManagedSoulAuthoringService)(nil)

// AuthoringOption customizes managed authoring service dependencies.
type AuthoringOption func(*ManagedSoulAuthoringService)

// WithSoulAuthoringClock injects deterministic timestamps.
func WithSoulAuthoringClock(clock func() time.Time) AuthoringOption {
	return func(service *ManagedSoulAuthoringService) {
		if clock != nil {
			service.now = clock
		}
	}
}

// WithSoulAuthoringIDGenerator injects deterministic snapshot and revision ids.
func WithSoulAuthoringIDGenerator(generator func(prefix string) string) AuthoringOption {
	return func(service *ManagedSoulAuthoringService) {
		if generator != nil {
			service.newID = generator
		}
	}
}

// NewManagedSoulAuthoringService creates the managed SOUL.md authoring service.
func NewManagedSoulAuthoringService(
	persistence AuthoringStore,
	options ...AuthoringOption,
) (*ManagedSoulAuthoringService, error) {
	if persistence == nil {
		return nil, errors.New("soul: authoring store is required")
	}
	service := &ManagedSoulAuthoringService{
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

// Validate validates current or proposed SOUL.md content without mutating files or storage.
func (s *ManagedSoulAuthoringService) Validate(
	ctx context.Context,
	req ValidateRequest,
) (ValidateResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return ValidateResult{}, err
	}
	if req.Body != nil {
		resolved, err := Parse(ctx, ParseRequest{
			SourcePath:    target.soulPath,
			WorkspaceRoot: target.workspaceRoot,
			Content:       []byte(*req.Body),
			Config:        target.config,
		})
		if err != nil {
			return ValidateResult{Soul: resolved}, authoringSoulResolutionError(err, resolved.Diagnostics)
		}
		return ValidateResult{Soul: resolved}, nil
	}
	resolved, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err != nil {
		return ValidateResult{Soul: resolved}, authoringSoulResolutionError(err, resolved.Diagnostics)
	}
	return ValidateResult{Soul: resolved}, nil
}

// Put creates or updates SOUL.md through CAS-protected managed authoring.
func (s *ManagedSoulAuthoringService) Put(ctx context.Context, req PutRequest) (MutationResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentSoulForMutation(ctx, target)
	if err != nil {
		return MutationResult{}, err
	}
	if err := validateExpectedDigest(&current.resolved, req.ExpectedDigest, target.sourcePath); err != nil {
		return MutationResult{}, err
	}
	proposed, err := Parse(ctx, ParseRequest{
		SourcePath:    target.soulPath,
		WorkspaceRoot: target.workspaceRoot,
		Content:       []byte(req.Body),
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Soul: proposed}, authoringSoulResolutionError(err, proposed.Diagnostics)
	}
	if err := s.verifyUnchangedSoul(ctx, target, &current); err != nil {
		return MutationResult{}, err
	}
	if err := fileutil.AtomicWriteFile(target.soulPath, []byte(req.Body), s.mode); err != nil {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticSoulPathError,
			target.sourcePath,
			"SOUL.md could not be written",
			err,
			ErrAuthoringPathRejected,
		)
	}
	return s.persistPostWrite(
		ctx,
		target,
		current.resolved.Digest,
		RevisionActionPut,
		req.Body,
		req.Actor,
		req.Origin,
	)
}

// Delete removes SOUL.md through CAS-protected managed authoring.
func (s *ManagedSoulAuthoringService) Delete(
	ctx context.Context,
	req DeleteRequest,
) (MutationResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentSoulForMutation(ctx, target)
	if err != nil {
		return MutationResult{}, err
	}
	if !current.resolved.Present {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticSoulMissing,
			target.sourcePath,
			"SOUL.md is not present",
			nil,
			ErrAuthoringMissing,
		)
	}
	if err := validateExpectedDigest(&current.resolved, req.ExpectedDigest, target.sourcePath); err != nil {
		return MutationResult{}, err
	}
	if err := s.verifyUnchangedSoul(ctx, target, &current); err != nil {
		return MutationResult{}, err
	}
	if err := fileutil.AtomicRemoveFile(target.soulPath); err != nil {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticSoulPathError,
			target.sourcePath,
			"SOUL.md could not be deleted",
			err,
			ErrAuthoringPathRejected,
		)
	}
	resolved, err := Resolve(ctx, ResolveRequest{
		AgentPath:     target.agentPath,
		WorkspaceRoot: target.workspaceRoot,
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Soul: resolved}, authoringSoulResolutionError(err, resolved.Diagnostics)
	}
	revision, err := s.appendRevision(
		ctx,
		target,
		RevisionActionDelete,
		current.resolved.Digest,
		"",
		"",
		resolved.Diagnostics,
		req.Actor,
		req.Origin,
	)
	if err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Soul: resolved, Revision: revision}, nil
}

// History lists managed SOUL.md revision history.
func (s *ManagedSoulAuthoringService) History(
	ctx context.Context,
	req HistoryRequest,
) (HistoryResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return HistoryResult{}, err
	}
	revisions, err := s.store.ListSoulRevisions(ctx, RevisionListQuery{
		WorkspaceID: target.workspaceID,
		AgentName:   target.agentName,
		Limit:       req.Limit,
	})
	if err != nil {
		return HistoryResult{}, fmt.Errorf("soul: list authoring history: %w", err)
	}
	return HistoryResult{Revisions: revisions}, nil
}

// Rollback restores a prior revision body through the same validation and CAS path as Put.
func (s *ManagedSoulAuthoringService) Rollback(
	ctx context.Context,
	req RollbackRequest,
) (MutationResult, error) {
	target, err := s.resolveTarget(ctx, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.currentSoulForMutation(ctx, target)
	if err != nil {
		return MutationResult{}, err
	}
	if err := validateExpectedDigest(&current.resolved, req.ExpectedDigest, target.sourcePath); err != nil {
		return MutationResult{}, err
	}
	selected, err := s.store.FindSoulRevisionForRollback(ctx, RollbackLookup{
		WorkspaceID: target.workspaceID,
		AgentName:   target.agentName,
		RevisionID:  strings.TrimSpace(req.RevisionID),
	})
	if err != nil {
		if errors.Is(err, ErrRevisionNotFound) {
			return MutationResult{}, authoringDiagnosticError(
				diagnosticRevisionMissing,
				target.sourcePath,
				"SOUL.md rollback revision was not found",
				err,
				ErrRevisionNotFound,
			)
		}
		return MutationResult{}, fmt.Errorf("soul: find rollback revision: %w", err)
	}
	proposed, err := Parse(ctx, ParseRequest{
		SourcePath:    target.soulPath,
		WorkspaceRoot: target.workspaceRoot,
		Content:       []byte(selected.Body),
		Config:        target.config,
	})
	if err != nil {
		return MutationResult{Soul: proposed}, authoringSoulResolutionError(err, proposed.Diagnostics)
	}
	if err := s.verifyUnchangedSoul(ctx, target, &current); err != nil {
		return MutationResult{}, err
	}
	if err := fileutil.AtomicWriteFile(target.soulPath, []byte(selected.Body), s.mode); err != nil {
		return MutationResult{}, authoringDiagnosticError(
			diagnosticSoulPathError,
			target.sourcePath,
			"SOUL.md could not be rolled back",
			err,
			ErrAuthoringPathRejected,
		)
	}
	return s.persistPostWrite(
		ctx,
		target,
		current.resolved.Digest,
		RevisionActionRollback,
		selected.Body,
		req.Actor,
		req.Origin,
	)
}
