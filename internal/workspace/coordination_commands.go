package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

var (
	// ErrCoordinationScopeInvalid reports a workspace/task/run ownership mismatch.
	ErrCoordinationScopeInvalid = errors.New("workspace: network coordination scope invalid")
	// ErrCoordinationConflict reports a stale expected revision.
	ErrCoordinationConflict = errors.New("workspace: network coordination revision conflict")
)

// CoordinationRef identifies one explicit workspace or task coordination scope.
type CoordinationRef struct {
	WorkspaceID string
	ScopeKind   string
	TaskID      string
	RunID       string
}

// CoordinationEligibility is daemon-owned invitation eligibility truth.
type CoordinationEligibility struct {
	Eligible    bool
	Reason      string
	RunID       string
	Coordinator bool
	WorkerCount int
}

// CoordinationView is one committed coordination setting, invitation, and eligibility projection.
type CoordinationView struct {
	Setting     CoordinationSetting
	Invitation  CoordinationInvitation
	Eligibility CoordinationEligibility
}

// SetCoordination is a compare-and-set mutation for one explicit scope.
type SetCoordination struct {
	Ref              CoordinationRef
	Enabled          bool
	ExpectedRevision int64
}

// SetInvitation is a compare-and-set dismissal/reset mutation for one explicit scope.
type SetInvitation struct {
	Ref              CoordinationRef
	Dismissed        bool
	ExpectedRevision int64
}

// CoordinationCommands owns public coordination reads and atomic mutations.
type CoordinationCommands interface {
	Get(ctx context.Context, ref CoordinationRef, actor taskpkg.ActorContext) (CoordinationView, error)
	Set(ctx context.Context, cmd SetCoordination, actor taskpkg.ActorContext) (CoordinationView, error)
	SetInvitation(ctx context.Context, cmd SetInvitation, actor taskpkg.ActorContext) (CoordinationView, error)
}

// CoordinationCommandStore persists already-authorized coordination commands atomically.
type CoordinationCommandStore interface {
	GetCoordination(ctx context.Context, ref CoordinationRef) (CoordinationView, error)
	SetCoordination(
		ctx context.Context,
		cmd SetCoordination,
		actor taskpkg.ActorContext,
	) (CoordinationView, error)
	SetCoordinationInvitation(
		ctx context.Context,
		cmd SetInvitation,
		actor taskpkg.ActorContext,
	) (CoordinationView, error)
}

type coordinationService struct {
	store CoordinationCommandStore
}

// NewCoordinationService builds the application command boundary used by HTTP and UDS.
func NewCoordinationService(store CoordinationCommandStore) CoordinationCommands {
	if store == nil {
		return nil
	}
	return &coordinationService{store: store}
}

func (s *coordinationService) Get(
	ctx context.Context,
	ref CoordinationRef,
	actor taskpkg.ActorContext,
) (CoordinationView, error) {
	normalized, err := NormalizeCoordinationRef(ref)
	if err != nil {
		return CoordinationView{}, err
	}
	if err := authorizeCoordinationActor(actor, normalized.WorkspaceID, false); err != nil {
		return CoordinationView{}, err
	}
	return s.store.GetCoordination(ctx, normalized)
}

func (s *coordinationService) Set(
	ctx context.Context,
	cmd SetCoordination,
	actor taskpkg.ActorContext,
) (CoordinationView, error) {
	normalized, err := normalizeSetCoordination(cmd)
	if err != nil {
		return CoordinationView{}, err
	}
	if err := authorizeCoordinationActor(actor, normalized.Ref.WorkspaceID, true); err != nil {
		return CoordinationView{}, err
	}
	return s.store.SetCoordination(ctx, normalized, actor)
}

func (s *coordinationService) SetInvitation(
	ctx context.Context,
	cmd SetInvitation,
	actor taskpkg.ActorContext,
) (CoordinationView, error) {
	normalized, err := normalizeSetInvitation(cmd)
	if err != nil {
		return CoordinationView{}, err
	}
	if err := authorizeCoordinationActor(actor, normalized.Ref.WorkspaceID, true); err != nil {
		return CoordinationView{}, err
	}
	return s.store.SetCoordinationInvitation(ctx, normalized, actor)
}

// NormalizeCoordinationRef validates and canonicalizes one scope reference.
func NormalizeCoordinationRef(ref CoordinationRef) (CoordinationRef, error) {
	normalized := CoordinationRef{
		WorkspaceID: strings.TrimSpace(ref.WorkspaceID),
		ScopeKind:   strings.ToLower(strings.TrimSpace(ref.ScopeKind)),
		TaskID:      strings.TrimSpace(ref.TaskID),
		RunID:       strings.TrimSpace(ref.RunID),
	}
	if normalized.WorkspaceID == "" {
		return CoordinationRef{}, fmt.Errorf("%w: workspace_id is required", ErrCoordinationScopeInvalid)
	}
	switch normalized.ScopeKind {
	case InvitationScopeWorkspace:
		if normalized.TaskID != "" {
			return CoordinationRef{}, fmt.Errorf(
				"%w: task_id must be empty for workspace scope",
				ErrCoordinationScopeInvalid,
			)
		}
	case InvitationScopeTask:
		if normalized.TaskID == "" {
			return CoordinationRef{}, fmt.Errorf(
				"%w: task_id is required for task scope",
				ErrCoordinationScopeInvalid,
			)
		}
	default:
		return CoordinationRef{}, fmt.Errorf(
			"%w: scope must be workspace or task",
			ErrCoordinationScopeInvalid,
		)
	}
	return normalized, nil
}

func normalizeSetCoordination(cmd SetCoordination) (SetCoordination, error) {
	ref, err := NormalizeCoordinationRef(cmd.Ref)
	if err != nil {
		return SetCoordination{}, err
	}
	if cmd.ExpectedRevision < 0 {
		return SetCoordination{}, fmt.Errorf("%w: expected_revision must be non-negative", ErrCoordinationConflict)
	}
	cmd.Ref = ref
	return cmd, nil
}

func normalizeSetInvitation(cmd SetInvitation) (SetInvitation, error) {
	ref, err := NormalizeCoordinationRef(cmd.Ref)
	if err != nil {
		return SetInvitation{}, err
	}
	if cmd.ExpectedRevision < 0 {
		return SetInvitation{}, fmt.Errorf("%w: expected_revision must be non-negative", ErrCoordinationConflict)
	}
	cmd.Ref = ref
	return cmd, nil
}

func authorizeCoordinationActor(actor taskpkg.ActorContext, workspaceID string, write bool) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if !actor.Authority.Read || (write && !actor.Authority.Write) {
		return taskpkg.ErrPermissionDenied
	}
	actorWorkspace := strings.TrimSpace(actor.Scope.WorkspaceID)
	if actorWorkspace != "" && actorWorkspace != workspaceID {
		return taskpkg.ErrPermissionDenied
	}
	if write && !actor.Scope.Operator {
		return taskpkg.ErrPermissionDenied
	}
	return nil
}
