//go:build integration

package demoseed

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	automation "github.com/compozy/compozy/internal/automation/model"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

// catalogSessions and catalogWorkspaces satisfy the manager's required collaborators;
// listing definitions and runs never reaches them.
type catalogSessions struct{ automationpkg.SessionManager }

type catalogWorkspaces struct {
	compozyworkspace.RuntimeResolver
}

func TestSeedAutomationCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should only record finished runs with one first attempt per slot at clock boundaries", func(t *testing.T) {
		t.Parallel()

		// 2026-07-29 is a Wednesday: exact hour, just after it, and the briefing hour itself.
		for _, now := range []time.Time{
			time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC),
			time.Date(2026, 7, 29, 8, 1, 0, 0, time.UTC),
			time.Date(2026, 7, 29, 8, 10, 0, 0, time.UTC),
		} {
			clock := newTimeline(now)
			firsts := map[string]map[time.Time]bool{}
			for _, run := range scenarioAutomationRuns(clock) {
				if run.EndedAt.After(clock.Now()) || run.StartedAt.Before(run.ScheduledAt) {
					t.Fatalf("now %s: run %s ends %s, starts %s for slot %s, want finished work",
						now, run.ID, run.EndedAt, run.StartedAt, run.ScheduledAt)
				}
				if run.Attempt != 1 || run.ScheduledAt.IsZero() {
					continue
				}
				key := run.JobID + run.TriggerID
				if firsts[key] == nil {
					firsts[key] = map[time.Time]bool{}
				}
				if firsts[key][run.ScheduledAt] {
					t.Fatalf("now %s: %s has two first attempts for %s", now, key, run.ScheduledAt)
				}
				firsts[key][run.ScheduledAt] = true
			}
		}
	})

	t.Run("Should list seeded schedules and triggers with recorded run history", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		now := time.Date(2026, 7, 29, 14, 30, 0, 0, time.UTC)
		homeDir := filepath.Join(t.TempDir(), "home")
		if _, err := Seed(ctx, Options{HomeDir: homeDir, Now: now}); err != nil {
			t.Fatalf("Seed(first) error = %v", err)
		}
		result, err := Seed(ctx, Options{HomeDir: homeDir, Replace: true, Now: now})
		if err != nil {
			t.Fatalf("Seed(replace) error = %v", err)
		}
		db, err := globaldb.OpenGlobalDB(ctx, result.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() { _ = db.Close(context.WithoutCancel(ctx)) })
		manager := openCatalogManager(t, ctx, db)
		scope := store.ReadScope{ProfileID: store.DefaultProfileID}

		jobs, err := manager.ListJobs(ctx, automationpkg.JobListQuery{ReadScope: scope})
		if err != nil {
			t.Fatalf("ListJobs() error = %v", err)
		}
		triggers, err := manager.ListTriggers(ctx, automationpkg.TriggerListQuery{ReadScope: scope})
		if err != nil {
			t.Fatalf("ListTriggers() error = %v", err)
		}
		jobIDs := make([]string, 0, len(jobs.Jobs))
		for _, job := range jobs.Jobs {
			jobIDs = append(jobIDs, job.ID)
		}
		triggerIDs := make([]string, 0, len(triggers.Triggers))
		for _, trigger := range triggers.Triggers {
			triggerIDs = append(triggerIDs, trigger.ID)
		}
		for _, id := range []string{briefingAutomationID, launchAutomationID, digestAutomationID} {
			if !slices.Contains(jobIDs, id) {
				t.Fatalf("ListJobs() ids = %v, want %q", jobIDs, id)
			}
		}
		for _, id := range []string{releaseNotesTriggerID, supportMacroTriggerID} {
			if !slices.Contains(triggerIDs, id) {
				t.Fatalf("ListTriggers() ids = %v, want %q", triggerIDs, id)
			}
		}
		if result.Counts.AutomationJobs != len(jobIDs) || result.Counts.AutomationTriggers != len(triggerIDs) {
			t.Fatalf("Counts = %#v, want %d jobs and %d triggers", result.Counts, len(jobIDs), len(triggerIDs))
		}

		runs, err := manager.ListRuns(ctx, automation.RunQuery{ReadScope: scope, AllWorkspaces: true, Limit: 100})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if len(runs) != result.Counts.AutomationRuns {
			t.Fatalf("ListRuns() = %d runs, want %d", len(runs), result.Counts.AutomationRuns)
		}
		retried, failed := false, 0
		for _, run := range runs {
			if run.Attribution != "recorded" || run.WorkspaceID == nil || run.SessionID == "" {
				t.Fatalf("run %q = %#v, want recorded attribution, workspace and session", run.ID, run)
			}
			if run.Status == automation.RunCompleted && run.Attempt == 2 {
				retried = true
			}
			if run.Status == automation.RunFailed {
				failed++
			}
		}
		if !retried || failed < 2 {
			t.Fatalf("runs = %#v, want a retried success and failures", runs)
		}
		latest, err := manager.LatestRunsByOwner(ctx, automation.RunOwnerJob, []string{briefingAutomationID})
		if err != nil || latest[briefingAutomationID].Status != automation.RunCompleted {
			t.Fatalf("LatestRunsByOwner() = %#v, %v, want the completed briefing", latest, err)
		}
		assertAutomationRunSessions(t, ctx, db, runs)
		assertParkedLoopRun(t, ctx, db, result)
	})
}

func openCatalogManager(t *testing.T, ctx context.Context, db *globaldb.GlobalDB) *automationpkg.Manager {
	t.Helper()
	stores, err := openAutomationStores(db, time.Now)
	if err != nil {
		t.Fatalf("openAutomationStores() error = %v", err)
	}
	manager, err := automationpkg.New(
		automationpkg.WithStore(db),
		automationpkg.WithSessions(catalogSessions{}),
		automationpkg.WithWorkspaceResolver(catalogWorkspaces{}),
		automationpkg.WithResourceDefinitions(stores.jobs, stores.triggers, automationResourceActor(), nil),
	)
	if err != nil {
		t.Fatalf("automation.New() error = %v", err)
	}
	actor := automationResourceActor()
	jobRecords, err := stores.jobs.List(ctx, actor, resources.ResourceFilter{Kind: automationpkg.JobResourceKind})
	if err != nil {
		t.Fatalf("List(job resources) error = %v", err)
	}
	jobPlan, err := manager.BuildJobResourceState(ctx, jobRecords)
	if err != nil {
		t.Fatalf("BuildJobResourceState() error = %v", err)
	}
	if err := manager.ApplyJobResourceState(ctx, jobPlan); err != nil {
		t.Fatalf("ApplyJobResourceState() error = %v", err)
	}
	triggerRecords, err := stores.triggers.List(
		ctx, actor, resources.ResourceFilter{Kind: automationpkg.TriggerResourceKind},
	)
	if err != nil {
		t.Fatalf("List(trigger resources) error = %v", err)
	}
	triggerPlan, err := manager.BuildTriggerResourceState(ctx, triggerRecords)
	if err != nil {
		t.Fatalf("BuildTriggerResourceState() error = %v", err)
	}
	if err := manager.ApplyTriggerResourceState(ctx, triggerPlan); err != nil {
		t.Fatalf("ApplyTriggerResourceState() error = %v", err)
	}
	return manager
}

// assertAutomationRunSessions keeps finished runs out of the operator's list and the
// newest exhausted failure in Needs you.
func assertAutomationRunSessions(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	runs []automation.Run,
) {
	t.Helper()
	unseenFailures := 0
	for _, run := range runs {
		found, err := db.ListSessions(ctx, store.SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, ID: run.SessionID, Limit: 1,
		})
		if err != nil || len(found) != 1 {
			t.Fatalf("ListSessions(%q) = %d, %v, want the run session", run.SessionID, len(found), err)
		}
		info := found[0]
		if info.InitiatorKind != "automation" || info.SessionType != string(session.SessionTypeSystem) {
			t.Fatalf("session %q = %#v, want an automation-initiated system session", run.SessionID, info)
		}
		attention, err := db.GetSessionAttention(ctx, run.SessionID)
		if err != nil {
			t.Fatalf("GetSessionAttention(%q) error = %v", run.SessionID, err)
		}
		if run.Status == automation.RunFailed && attention.FailureSeenAt == nil {
			unseenFailures++
		}
	}
	if unseenFailures != 1 {
		t.Fatalf("unseen failed automation sessions = %d, want 1", unseenFailures)
	}
}

func assertParkedLoopRun(t *testing.T, ctx context.Context, db *globaldb.GlobalDB, result Result) {
	t.Helper()
	statuses := map[looppkg.RunID]looppkg.Status{}
	for _, workspaceID := range result.WorkspaceIDs {
		runs, err := db.ListLoopRuns(ctx, looppkg.RunListQuery{
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: looppkg.WorkspaceID(workspaceID), Limit: 100,
		})
		if err != nil {
			t.Fatalf("ListLoopRuns(%q) error = %v", workspaceID, err)
		}
		for _, run := range runs {
			statuses[run.ID] = run.Status
		}
	}
	if statuses[loopApprovalRunID] != looppkg.StatusNeedsApproval {
		t.Fatalf("Loop run statuses = %#v, want %q parked in needs-approval", statuses, loopApprovalRunID)
	}
	ended := 0
	for _, status := range statuses {
		if status.Terminal() {
			ended++
		}
	}
	if ended == 0 {
		t.Fatalf("Loop run statuses = %#v, want finished runs with a named ending", statuses)
	}
}
