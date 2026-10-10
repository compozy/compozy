package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/worktree"
)

const (
	subagentMaterializationFailed = "materialization_failed"
	subagentPRStatusOpen          = "open"
	subagentPRStatusDraft         = "draft"
	subagentPRStatusMerged        = "merged"
	subagentPRStatusClosed        = "closed"
)

type subagentWorktreeService interface {
	executionWorktreeService
	List(context.Context, string, bool) (*worktree.Listing, error)
	Status(context.Context, string, string, bool) (*worktree.Status, error)
	StatusDetails(context.Context, string, string, bool, bool) (*worktree.StatusDetails, error)
	CommitsAheadOf(context.Context, string, string, string) (int, error)
	ResolveCommit(context.Context, string, string) (string, error)
}
type daemonSubagentWorktrees struct {
	lookup func() subagentWorktreeService
}

var _ session.SubagentWorktrees = daemonSubagentWorktrees{}

func (a daemonSubagentWorktrees) Provision(
	ctx context.Context,
	req session.SubagentWorktreeRequest,
) (session.SubagentWorktree, error) {
	svc := a.lookup()
	if svc == nil {
		return session.SubagentWorktree{}, &session.ErrSubagentIsolationFailed{Cause: "unavailable"}
	}
	old, err := a.FindByRun(ctx, req.WorkspaceID, req.SubagentID)
	if err != nil {
		return session.SubagentWorktree{}, &session.ErrSubagentIsolationFailed{
			Cause: subagentMaterializationFailed,
			Err:   err,
		}
	}
	if old != nil {
		return *old, nil
	}
	base, err := subagentWorktreeBase(ctx, svc, req)
	if err != nil {
		return session.SubagentWorktree{}, err
	}

	wt, err := svc.MaterializeForRun(
		ctx,
		req.WorkspaceID,
		worktree.RunWorktreeRequest{
			ProfileID: req.ProfileID,
			TaskSlug:  req.Title,
			RunID:     req.SubagentID,
			BaseRef:   base,
		},
	)
	if err != nil {
		if errors.Is(err, worktree.ErrDeniedByHook) {
			return session.SubagentWorktree{}, &session.SubagentError{
				Code:    "capability_denied",
				Message: err.Error(),
				Err:     errors.Join(session.ErrSubagentCapabilityDenied, err),
			}
		}
		cause := subagentMaterializationFailed
		if errors.Is(err, worktree.ErrBaseRefNotFound) ||
			strings.Contains(err.Error(), worktree.ErrBaseRefNotFound.Error()) {
			cause = "base_ref_not_found"
		}
		if errors.Is(err, worktree.ErrBranchHeld) || strings.Contains(err.Error(), worktree.ErrBranchHeld.Error()) {
			cause = "branch_held"
		}
		return session.SubagentWorktree{}, &session.ErrSubagentIsolationFailed{Cause: cause, Err: err}
	}
	result := subagentWorktreeFromDomain(wt)
	if wt.SetupState == worktree.SetupFailed {
		// Return the ownership anchor even if cleanup fails, so the service persists it.
		_, rollbackErr := a.SafeRollback(ctx, req.WorkspaceID, wt.ID, req.SubagentID, wt.CreatedHead)
		return result, &session.ErrSubagentIsolationFailed{
			Cause:  "setup_failed",
			Detail: diagnostics.RedactAndBound(wt.SetupError, 2048),
			Err:    rollbackErr,
		}
	}
	return result, nil
}

func subagentWorktreeBase(
	ctx context.Context,
	svc subagentWorktreeService,
	req session.SubagentWorktreeRequest,
) (string, error) {
	if req.BaseRef != "" {
		return req.BaseRef, nil
	}
	if req.CallerWorktreeID != "" {
		parent, err := svc.Get(ctx, req.WorkspaceID, req.CallerWorktreeID)
		if err != nil {
			return "", &session.ErrSubagentIsolationFailed{
				Cause: subagentMaterializationFailed,
				Err:   err,
			}
		}
		req.CallerPath = parent.Path
	}

	base, err := svc.ResolveCommit(ctx, req.CallerPath, "HEAD")
	if err != nil {
		return "", &session.ErrSubagentIsolationFailed{Cause: "base_ref_not_found", Err: err}
	}
	return base, nil
}

func subagentWorktreeFromDomain(wt *worktree.Worktree) session.SubagentWorktree {
	return session.SubagentWorktree{
		ID:      wt.ID,
		Name:    wt.Name,
		Branch:  wt.Branch,
		Path:    wt.Path,
		BaseRef: wt.BaseRef,
		BaseSHA: wt.CreatedHead,
	}
}
func (a daemonSubagentWorktrees) FindByRun(ctx context.Context, ws, id string) (*session.SubagentWorktree, error) {
	svc := a.lookup()
	if svc == nil {
		return nil, errors.New("daemon: worktree service unavailable")
	}
	rows, err := svc.List(ctx, ws, false)
	if err != nil {
		return nil, err
	}
	for _, wt := range rows.Worktrees {
		if wt.Origin == worktree.OriginPerRun && wt.RunID == id &&
			(wt.State == worktree.StatePending || wt.State == worktree.StateReady) {
			return new(subagentWorktreeFromDomain(&wt)), nil
		}
	}
	return nil, nil
}
func (a daemonSubagentWorktrees) Rollback(ctx context.Context, ws, id, run string) error {
	svc := a.lookup()
	if svc == nil {
		return errors.New("daemon: worktree service unavailable")
	}
	err := svc.RollbackRunMaterialization(ctx, ws, id, run)
	if errors.Is(err, worktree.ErrNotFound) {
		return nil
	}
	return err
}
func (a daemonSubagentWorktrees) SafeRollback(ctx context.Context, ws, id, run, base string) (bool, error) {
	svc := a.lookup()
	if svc == nil {
		return false, errors.New("daemon: worktree service unavailable")
	}
	item, err := svc.Get(ctx, ws, id)
	if errors.Is(err, worktree.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if item.State == worktree.StateRemoved || item.State == worktree.StateDismissed {
		return false, nil
	}
	// Setup failed before a child could write any work; retries must use the same force cleanup.
	if item.SetupState == worktree.SetupFailed {
		return false, a.Rollback(ctx, ws, id, run)
	}
	status, err := svc.Status(ctx, ws, id, true)
	if errors.Is(err, worktree.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status.ReadError != "" || status.DirtyFiles == nil {
		return false, fmt.Errorf("worktree: rollback status unreadable: %s", status.ReadError)
	}
	if *status.DirtyFiles > 0 {
		return true, nil
	}
	n, err := svc.CommitsAheadOf(ctx, ws, id, base)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	return false, a.Rollback(ctx, ws, id, run)
}
func (a daemonSubagentWorktrees) Observe(ctx context.Context, ws, id, base string) session.SubagentWorktreeFacts {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	facts := session.SubagentWorktreeFacts{PRStatus: daemonUnknownValue}
	svc := a.lookup()
	if svc == nil {
		return facts
	}
	status, err := svc.Status(ctx, ws, id, true)
	if err == nil && status.ReadError == "" && status.HeadSHA != nil && status.DirtyFiles != nil {
		if n, err := svc.CommitsAheadOf(ctx, ws, id, base); err == nil {
			facts.HeadSHA = *status.HeadSHA
			facts.DirtyFiles = status.DirtyFiles
			facts.CommitsAhead = new(n)
			if status.RefreshedAt != nil {
				facts.ObservedAt = *status.RefreshedAt
			}
		}
	}
	detail, err := svc.StatusDetails(ctx, ws, id, false, true)
	if err != nil || detail.ForgeStatus == nil {
		return facts
	}
	pr := detail.ForgeStatus
	facts.PRStatus = "none"
	if pr.PRNumber != nil && pr.PRState != nil {
		facts.PRStatus = strings.ToLower(strings.TrimSpace(*pr.PRState))
		if pr.Merged != nil && *pr.Merged {
			facts.PRStatus = subagentPRStatusMerged
		} else if pr.Draft != nil && *pr.Draft && facts.PRStatus == subagentPRStatusOpen {
			facts.PRStatus = subagentPRStatusDraft
		}
		switch facts.PRStatus {
		case subagentPRStatusOpen, subagentPRStatusDraft, subagentPRStatusMerged, subagentPRStatusClosed:
			facts.PRURL = pr.PRURL
			facts.PRNumber = pr.PRNumber
		default:
			facts.PRStatus = "unknown"
		}
	}
	return facts
}
