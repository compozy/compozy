//go:build integration

package demoseed

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
)

func TestSeedBoard(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should fill the durable board columns, Later, and orchestration records across a replace",
		func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			now := time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC)
			homeDir := filepath.Join(t.TempDir(), "home")
			first, err := Seed(ctx, Options{HomeDir: homeDir, Now: now})
			if err != nil {
				t.Fatalf("Seed(first) error = %v", err)
			}
			replaced, err := Seed(ctx, Options{HomeDir: homeDir, Replace: true, Now: now.Add(time.Hour)})
			if err != nil {
				t.Fatalf("Seed(replace) error = %v", err)
			}
			if replaced.Counts.Subagents != first.Counts.Subagents ||
				replaced.Counts.PullRequests != first.Counts.PullRequests ||
				replaced.Counts.LaterSessions != first.Counts.LaterSessions ||
				replaced.Counts.Monitors != 1 {
				t.Fatalf(
					"Seed(replace).Counts = %#v, want the first run's board counts %#v",
					replaced.Counts,
					first.Counts,
				)
			}

			db, err := globaldb.OpenGlobalDB(ctx, replaced.DatabaseFile)
			if err != nil {
				t.Fatalf("OpenGlobalDB() error = %v", err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.WithoutCancel(ctx)); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			})
			platformID := replaced.WorkspaceIDs[1]
			assertPixTranscript(t, ctx, homeDir, platformID)
			assertSessionFolders(t, ctx, db, replaced)
			keys := map[string]string{platformID: northstarRepoKey}

			for column, want := range map[string][]string{
				store.SessionBoardNeedsYou: {sessionChargebackBackfillID, sessionDisputePlaybookID},
				store.SessionBoardInReview: {sessionRetryBudgetID, sessionWebhookSecretsID, sessionPixRefundsID},
				store.SessionBoardDone:     {sessionLedgerFlakeID, sessionPagerSummaryID},
			} {
				ids := boardColumnIDs(t, ctx, db, replaced.WorkspaceIDs, keys, column, store.SessionLaterExclude)
				for _, id := range want {
					if !slices.Contains(ids, id) {
						t.Fatalf("board column %q = %v, want it to contain %q", column, ids, id)
					}
				}
			}
			later := boardColumnIDs(t, ctx, db, replaced.WorkspaceIDs, keys, "", store.SessionLaterOnly)
			if !slices.Contains(later, sessionPSPUpgradeID) || slices.Contains(later, sessionDisputePlaybookID) {
				t.Fatalf("Later sessions = %v, want the parked PSP upgrade and not the returned playbook", later)
			}

			subagents, err := db.ListSubagents(ctx, store.SubagentListQuery{
				WorkspaceID: platformID, ParentSessionID: sessionPixRefundsID, Limit: 10,
			})
			if err != nil {
				t.Fatalf("ListSubagents() error = %v", err)
			}
			if len(subagents.Items) != 3 {
				t.Fatalf("ListSubagents() = %d rows, want 3", len(subagents.Items))
			}
			for _, row := range subagents.Items {
				if row.Status != store.SubagentStatusCompleted || row.ChildSessionID == nil || row.Result == nil {
					t.Fatalf("subagent %q = %#v, want a completed card linked to its child with a result", row.ID, row)
				}
			}

			monitors, err := db.LoadMonitors(ctx)
			if err != nil {
				t.Fatalf("LoadMonitors() error = %v", err)
			}
			found := slices.IndexFunc(monitors, func(record monitor.Record) bool {
				return record.Monitor.ID == pixRefundsMonitorID
			})
			if found < 0 || monitors[found].WorkspaceID != platformID ||
				monitors[found].Monitor.Kind != monitor.KindPullRequest {
				t.Fatalf(
					"LoadMonitors() = %#v, want the PIX refunds pull_request monitor in the replaced workspace",
					monitors,
				)
			}
		},
	)
}

func boardColumnIDs(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceIDs []string,
	keys map[string]string,
	column string,
	later store.SessionLaterFilter,
) []string {
	t.Helper()
	query := store.SessionCatalogPageQuery{
		RepositoryKeys: keys, HideChildren: true, ExcludeVerdictOnly: true,
		ReadScope:    store.ReadScope{ProfileID: store.DefaultProfileID},
		WorkspaceIDs: workspaceIDs, Archive: store.SessionArchiveExclude, Later: later, Sort: "attention", Limit: 100,
	}
	if column != "" {
		query.Columns = []string{column}
	}
	page, err := db.PageSessions(ctx, query)
	if err != nil {
		t.Fatalf("PageSessions(%q) error = %v", column, err)
	}
	ids := make([]string, 0, len(page.Sessions))
	for _, info := range page.Sessions {
		ids = append(ids, info.ID)
	}
	return ids
}

// assertPixTranscript reads the parent transcript the way the session view projects it.
func assertPixTranscript(t *testing.T, ctx context.Context, homeDir string, workspaceID string) {
	t.Helper()
	reader, err := sessiondb.OpenSessionDBReadOnly(
		ctx,
		store.SessionDBOwner{SessionID: sessionPixRefundsID, WorkspaceID: workspaceID},
		store.SessionDBFile(filepath.Join(homeDir, "sessions", sessionPixRefundsID)),
	)
	if err != nil {
		t.Fatalf("OpenSessionDBReadOnly() error = %v", err)
	}
	t.Cleanup(func() {
		if err := reader.Close(context.WithoutCancel(ctx)); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	events, err := reader.Query(ctx, store.EventQuery{Limit: 200})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	cards := []string{}
	for _, event := range events {
		if part, ok := transcript.SubagentPartForStoredEvent(event); ok {
			cards = append(cards, part.SubagentID)
		}
	}
	slices.Sort(cards)
	want := []string{pixSubagentReconcileID, pixSubagentRunbookID, pixSubagentStatesID}
	if !slices.Equal(cards, want) {
		t.Fatalf("subagent cards = %v, want %v", cards, want)
	}
	messages, err := transcript.ToUIMessages(events)
	if err != nil {
		t.Fatalf("ToUIMessages() error = %v", err)
	}
	projected, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("Marshal(messages) error = %v", err)
	}
	if !strings.Contains(string(projected), "payments-platform/pull/485") {
		t.Fatal("projected transcript lost the Bash stdout of gh pr create")
	}
}

// assertSessionFolders reads back the folders the session list groups by after a replace.
func assertSessionFolders(t *testing.T, ctx context.Context, db *globaldb.GlobalDB, result Result) {
	t.Helper()
	listed := 0
	for _, workspaceID := range result.WorkspaceIDs {
		folders, err := db.ListSessionFolders(ctx, store.SessionLayoutScope{
			ProfileID: store.DefaultProfileID, WorkspaceID: workspaceID,
		})
		if err != nil {
			t.Fatalf("ListSessionFolders(%s) error = %v", workspaceID, err)
		}
		for _, folder := range folders {
			if folder.SessionCount == 0 {
				t.Fatalf("session folder %q is empty", folder.Name)
			}
			listed++
		}
	}
	if listed != result.Counts.SessionFolders || listed == 0 {
		t.Fatalf("session folders = %d, Counts.SessionFolders = %d", listed, result.Counts.SessionFolders)
	}
	layout, err := db.WorkspaceLayout(ctx, store.DefaultProfileID)
	if err != nil {
		t.Fatalf("WorkspaceLayout() error = %v", err)
	}
	filed := 0
	for _, placement := range layout.Placements {
		if placement.FolderID == northstarProjectFolderID {
			filed++
		}
	}
	if len(layout.Folders) != 1 || filed != len(result.WorkspaceIDs) {
		t.Fatalf("project layout folders=%d filed=%d, want one folder holding every project", len(layout.Folders), filed)
	}
}
