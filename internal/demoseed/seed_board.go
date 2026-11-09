package demoseed

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/worktree"
)

// boardLaterSweepLimit bounds the one Later return sweep the seed runs, like the daemon's.
const boardLaterSweepLimit = 20

// boardCounts summarizes the board and orchestration records a seed run writes.
type boardCounts struct {
	sessions, transcriptEvents, subagents, monitors, pullRequests, later int
}

// boardSessionIDs lists every board session directory a replace must clear.
func boardSessionIDs(clock timeline) []string {
	stories := append(boardSessions(clock), boardOrchestrationSessions(clock)...)
	ids := make([]string, 0, len(stories))
	for _, story := range stories {
		ids = append(ids, story.ID)
	}
	return ids
}

// seedBoard runs after the worktree fixture exists, so a session can bind to it.
func seedBoard(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (boardCounts, error) {
	clock := state.clock
	stories := append(boardSessions(clock), boardOrchestrationSessions(clock)...)
	counts := boardCounts{}
	for _, story := range stories {
		written, err := seedSession(ctx, db, state, story.sessionStory)
		if err != nil {
			return boardCounts{}, err
		}
		counts.sessions++
		counts.transcriptEvents += written
		if err := seedBoardSessionExtras(ctx, db, state, story, &counts); err != nil {
			return boardCounts{}, err
		}
	}
	// The daemon's Later sweep brings back what is due; the seed runs the same sweep once.
	if _, err := db.ReturnDueLaterSessions(ctx, clock.Now(), boardLaterSweepLimit); err != nil {
		return boardCounts{}, fmt.Errorf("demo seed: return due Later sessions: %w", err)
	}
	subagents, err := seedPixSubagents(ctx, db, state)
	if err != nil {
		return boardCounts{}, err
	}
	counts.subagents = subagents
	if err := seedPixMonitor(ctx, db, state); err != nil {
		return boardCounts{}, err
	}
	counts.monitors = 1
	return counts, nil
}

func seedBoardSessionExtras(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	story boardSessionStory,
	counts *boardCounts,
) error {
	record, err := state.recordFor(story.WorkspaceKey)
	if err != nil {
		return err
	}
	worktreeID, worktreeName := "", ""
	if story.BindWorktree {
		if err := bindBoardWorktree(ctx, db, state, record, story); err != nil {
			return err
		}
		worktreeID, worktreeName = seedWorktreeID, seedWorktreeName
	}
	if story.PullRequest != nil {
		if err := seedBoardPullRequest(ctx, db, record, story, worktreeID, worktreeName); err != nil {
			return err
		}
		counts.pullRequests++
	}
	if story.Later != nil {
		if err := parkBoardSession(ctx, db, state, record, story); err != nil {
			return err
		}
		counts.later++
	}
	return nil
}

func bindBoardWorktree(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	record workspaceRecord,
	story boardSessionStory,
) error {
	item, err := db.WorktreeStore().Get(ctx, record.ID, seedWorktreeID)
	if err != nil {
		return fmt.Errorf("demo seed: read worktree for session %q: %w", story.ID, err)
	}
	if err := db.UpdateSessionWorktreeBinding(ctx, store.SessionWorktreeBindingUpdate{
		SessionID: story.ID, WorkspaceID: record.ID, ProfileID: store.DefaultProfileID,
		WorktreeID: seedWorktreeID, UpdatedAt: story.EndedAt,
	}); err != nil {
		return fmt.Errorf("demo seed: bind session %q to worktree: %w", story.ID, err)
	}
	metaPath := store.SessionMetaFile(filepath.Join(state.paths.SessionsDir, story.ID))
	meta, err := store.ReadSessionMeta(metaPath)
	if err != nil {
		return fmt.Errorf("demo seed: read metadata for session %q: %w", story.ID, err)
	}
	meta.SetWorktreeID(seedWorktreeID)
	meta.SetCWD(item.Path)
	if err := store.WriteSessionMeta(metaPath, &meta); err != nil {
		return fmt.Errorf("demo seed: write worktree metadata for session %q: %w", story.ID, err)
	}
	return nil
}

func seedBoardPullRequest(
	ctx context.Context,
	db *globaldb.GlobalDB,
	record workspaceRecord,
	story boardSessionStory,
	worktreeID string,
	worktreeName string,
) error {
	pr := story.PullRequest
	if err := db.SaveSessionWorkLink(ctx, store.SessionWorkLink{
		SessionID: story.ID, WorkspaceID: record.ID, Repository: northstarRepoKey,
		Branch: pr.Branch, PRNumber: pr.Number, WorktreeID: worktreeID, WorktreeName: worktreeName,
		Provenance: "agent",
	}, false); err != nil {
		return fmt.Errorf("demo seed: link work for session %q: %w", story.ID, err)
	}
	number, prState, draft, merged := pr.Number, "open", false, false
	updatedAt, observedAt := pr.UpdatedAt, pr.ObservedAt
	failedNames := pr.FailedNames
	if failedNames == nil {
		failedNames = []string{}
	}
	if err := db.Worktrees.SavePullRequest(ctx, worktree.PullRequestEntry{
		Key: worktree.PullRequestKey{WorkspaceID: record.ID, RepoKey: northstarRepoKey, Branch: pr.Branch},
		Facts: &worktree.ForgeStatus{
			Title: pr.Title, Provider: "github", PRNumber: &number, PRState: &prState,
			PRURL: fmt.Sprintf("%s/pull/%d", northstarRepoURL, pr.Number),
			Head:  pr.Branch, Base: "main", Draft: &draft, Merged: &merged,
			HeadSHA: fmt.Sprintf("%07x", pr.Number*104729),
			Checks: &worktree.ForgeChecks{
				State: pr.ChecksState, Passed: pr.Passed, Failed: pr.Failed, Pending: pr.Pending,
				FailedNames: failedNames,
			},
			ReviewDecision: pr.ReviewDecision, Mergeable: pr.Mergeable,
			UpdatedAt: &updatedAt, FetchedAt: &observedAt,
		},
		LookupState: "ok", AttemptAt: &observedAt,
	}); err != nil {
		return fmt.Errorf("demo seed: save pull request #%d: %w", pr.Number, err)
	}
	return nil
}

func parkBoardSession(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	record workspaceRecord,
	story boardSessionStory,
) error {
	returnAt := story.Later.ReturnAt
	state.writeClock.at(story.Later.ParkedAt)
	defer state.writeClock.reset()
	if _, err := db.ParkSession(ctx, store.SessionLaterPark{
		WorkspaceID: record.ID, SessionID: story.ID, Reason: story.Later.Reason, ReturnAt: &returnAt,
	}); err != nil {
		return fmt.Errorf("demo seed: park session %q in Later: %w", story.ID, err)
	}
	return nil
}

func seedPixSubagents(ctx context.Context, db *globaldb.GlobalDB, state *scenario) (int, error) {
	record, err := state.recordFor(workspaceKeyPlatform)
	if err != nil {
		return 0, err
	}
	subs := pixSubagents(state.clock)
	for _, sub := range subs {
		if err := seedPixSubagent(ctx, db, state, record, sub); err != nil {
			return 0, err
		}
	}
	return len(subs), nil
}

func seedPixSubagent(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	record workspaceRecord,
	sub subagentStory,
) error {
	createdAt := sub.StartedAt.Add(-10 * time.Second)
	parentTurn := pixRefundsParentTurnLabel
	if _, _, err := db.ReserveSubagent(ctx, store.SessionSubagent{
		ID: sub.ID, WorkspaceID: record.ID, ParentSessionID: sessionPixRefundsID,
		ParentTurnID: parentTurn, ParentToolCallID: pixSubagentToolCallID(sub),
		Origin: store.SubagentOriginDelegated, IdempotencyKey: sub.ID, RequestFingerprint: "seed:" + sub.ID,
		Title: sub.Title, Role: sub.Role, TaskChars: sub.TaskChars,
		RuntimeAgent: sub.Agent, RuntimeProvider: sub.Provider, RuntimeModel: sub.Model,
		Depth: 1, Status: store.SubagentStatusQueued, WorkState: store.SubagentWorkStateWorking,
		WakePolicy: store.SubagentWakePolicySettledOnly, Delivery: store.SubagentDeliveryNone,
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}); err != nil {
		return fmt.Errorf("demo seed: reserve subagent %q: %w", sub.ID, err)
	}
	if _, err := db.LinkChild(ctx, sub.ID, sub.ChildID, sub.StartedAt); err != nil {
		return fmt.Errorf("demo seed: link subagent %q: %w", sub.ID, err)
	}
	result := sub.Result
	if _, _, err := db.FinalizeSubagent(ctx, store.SubagentFinalize{
		ID: sub.ID, Status: sub.Status, Result: &result, SettledAt: sub.SettledAt,
	}); err != nil {
		return fmt.Errorf("demo seed: finalize subagent %q: %w", sub.ID, err)
	}
	// The parent read every result before it opened the pull request, so nothing waits for delivery.
	state.writeClock.at(sub.SettledAt.Add(time.Minute))
	defer state.writeClock.reset()
	if _, _, err := db.Acknowledge(ctx, sub.ID, parentTurn); err != nil {
		return fmt.Errorf("demo seed: acknowledge subagent %q: %w", sub.ID, err)
	}
	return nil
}

// seedPixMonitor records the pull_request monitor the parent used to wait on #485's checks.
// It already fired, so a boot restores it as history instead of resuming its owner.
func seedPixMonitor(ctx context.Context, db *globaldb.GlobalDB, state *scenario) error {
	record, err := state.recordFor(workspaceKeyPlatform)
	if err != nil {
		return err
	}
	clock := state.clock
	created, fired := clock.minutesAgo(23), clock.minutesAgo(14)
	if err := db.SaveMonitor(ctx, monitor.Record{
		Monitor: monitor.Monitor{
			ID: pixRefundsMonitorID, SessionID: sessionPixRefundsID, Kind: monitor.KindPullRequest,
			Description: "Wake when the checks on #485 change", Branch: pixRefundsBranch,
			Status: monitor.StatusFired, CreatedAt: created, DeadlineAt: created.Add(6 * time.Hour),
			FiredAt: fired, Reason: monitor.ReasonPullRequestChanged, FireCount: 1, LastFiredAt: fired,
		},
		WorkspaceID: record.ID, ProfileID: store.DefaultProfileID, Timeout: 6 * time.Hour,
		RepoKey: northstarRepoKey, Version: clock.Now().UnixNano(),
	}); err != nil {
		return fmt.Errorf("demo seed: save pull request monitor: %w", err)
	}
	return nil
}
