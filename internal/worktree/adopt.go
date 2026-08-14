package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func (s *Service) Adopt(ctx context.Context, workspaceID, candidatePath string) (*Worktree, error) {
	if _, err := s.capability.Check(ctx); err != nil {
		return nil, err
	}
	workspace, err := s.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	canonicalCandidate, err := fileutil.CanonicalExistingDirectory(candidatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, refusal(ErrAdoptionUnreadable, err.Error())
	}
	canonicalWorkspace, err := fileutil.CanonicalExistingDirectory(workspace.Root)
	if err != nil {
		return nil, fmt.Errorf("worktree: canonicalize workspace root: %w", err)
	}
	if canonicalCandidate == canonicalWorkspace {
		return nil, ErrAdoptionMainCheckout
	}
	commonDir, err := s.commonDir(ctx, workspace.Root)
	if err != nil {
		return nil, err
	}
	release, err := s.locks.Acquire(ctx, commonDir)
	if err != nil {
		return nil, err
	}
	defer release()
	identity, matched, err := s.inspectAdoptionCandidate(ctx, workspace, canonicalCandidate, commonDir)
	if err != nil {
		return nil, err
	}
	existing, getErr := s.store.GetByPath(ctx, workspaceID, canonicalCandidate)
	if getErr == nil && existing != nil {
		return s.reuseAdoptedWorktree(ctx, existing, identity.adminGitDir)
	}
	if getErr != nil && !errors.Is(getErr, ErrNotFound) {
		return nil, fmt.Errorf("worktree: inspect adopted path: %w", getErr)
	}
	return s.insertAdoptedWorktree(ctx, workspaceID, canonicalCandidate, identity.adminGitDir, *matched)
}

func (s *Service) insertAdoptedWorktree(
	ctx context.Context,
	workspaceID string,
	candidatePath string,
	adminGitDir string,
	entry GitWorktree,
) (*Worktree, error) {
	rows, err := s.store.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("worktree: list adoption names: %w", err)
	}
	taken := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		taken[row.Name] = struct{}{}
	}
	baseName := discoveredLabel(entry)
	name := baseName
	for suffix := 2; ; suffix++ {
		if _, exists := taken[name]; !exists {
			break
		}
		name = fmt.Sprintf("%s-%d", baseName, suffix)
	}
	id, err := s.newID("wt")
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	item := Worktree{
		ID: id, WorkspaceID: workspaceID, Name: name, Branch: entry.Branch,
		Path: candidatePath, GitDir: adminGitDir, State: StateReady,
		Origin: OriginAdopted, SetupState: SetupNone, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.Insert(ctx, item); err != nil {
		if existing, getErr := s.store.GetByPath(
			ctx,
			workspaceID,
			candidatePath,
		); getErr == nil &&
			existing != nil {
			if filepath.Clean(existing.GitDir) != adminGitDir {
				return nil, refusal(ErrAdoptionUnreadable, "adopted worktree identity changed")
			}
			return existing, nil
		}
		return nil, mapInsertError(err, item)
	}
	s.invalidateDiscovery(workspaceID)
	s.emit(ctx, EventAdopted, item)
	return &item, nil
}

func (s *Service) reuseAdoptedWorktree(
	ctx context.Context,
	existing *Worktree,
	adminGitDir string,
) (*Worktree, error) {
	if filepath.Clean(existing.GitDir) != adminGitDir {
		return nil, refusal(ErrAdoptionUnreadable, "adopted worktree identity changed")
	}
	if existing.State != StateMissing {
		return existing, nil
	}
	now := s.now().UTC()
	swapped, err := s.store.CompareAndSwapState(
		ctx, existing.WorkspaceID, existing.ID, StateMissing, StateReady, now,
	)
	if err != nil {
		return nil, fmt.Errorf("worktree: restore adopted path: %w", err)
	}
	if !swapped {
		current, currentErr := s.store.Get(ctx, existing.WorkspaceID, existing.ID)
		if currentErr != nil {
			return nil, fmt.Errorf("worktree: reread adopted path: %w", currentErr)
		}
		return current, nil
	}
	existing.State = StateReady
	existing.UpdatedAt = now
	s.invalidateDiscovery(existing.WorkspaceID)
	s.publishCatalogEvent(CatalogEventUpserted, *existing)
	return existing, nil
}

func (s *Service) inspectAdoptionCandidate(
	ctx context.Context,
	workspace Workspace,
	candidate string,
	commonDir string,
) (adoptionIdentity, *GitWorktree, error) {
	identity, err := s.validateAdoptionIdentity(ctx, candidate, commonDir)
	if err != nil {
		return adoptionIdentity{}, nil, err
	}
	entries, err := s.readWorktreeList(ctx, workspace.Root, commonDir)
	if err != nil {
		return adoptionIdentity{}, nil, refusal(ErrAdoptionUnreadable, err.Error())
	}
	var matched *GitWorktree
	for index := range entries {
		if canonicalComparablePath(entries[index].Path) == candidate {
			matched = &entries[index]
			break
		}
	}
	if matched == nil || matched.Main || matched.Bare || matched.Prunable {
		return adoptionIdentity{}, nil, refusal(ErrAdoptionUnreadable, "candidate is not an attached linked worktree")
	}
	stdout, stderr, configErr := s.runner.Run(ctx, candidate, "config", "--get", "core.worktree")
	if configErr == nil && strings.TrimSpace(string(stdout)) != "" {
		return adoptionIdentity{}, nil, refusal(ErrAdoptionUnreadable, "core.worktree redirects the checkout")
	}
	if configErr != nil && strings.TrimSpace(string(stderr)) != "" {
		return adoptionIdentity{}, nil, refusal(ErrAdoptionUnreadable, "core.worktree configuration is unreadable")
	}
	return identity, matched, nil
}

type adoptionIdentity struct {
	adminGitDir string
}

func (s *Service) validateAdoptionIdentity(
	ctx context.Context,
	candidate string,
	commonDir string,
) (adoptionIdentity, error) {
	dotGitPath := filepath.Join(candidate, ".git")
	info, err := os.Lstat(dotGitPath)
	if err != nil {
		return adoptionIdentity{}, refusal(ErrAdoptionUnreadable, "candidate .git metadata is unreadable")
	}
	if info.IsDir() {
		return adoptionIdentity{}, ErrAdoptionForeignRepo
	}
	rawPointer, err := os.ReadFile(dotGitPath)
	if err != nil {
		return adoptionIdentity{}, refusal(ErrAdoptionUnreadable, "candidate .git pointer is unreadable")
	}
	pointer := strings.TrimSpace(string(rawPointer))
	if !strings.HasPrefix(pointer, "gitdir:") {
		return adoptionIdentity{}, refusal(ErrAdoptionUnreadable, "candidate .git pointer is malformed")
	}
	adminPath := strings.TrimSpace(strings.TrimPrefix(pointer, "gitdir:"))
	if !filepath.IsAbs(adminPath) {
		adminPath = filepath.Join(candidate, adminPath)
	}
	adminPath, err = filepath.Abs(filepath.Clean(adminPath))
	if err != nil {
		return adoptionIdentity{}, refusal(ErrAdoptionUnreadable, "linked-worktree admin directory is invalid")
	}
	adminGitDir, err := validateAdoptionAdminPath(adminPath, commonDir)
	if err != nil {
		return adoptionIdentity{}, err
	}
	commonPointer, err := os.ReadFile(filepath.Join(adminGitDir, "commondir"))
	linkedCommonPath := ""
	switch {
	case err == nil:
		linkedCommonPath = filepath.Join(adminGitDir, strings.TrimSpace(string(commonPointer)))
	case errors.Is(err, os.ErrNotExist):
		stdout, stderr, fallbackErr := s.runner.Run(
			ctx, candidate, "rev-parse", "--path-format=absolute", "--git-common-dir",
		)
		if fallbackErr != nil {
			return adoptionIdentity{}, refusal(
				ErrAdoptionUnreadable,
				"linked-worktree commondir is unreadable: "+strings.TrimSpace(string(stderr)),
			)
		}
		linkedCommonPath = strings.TrimSpace(string(stdout))
	default:
		return adoptionIdentity{}, refusal(ErrAdoptionUnreadable, "linked-worktree commondir is unreadable")
	}
	linkedCommon, err := fileutil.CanonicalExistingDirectory(linkedCommonPath)
	if err != nil || linkedCommon != commonDir {
		return adoptionIdentity{}, ErrAdoptionForeignRepo
	}
	backlink, err := os.ReadFile(filepath.Join(adminGitDir, "gitdir"))
	if err != nil {
		return adoptionIdentity{}, refusal(ErrAdoptionUnreadable, "linked-worktree backlink is unreadable")
	}
	backlinkPath := strings.TrimSpace(string(backlink))
	if !filepath.IsAbs(backlinkPath) {
		backlinkPath = filepath.Join(adminGitDir, backlinkPath)
	}
	backlinkAbs, err := filepath.Abs(filepath.Clean(backlinkPath))
	if err != nil {
		return adoptionIdentity{}, refusal(ErrAdoptionUnreadable, "linked-worktree backlink is invalid")
	}
	canonicalBacklink, err := filepath.EvalSymlinks(backlinkAbs)
	if err != nil || canonicalBacklink != filepath.Join(candidate, ".git") {
		return adoptionIdentity{}, refusal(
			ErrAdoptionUnreadable,
			"linked-worktree backlink does not match the candidate",
		)
	}
	return adoptionIdentity{adminGitDir: adminGitDir}, nil
}

func validateAdoptionAdminPath(adminPath string, commonDir string) (string, error) {
	adminRoot := filepath.Join(commonDir, "worktrees")
	lexicallyContained, lexicalErr := fileutil.PathWithinRoot(adminRoot, adminPath)
	adminGitDir, err := fileutil.CanonicalExistingDirectory(adminPath)
	if err != nil {
		return "", refusal(ErrAdoptionUnreadable, "linked-worktree admin directory is unreadable")
	}
	contained, err := fileutil.PathWithinRoot(adminRoot, adminGitDir)
	if err == nil && contained && adminGitDir != filepath.Clean(adminRoot) {
		return adminGitDir, nil
	}
	if lexicalErr == nil && lexicallyContained {
		return "", refusal(ErrAdoptionUnreadable, "linked-worktree admin directory escapes the repository")
	}
	modulesRoot := filepath.Join(commonDir, "modules")
	if inModules, modulesErr := fileutil.PathWithinRoot(modulesRoot, adminGitDir); modulesErr == nil && inModules {
		return "", refusal(ErrAdoptionUnreadable, "candidate is a submodule checkout")
	}
	return "", ErrAdoptionForeignRepo
}
