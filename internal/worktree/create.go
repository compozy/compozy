package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/redact"
)

type CreateOptions struct {
	Name           string
	Branch         string
	BaseRef        string
	ExistingBranch string
	Path           string
	Origin         Origin
	RunID          string
	RunNamespace   string
}

type RunWorktreeRequest struct {
	TaskSlug string
	RunID    string
}

func (s *Service) Create(ctx context.Context, workspaceID string, options CreateOptions) (*Worktree, error) {
	return s.create(ctx, workspaceID, options)
}

func (s *Service) CancelCreate(ctx context.Context, workspaceID, id string) error {
	canceled, err := s.cancelAcceptedCreation(ctx, workspaceID, id)
	if canceled {
		return err
	}
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || item == nil {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("worktree: read create cancellation target: %w", err)
	}
	if item.State != StatePending {
		return ErrNotPending
	}
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return err
	}
	defer release()
	item, err = s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || item == nil {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("worktree: reread create cancellation target: %w", err)
	}
	if item.State != StatePending {
		return ErrNotPending
	}
	rollbackErr := s.rollbackCreate(context.WithoutCancel(ctx), workspace, commonDir, *item)
	if rollbackErr != nil {
		return rollbackErr
	}
	if deleteErr := s.store.DeletePending(context.WithoutCancel(ctx), workspaceID, id); deleteErr != nil {
		return deleteErr
	}
	s.invalidateDiscovery(workspaceID)
	s.emit(ctx, EventCreationCanceled, *item)
	return nil
}

func (s *Service) MaterializeForRun(
	ctx context.Context,
	workspaceID string,
	request RunWorktreeRequest,
) (*Worktree, error) {
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	settings, _ := s.worktreeSettings(workspace)
	name := RunWorktreeName(request.TaskSlug, request.RunID)
	branch := RunBranchName(settings.RunBranchNamespace, request.TaskSlug, request.RunID)
	item, err := s.create(ctx, workspaceID, CreateOptions{
		Name: name, Branch: branch, Origin: OriginPerRun, RunID: request.RunID,
		RunNamespace: settings.RunBranchNamespace,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %s",
			ErrPerRunMaterialization,
			diagnostics.RedactAndBound(err.Error(), setupDiagnosticLimit),
		)
	}
	return item, nil
}

func (s *Service) create(ctx context.Context, workspaceID string, options CreateOptions) (*Worktree, error) {
	workspace, item, commonDir, err := s.prepareCreation(ctx, workspaceID, options)
	if err != nil {
		return nil, err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return nil, err
	}
	defer release()

	if err := s.insertPendingUnderLock(ctx, workspace, commonDir, item, options); err != nil {
		return nil, err
	}
	if err := s.materializePending(ctx, workspace, &item, options); err != nil {
		rollbackErr := s.rollbackCreate(context.WithoutCancel(ctx), workspace, commonDir, item)
		if rollbackErr != nil {
			return nil, errors.Join(err, rollbackErr)
		}
		deleteErr := s.store.DeletePending(context.WithoutCancel(ctx), item.WorkspaceID, item.ID)
		if deleteErr == nil {
			s.publishCatalogEvent(CatalogEventDeleted, item)
		}
		return nil, errors.Join(err, deleteErr)
	}
	s.invalidateDiscovery(workspaceID)
	s.emit(ctx, EventCreated, item)
	return &item, nil
}

func (s *Service) prepareCreation(
	ctx context.Context,
	workspaceID string,
	options CreateOptions,
) (Workspace, Worktree, string, error) {
	if _, err := s.capability.Check(ctx); err != nil {
		return Workspace{}, Worktree{}, "", err
	}
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return Workspace{}, Worktree{}, "", err
	}
	item, err := s.prepareCreate(ctx, workspace, options)
	if err != nil {
		return Workspace{}, Worktree{}, "", err
	}
	if err := s.refuseRemovingBranch(ctx, workspace.ID, item.Branch); err != nil {
		return Workspace{}, Worktree{}, "", err
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return Workspace{}, Worktree{}, "", err
	}
	return workspace, item, commonDir, nil
}

func (s *Service) insertPendingUnderLock(
	ctx context.Context,
	workspace Workspace,
	commonDir string,
	item Worktree,
	options CreateOptions,
) error {
	if err := s.validateCreateUnderLock(ctx, workspace, commonDir, item, options); err != nil {
		return err
	}
	if err := s.dispatchGate(ctx, EventPreCreate, eventPayloadFromWorktree(item, workspace.Root)); err != nil {
		return err
	}
	if err := s.store.Insert(ctx, item); err != nil {
		return mapInsertError(err, item)
	}
	s.publishCatalogEvent(CatalogEventUpserted, item)
	return nil
}

func (s *Service) refuseRemovingBranch(ctx context.Context, workspaceID, branch string) error {
	rows, err := s.store.List(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("worktree: inspect branch removal fence: %w", err)
	}
	for _, row := range rows {
		if row.State == StateRemoving && row.Branch == branch {
			return refusal(ErrBranchHeld, row.Path)
		}
	}
	return nil
}

func (s *Service) prepareCreate(
	ctx context.Context,
	workspace Workspace,
	options CreateOptions,
) (Worktree, error) {
	name := strings.TrimSpace(options.Name)
	if name == "" {
		rows, err := s.store.List(ctx, workspace.ID)
		if err != nil {
			return Worktree{}, fmt.Errorf("worktree: list names: %w", err)
		}
		taken := make(map[string]struct{}, len(rows))
		for _, row := range rows {
			taken[row.Name] = struct{}{}
		}
		name = GenerateName(func(candidate string) bool { _, exists := taken[candidate]; return exists })
	}
	names := DeriveNames(name)
	if existing, getErr := s.store.Get(ctx, workspace.ID, names.Directory); getErr == nil && existing != nil {
		return Worktree{}, refusal(ErrNameTaken, names.Directory)
	} else if getErr != nil && !errors.Is(getErr, ErrNotFound) {
		return Worktree{}, fmt.Errorf("worktree: check name availability: %w", getErr)
	}
	branch := strings.TrimSpace(options.Branch)
	if options.ExistingBranch != "" {
		branch = strings.TrimSpace(options.ExistingBranch)
	}
	if branch == "" {
		branch = names.Branch
	}
	path := strings.TrimSpace(options.Path)
	_, placementRoot := s.worktreeSettings(workspace)
	if path == "" {
		path = filepath.Join(placementRoot, SanitizeName(workspace.Name), names.Directory)
	}
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return Worktree{}, fmt.Errorf("worktree: resolve create path: %w", err)
	}
	path, err = canonicalFuturePath(path)
	if err != nil {
		if options.Path == "" {
			return Worktree{}, refusal(ErrConfigInvalid, "worktrees.root cannot resolve the derived path: "+err.Error())
		}
		return Worktree{}, refusal(ErrPathExists, "explicit path cannot be resolved: "+err.Error())
	}
	id, err := s.newID("wt")
	if err != nil {
		return Worktree{}, err
	}
	origin := options.Origin
	if origin == "" {
		origin = OriginManual
	}
	now := s.now().UTC()
	return Worktree{
		ID: id, WorkspaceID: workspace.ID, Name: names.Directory, Branch: branch, Path: path,
		State: StatePending, Origin: origin, SetupState: SetupNone, BaseRef: strings.TrimSpace(options.BaseRef),
		RunID: strings.TrimSpace(options.RunID), RunNamespace: strings.TrimSpace(options.RunNamespace),
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Service) validateCreateUnderLock(
	ctx context.Context,
	workspace Workspace,
	commonDir string,
	item Worktree,
	options CreateOptions,
) error {
	if existing, getErr := s.store.Get(ctx, workspace.ID, item.Name); getErr == nil && existing != nil {
		return refusal(ErrNameTaken, item.Name)
	} else if getErr != nil && !errors.Is(getErr, ErrNotFound) {
		return fmt.Errorf("worktree: recheck name availability: %w", getErr)
	}
	if _, err := os.Lstat(item.Path); err == nil {
		return refusal(ErrPathExists, item.Path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("worktree: inspect create path: %w", err)
	}
	if err := s.validateCreatePath(ctx, workspace, item.Path, options.Path != "", item.ID); err != nil {
		return err
	}
	entries, err := s.readWorktreeList(ctx, workspace.Root, commonDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Branch != item.Branch {
			continue
		}
		if entry.Main {
			return refusal(ErrBranchCheckedOutAtRoot, item.Branch)
		}
		return refusal(ErrBranchHeld, entry.Path)
	}
	ref := "refs/heads/" + item.Branch
	_, _, branchProbeErr := s.runner.Run(ctx, workspace.Root, "show-ref", "--verify", "--quiet", ref)
	if options.ExistingBranch != "" {
		if branchProbeErr != nil {
			return refusal(ErrBaseRefNotFound, item.Branch)
		}
	} else if branchProbeErr == nil {
		return refusal(ErrBranchHeld, item.Branch)
	}
	if _, stderr, err := s.runner.Run(ctx, workspace.Root, "rev-parse", "--verify", "HEAD"); err != nil {
		return refusal(ErrRepoHasNoCommits, diagnostics.RedactAndBound(string(stderr), 2048))
	}
	return nil
}

func (s *Service) materializePending(
	ctx context.Context,
	workspace Workspace,
	item *Worktree,
	options CreateOptions,
) error {
	_, placementRoot := s.worktreeSettings(workspace)
	explicitPath := options.Path != ""
	item.CreatedBranch = options.ExistingBranch == ""
	if item.CreatedBranch {
		baseRef, baseHead, err := s.resolveBaseRef(ctx, workspace.Root, item.BaseRef)
		if err != nil {
			return err
		}
		item.BaseRef, item.CreatedHead = baseRef, baseHead
	} else {
		head, stderr, err := s.runner.Run(ctx, workspace.Root, "rev-parse", "--verify", item.Branch+"^{commit}")
		if err != nil {
			return refusal(ErrBaseRefNotFound, diagnostics.RedactAndBound(string(stderr), 2048))
		}
		item.CreatedHead = strings.TrimSpace(string(head))
	}
	if err := s.setPending(ctx, item, PhaseBranch); err != nil {
		return err
	}
	if err := s.ensurePlacementParent(ctx, workspace, *item, placementRoot, explicitPath); err != nil {
		return err
	}
	if item.CreatedBranch {
		if err := s.revalidateMutationPath(ctx, workspace, *item, placementRoot, explicitPath); err != nil {
			return err
		}
		if _, stderr, err := s.runner.Run(ctx, workspace.Root, "branch", item.Branch, item.BaseRef); err != nil {
			return refusal(ErrBaseRefNotFound, diagnostics.RedactAndBound(string(stderr), 2048))
		}
	}
	if err := s.setPending(ctx, item, PhaseCheckout); err != nil {
		return err
	}
	if err := s.validateCreatePath(ctx, workspace, item.Path, options.Path != "", item.ID); err != nil {
		return err
	}
	if _, stderr, err := s.runner.Run(ctx, workspace.Root, "worktree", "add", item.Path, item.Branch); err != nil {
		return fmt.Errorf("worktree: add checkout: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	gitDir, err := s.resolveGitDir(ctx, item.Path)
	if err != nil {
		return err
	}
	item.GitDir = gitDir
	if item.CreatedBranch {
		key := "branch." + item.Branch + ".gh-merge-base"
		if err := s.revalidateMutationPath(ctx, workspace, *item, placementRoot, explicitPath); err != nil {
			return err
		}
		if _, stderr, err := s.runner.Run(ctx, workspace.Root, "config", key, item.BaseRef); err != nil {
			return fmt.Errorf("worktree: record branch base: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
		}
	}
	if err := s.setPending(ctx, item, PhaseCopy); err != nil {
		return err
	}
	if err := s.copyBootstrapFiles(ctx, workspace, *item, explicitPath); err != nil {
		return err
	}
	if err := s.setPending(ctx, item, PhaseSetup); err != nil {
		return err
	}
	setupState, setupError := s.runSetup(ctx, workspace, *item, explicitPath)
	completedAt := s.now().UTC()
	if err := s.store.CompleteCreate(ctx, item.WorkspaceID, item.ID, setupState, setupError, completedAt); err != nil {
		return fmt.Errorf("worktree: complete creation: %w", err)
	}
	item.State, item.PendingPhase = StateReady, PhaseNone
	item.SetupState, item.SetupError = setupState, setupError
	item.UpdatedAt = completedAt
	if setupState == SetupFailed {
		s.emit(ctx, EventSetupFailed, *item)
	}
	return nil
}

func (s *Service) setPending(ctx context.Context, item *Worktree, phase PendingPhase) error {
	item.PendingPhase = phase
	item.UpdatedAt = s.now().UTC()
	err := s.store.UpdatePending(ctx, item.WorkspaceID, item.ID, PendingUpdate{
		Phase: phase, Branch: item.Branch, GitDir: item.GitDir, BaseRef: item.BaseRef,
		CreatedBranch: item.CreatedBranch, RunNamespace: item.RunNamespace, CreatedHead: item.CreatedHead,
	})
	if err == nil {
		s.publishCatalogEvent(CatalogEventUpserted, *item)
	}
	return err
}

func mapInsertError(err error, item Worktree) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "path") {
		return refusal(ErrPathExists, item.Path)
	}
	if strings.Contains(message, "name") || strings.Contains(message, "unique") {
		return refusal(ErrNameTaken, item.Name)
	}
	return fmt.Errorf("worktree: insert pending row: %w", err)
}

func eventPayloadFromWorktree(item Worktree, workspaceRoot string) HookWorktree {
	return HookWorktree{
		WorktreeID: item.ID, WorkspaceID: item.WorkspaceID, Name: redact.String(item.Name),
		WorkspaceRoot: redact.String(workspaceRoot), Branch: redact.String(item.Branch),
		Path: redact.String(item.Path), Origin: item.Origin,
	}
}

func (s *Service) rollbackCreate(
	ctx context.Context,
	workspace Workspace,
	commonDir string,
	item Worktree,
) error {
	var rollbackErr error
	if item.PendingPhase == PhaseCheckout || item.PendingPhase == PhaseCopy || item.PendingPhase == PhaseSetup || item.GitDir != "" {
		_, stderr, err := s.runner.Run(ctx, workspace.Root, "--git-dir", commonDir, "worktree", "remove", "--force", item.Path)
		if err != nil {
			_, statErr := os.Lstat(item.Path)
			if !errors.Is(statErr, os.ErrNotExist) {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf(
					"worktree: roll back checkout: %s: %w",
					diagnostics.RedactAndBound(string(stderr), 2048), err,
				))
				if statErr != nil {
					rollbackErr = errors.Join(
						rollbackErr,
						fmt.Errorf("worktree: inspect failed checkout rollback path: %w", statErr),
					)
				}
			}
		}
	}
	if item.CreatedBranch && item.CreatedHead != "" {
		ref := "refs/heads/" + item.Branch
		_, stderr, err := s.runner.Run(ctx, workspace.Root, "update-ref", "-d", ref, item.CreatedHead)
		if err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf(
				"worktree: roll back branch: %s: %w",
				diagnostics.RedactAndBound(string(stderr), 2048), err,
			))
		}
	}
	return rollbackErr
}
