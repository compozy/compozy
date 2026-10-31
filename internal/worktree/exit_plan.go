package worktree

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

const (
	reasonNoChanges        = "No uncommitted changes to commit."
	reasonCommitFirst      = "Commit changes before pushing branches."
	reasonNoPendingPush    = "No commits pending push."
	reasonUpstreamGone     = "Upstream is unavailable. Refresh forge status to check whether the PR was merged."
	reasonPushFirst        = "Push commits before opening pull requests."
	reasonDiverged         = "Branch has diverged from upstream. Rebase/merge first."
	reasonBehind           = "Branch is behind upstream. Pull before creating a PR."
	reasonDetached         = "Detached HEAD: create a branch before continuing."
	reasonMissingRemote    = "Add an \"origin\" remote before continuing."
	reasonNoPRChanges      = "No branch changes to include in a PR."
	reasonSessionRunning   = "Git actions are paused while a bound session is running."
	reasonStatusUnreadable = "Git status could not be read for this worktree."
	reasonRemoteUnreadable = "Git remote configuration could not be read for this worktree."
	exitUntrackedFileLimit = 200
	exitOpenPRLabel        = "Open PR"
	defaultBaseBranch      = "main"
)

func (s *Service) ExitPlan(ctx context.Context, workspaceID, id string) (*ExitPlan, error) {
	return s.exitPlan(ctx, workspaceID, id, false)
}

// PrepareExitPlan resolves provider capabilities only for an explicit delivery gesture.
func (s *Service) PrepareExitPlan(ctx context.Context, workspaceID, id string) (*ExitPlan, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	plan, err := s.exitPlan(ctx, workspaceID, id, true)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *Service) exitPlan(ctx context.Context, workspaceID, id string, action bool) (*ExitPlan, error) {
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || item == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("worktree: read exit target: %w", err)
	}
	if item.State != StateReady {
		return nil, ErrNotReady
	}
	status, err := s.readExitStatus(ctx, *item)
	if err != nil {
		return nil, err
	}
	if status.ReadError != "" {
		return s.unreadableExitPlan(ctx, *item, status)
	}
	scope, err := s.readExitCommitScope(ctx, *item, status)
	if err != nil {
		return nil, err
	}
	remoteURLs, err := s.readOriginRemoteURLs(ctx, item.Path)
	if err != nil {
		return s.remoteUnreadableExitPlan(ctx, *item, status, scope)
	}
	forge := s.cachedExitForgeCapabilities(remoteURLs)
	if action {
		forge = s.exitForgeCapabilities(ctx, remoteURLs)
	}
	forgeStatus := s.readDisplayPullRequest(ctx, *item)
	base := s.resolveExitBase(ctx, *item, status, forge)
	baseAhead := s.countCommitsAhead(ctx, item.Path, base)
	paused, err := s.exitPauseCause(ctx, *item, status)
	if err != nil {
		return nil, err
	}
	plan := &ExitPlan{
		WorktreeID: item.ID, CommitScope: scope, Forge: forge, ForgeStatus: forgeStatus,
		Base: base, RemoteURLs: remoteURLs,
	}
	if forge != nil && (forgeStatus == nil || forgeStatus.PRURL == "") {
		plan.PRPrefill = &ExitPRPrefill{
			Title: item.Branch,
			Body:  s.detectPRTemplate(ctx, item.Path, base, forge.TemplatePaths),
		}
	}
	if paused != "" {
		plan.GlobalPauseCause = paused
	}
	plan.BrowserURL = browserCompareURL(remoteURLs, base, item.Branch, forge)
	plan.Actions = buildExitActions(
		status, forge, forgeStatus, baseAhead, len(remoteURLs) > 0, plan.BrowserURL, paused,
	)
	plan.Primary = selectExitPrimary(plan.Actions, status)
	plan.Cleanup = s.exitCleanupEvidence(ctx, *item, nil)
	return plan, nil
}

// readExitStatus refreshes the status the plan is built from without emitting
// worktree.status_refreshed. The Web ladder rereads the plan on that frame, so
// a plan read that announced its own refresh would refetch itself without end.
func (s *Service) readExitStatus(ctx context.Context, item Worktree) (*Status, error) {
	if _, err := s.capability.Check(ctx); err != nil {
		return nil, err
	}
	return s.persistStatus(ctx, item)
}

func (s *Service) remoteUnreadableExitPlan(
	ctx context.Context,
	item Worktree,
	status *Status,
	scope ExitCommitScope,
) (*ExitPlan, error) {
	forgeStatus := s.readDisplayPullRequest(ctx, item)
	plan := &ExitPlan{
		WorktreeID:       item.ID,
		GlobalPauseCause: reasonRemoteUnreadable,
		ForgeStatus:      forgeStatus,
		Base:             strings.TrimPrefix(strings.TrimSpace(item.BaseRef), "origin/"),
		CommitScope:      scope,
		Cleanup: ExitCleanupEvidence{
			Blocker: reasonRemoteUnreadable,
		},
	}
	plan.Actions = buildExitActions(status, nil, nil, 0, false, "", reasonRemoteUnreadable)
	plan.Primary = selectExitPrimary(plan.Actions, status)
	return plan, nil
}

func (s *Service) unreadableExitPlan(
	ctx context.Context,
	item Worktree,
	status *Status,
) (*ExitPlan, error) {
	forgeStatus := s.readDisplayPullRequest(ctx, item)
	plan := &ExitPlan{
		WorktreeID:       item.ID,
		GlobalPauseCause: reasonStatusUnreadable,
		ForgeStatus:      forgeStatus,
		Base:             strings.TrimPrefix(strings.TrimSpace(item.BaseRef), "origin/"),
		Cleanup: ExitCleanupEvidence{
			Blocker: reasonStatusUnreadable,
		},
	}
	plan.Actions = buildExitActions(status, nil, forgeStatus, 0, false, "", reasonStatusUnreadable)
	plan.Primary = selectExitPrimary(plan.Actions, status)
	return plan, nil
}

func (s *Service) exitPauseCause(ctx context.Context, item Worktree, status *Status) (string, error) {
	if status.ReadError != "" {
		return reasonStatusUnreadable, nil
	}
	if s.sessions == nil {
		return "", nil
	}
	active, err := s.sessions.HasActiveSession(ctx, item.WorkspaceID, item.ID)
	if err != nil {
		return "", fmt.Errorf("worktree: inspect exit session activity: %w", err)
	}
	if active {
		return reasonSessionRunning, nil
	}
	return "", nil
}

func (s *Service) exitForgeCapabilities(ctx context.Context, remotes []string) *ForgeCapabilities {
	if s.forge == nil || len(remotes) == 0 {
		return nil
	}
	capabilities, err := s.forge.Capabilities(ctx, remotes)
	if err != nil || capabilities == nil || strings.TrimSpace(capabilities.Provider) == "" {
		return nil
	}
	s.forgeCapabilitiesMu.Lock()
	if s.forgeCapabilities == nil {
		s.forgeCapabilities = make(map[string]*ForgeCapabilities)
	}
	s.forgeCapabilities[strings.Join(remotes, "\n")] = capabilities
	s.forgeCapabilitiesMu.Unlock()
	return capabilities
}

func (s *Service) cachedExitForgeCapabilities(remotes []string) *ForgeCapabilities {
	s.forgeCapabilitiesMu.Lock()
	defer s.forgeCapabilitiesMu.Unlock()
	return s.forgeCapabilities[strings.Join(remotes, "\n")]
}

func buildExitActions(
	status *Status,
	forge *ForgeCapabilities,
	forgeStatus *ForgeStatus,
	baseAhead int,
	hasRemote bool,
	browserURL string,
	pause string,
) []ExitActionPlan {
	dirty := valueOrZero(status.DirtyFiles) > 0
	detached := status.Detached != nil && *status.Detached
	ahead := valueOrZero(status.Ahead)
	behind := valueOrZero(status.Behind)
	hasUpstream := status.HasUpstream != nil && *status.HasUpstream
	pendingPush := ahead > 0 || (!hasUpstream && hasRemote && !detached)
	rows := []ExitActionPlan{{Action: ExitActionCommit, Label: "Commit", Enabled: dirty}}
	if hasRemote {
		rows = append(
			rows,
			ExitActionPlan{Action: ExitActionCommitPush, Label: "Commit & push", Enabled: dirty && !detached},
			ExitActionPlan{
				Action:  ExitActionPush,
				Label:   "Push",
				Enabled: !dirty && pendingPush && !detached,
				Publish: !hasUpstream,
			},
		)
	}
	switch {
	case hasRemote && forgeStatus != nil && forgeStatus.PRURL != "":
		viewLabel := "View in browser"
		if forge != nil && strings.TrimSpace(forge.ViewActionLabel) != "" {
			viewLabel = strings.TrimSpace(forge.ViewActionLabel)
		}
		rows = append(rows, ExitActionPlan{
			Action: ExitActionViewPR, Label: viewLabel, Enabled: true,
			URL: forgeStatus.PRURL, PRNumber: valueOrZero(forgeStatus.PRNumber),
		})
	case hasRemote && forge != nil:
		openLabel := strings.TrimSpace(forge.OpenActionLabel)
		if openLabel == "" {
			openLabel = exitOpenPRLabel
		}
		rows = append(rows, ExitActionPlan{Action: ExitActionOpenPR, Label: openLabel, Enabled: true})
	case hasRemote && browserURL != "":
		rows = append(rows, ExitActionPlan{
			Action: ExitActionOpenPR, Label: exitOpenBrowserLabel, Enabled: true, URL: browserURL,
		})
	}
	applyExitActionBlocks(rows, exitActionBlockState{
		dirty: dirty, detached: detached, hasRemote: hasRemote, hasUpstream: hasUpstream,
		pendingPush: pendingPush, ahead: ahead, behind: behind, baseAhead: baseAhead, pause: pause,
	})
	blockGoneUpstreamActions(rows, status, pause)
	return rows
}

func blockGoneUpstreamActions(rows []ExitActionPlan, status *Status, pause string) {
	if pause != "" || status.HasUpstream == nil || !*status.HasUpstream || status.Ahead != nil {
		return
	}
	for index := range rows {
		action := &rows[index]
		switch action.Action {
		case ExitActionCommitPush, ExitActionPush, ExitActionOpenPR:
			action.Enabled, action.BlockedReason = false, reasonUpstreamGone
		}
	}
}

type exitActionBlockState struct {
	dirty, detached, hasRemote, hasUpstream, pendingPush bool
	ahead, behind, baseAhead                             int
	pause                                                string
}

func applyExitActionBlocks(rows []ExitActionPlan, state exitActionBlockState) {
	for index := range rows {
		row := &rows[index]
		if state.pause != "" {
			row.Enabled, row.BlockedReason = false, state.pause
			continue
		}
		switch row.Action {
		case ExitActionCommit:
			if !state.dirty {
				row.BlockedReason = reasonNoChanges
			}
		case ExitActionCommitPush:
			row.BlockedReason = commitPushBlockReason(
				state.dirty, state.detached, state.hasRemote, state.ahead, state.behind,
			)
			row.Enabled = row.BlockedReason == ""
		case ExitActionPush:
			row.BlockedReason = pushBlockReason(
				state.dirty, state.detached, state.hasRemote, state.pendingPush, state.ahead, state.behind,
			)
			row.Enabled = row.BlockedReason == ""
		case ExitActionOpenPR:
			row.BlockedReason = openPRBlockReason(
				state.dirty, state.detached, state.hasRemote, state.hasUpstream,
				state.ahead, state.behind, state.baseAhead,
			)
			row.Enabled = row.BlockedReason == ""
		case ExitActionViewPR:
			// A cached PR remains viewable regardless of local branch divergence.
		}
	}
}

func commitPushBlockReason(dirty, detached, hasRemote bool, ahead, behind int) string {
	if !dirty {
		return reasonNoChanges
	}
	if detached {
		return reasonDetached
	}
	if !hasRemote {
		return reasonMissingRemote
	}
	if ahead > 0 && behind > 0 {
		return reasonDiverged
	}
	if behind > 0 {
		return reasonBehind
	}
	return ""
}

func pushBlockReason(dirty, detached, hasRemote, pending bool, ahead, behind int) string {
	if dirty {
		return reasonCommitFirst
	}
	if detached {
		return reasonDetached
	}
	if !hasRemote {
		return reasonMissingRemote
	}
	if ahead > 0 && behind > 0 {
		return reasonDiverged
	}
	if behind > 0 {
		return reasonBehind
	}
	if !pending {
		return reasonNoPendingPush
	}
	return ""
}

func openPRBlockReason(dirty, detached, hasRemote, hasUpstream bool, ahead, behind, baseAhead int) string {
	if dirty {
		return reasonCommitFirst
	}
	if detached {
		return reasonDetached
	}
	if !hasRemote {
		return reasonMissingRemote
	}
	if ahead > 0 && behind > 0 {
		return reasonDiverged
	}
	if behind > 0 {
		return reasonBehind
	}
	if !hasUpstream || ahead > 0 {
		return reasonPushFirst
	}
	if baseAhead == 0 {
		return reasonNoPRChanges
	}
	return ""
}

func selectExitPrimary(rows []ExitActionPlan, status *Status) ExitAction {
	dirty := valueOrZero(status.DirtyFiles) > 0
	preferred := []ExitAction{ExitActionCommit, ExitActionPush, ExitActionOpenPR, ExitActionViewPR}
	if !dirty {
		preferred = preferred[1:]
	}
	for _, action := range preferred {
		for _, row := range rows {
			if row.Action == action && row.Enabled {
				return action
			}
		}
	}
	if dirty {
		return ExitActionCommit
	}
	return ""
}

func (s *Service) readOriginRemoteURLs(ctx context.Context, path string) ([]string, error) {
	stdout, stderr, err := s.runner.Run(ctx, path, "remote", "get-url", "--all", "origin")
	if err != nil {
		var exitError interface{ ExitCode() int }
		if errors.As(err, &exitError) && exitError.ExitCode() == 2 { //nolint:forbidigo // ExitCode is not error.
			return nil, nil
		}
		safe := diagnostics.RedactAndBound(exitCommandOutput(stdout, stderr, err), 2048)
		return nil, fmt.Errorf("worktree: read origin remote URLs: %w", errors.New(safe))
	}
	values := strings.Fields(string(stdout))
	remotes := make([]string, 0, len(values))
	for _, value := range values {
		if sanitized := sanitizeGitRemote(value); sanitized != "" {
			remotes = append(remotes, sanitized)
		}
	}
	return remotes, nil
}

func (s *Service) resolveExitBase(ctx context.Context, item Worktree, status *Status, forge *ForgeCapabilities) string {
	stdout, _, err := s.runner.Run(ctx, item.Path, "config", "--get", "branch."+item.Branch+".gh-merge-base")
	if err == nil && strings.TrimSpace(string(stdout)) != "" {
		return strings.TrimSpace(string(stdout))
	}
	if status.HasUpstream != nil && *status.HasUpstream {
		stdout, _, err = s.runner.Run(ctx, item.Path, "rev-parse", "--abbrev-ref", "@{upstream}")
		if err == nil {
			upstream := strings.TrimSpace(string(stdout))
			if _, branch, found := strings.Cut(upstream, "/"); found && branch != item.Branch {
				return branch
			}
		}
	}
	if forge != nil && strings.TrimSpace(forge.DefaultBranch) != "" {
		return strings.TrimSpace(forge.DefaultBranch)
	}
	if strings.TrimSpace(item.BaseRef) != "" {
		return strings.TrimPrefix(strings.TrimSpace(item.BaseRef), "origin/")
	}
	return defaultBaseBranch
}

func (s *Service) countCommitsAhead(ctx context.Context, path, base string) int {
	if strings.TrimSpace(base) == "" {
		return 0
	}
	stdout, _, err := s.runner.Run(ctx, path, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
	if err != nil || count < 0 {
		return 0
	}
	return count
}

func (s *Service) readExitCommitScope(ctx context.Context, item Worktree, status *Status) (ExitCommitScope, error) {
	stdout, stderr, err := s.runner.Run(ctx, item.Path, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	if err != nil {
		return ExitCommitScope{}, refusal(
			ErrStatusUnreadable,
			diagnostics.RedactAndBound(strings.TrimSpace(string(stderr)), 2048),
		)
	}
	untracked, err := ParseUntrackedStatusV2(stdout)
	if err != nil {
		return ExitCommitScope{}, refusal(ErrStatusUnreadable, err.Error())
	}
	total := len(untracked)
	if len(untracked) > exitUntrackedFileLimit {
		untracked = untracked[:exitUntrackedFileLimit]
	}
	return ExitCommitScope{
		ChangedFiles: valueOrZero(status.DirtyFiles), Insertions: valueOrZero(status.Insertions),
		Deletions: valueOrZero(status.Deletions), UntrackedFiles: untracked,
		UntrackedTotal: total, UntrackedTruncated: total > len(untracked), Complete: total <= len(untracked),
	}, nil
}
