//go:build integration

package demoseed

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
)

func TestSeedAlwaysOn(t *testing.T) {
	t.Parallel()

	t.Run("Should replay a coherent always-on team across a replace", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		now := time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)
		homeDir := filepath.Join(t.TempDir(), "home")
		if _, err := Seed(ctx, Options{HomeDir: homeDir, Now: now}); err != nil {
			t.Fatalf("Seed(first) error = %v", err)
		}
		result, err := Seed(ctx, Options{HomeDir: homeDir, Replace: true, Now: now})
		if err != nil {
			t.Fatalf("Seed(replace) error = %v", err)
		}
		if result.Counts.AlwaysOnInstances != 5 || result.Counts.InstanceReports == 0 ||
			result.Counts.ProposalPicks == 0 || result.Counts.CheckIns == 0 {
			t.Fatalf("Seed(replace).Counts = %#v, want five instances with check-ins, reports and picks", result.Counts)
		}
		db, err := globaldb.OpenGlobalDB(ctx, result.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.WithoutCancel(ctx)); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		instances, err := db.Instances().List(ctx, agentinstance.ListQuery{
			ProfileID: store.DefaultProfileID, AllWorkspaces: true, Limit: 50,
		})
		if err != nil {
			t.Fatalf("Instances().List() error = %v", err)
		}
		byAgent := map[string]agentinstance.Instance{}
		for _, instance := range instances {
			byAgent[instance.AgentName] = instance
		}
		if len(byAgent) != 5 {
			t.Fatalf("live instances = %d (%v), want 5 after replace", len(instances), byAgent)
		}
		assertInstanceStates(t, ctx, db, byAgent)
		assertTriagerProposals(t, ctx, db, byAgent[agentIssueTriager])
		assertReleaseWatchAway(t, ctx, db, byAgent[agentReleaseWatch], now)
		assertMainSessionReports(t, ctx, homeDir, byAgent[agentIssueTriager])
		assertAgentNotes(t, homeDir, byAgent[agentIssueTriager])
	})
}

func assertInstanceStates(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	byAgent map[string]agentinstance.Instance,
) {
	t.Helper()
	if got := byAgent[agentLogTriage]; got.State != "paused" || got.PauseReason != pauseReasonOperator {
		t.Fatalf("log-triage = %s/%s, want paused by the operator", got.State, got.PauseReason)
	}
	for _, name := range []string{agentIssueTriager, agentReleaseWatch, agentRiskMonitor, agentOrchestrator} {
		instance := byAgent[name]
		if instance.State != "active" || instance.MainSessionID == "" {
			t.Fatalf("%s = %s main %q, want active with a main session", name, instance.State, instance.MainSessionID)
		}
		needs, err := db.Instances().NeedsAttention(ctx, instance.ID)
		if err != nil {
			t.Fatalf("NeedsAttention(%s) error = %v", name, err)
		}
		if want := name == agentRiskMonitor; needs != want {
			t.Fatalf("NeedsAttention(%s) = %v, want %v", name, needs, want)
		}
	}
}

func assertTriagerProposals(t *testing.T, ctx context.Context, db *globaldb.GlobalDB, instance agentinstance.Instance) {
	t.Helper()
	reports, err := db.InstanceReports().
		ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 100})
	if err != nil {
		t.Fatalf("ListReports() error = %v", err)
	}
	checkIns, err := db.Instances().ListCheckIns(ctx, instance.ID, "", 100)
	if err != nil {
		t.Fatalf("ListCheckIns() error = %v", err)
	}
	var autoAccepted, waiting int64
	for _, checkIn := range checkIns {
		autoAccepted += checkIn.Proposals.AutoAccepted
		waiting += checkIn.Proposals.Waiting
	}
	if len(reports) != 3 || autoAccepted != 1 || waiting < 2 {
		t.Fatalf("issue-triager reports=%d auto-accepted=%d waiting=%d, want 3 reports, 1 auto-accepted, 2+ waiting",
			len(reports), autoAccepted, waiting)
	}
	for _, report := range reports {
		if report.EventSessionID != instance.MainSessionID || report.EventSeq == 0 {
			t.Fatalf(
				"report %s published to %q seq %d, want the main session",
				report.ID,
				report.EventSessionID,
				report.EventSeq,
			)
		}
	}
}

func assertReleaseWatchAway(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	instance agentinstance.Instance,
	now time.Time,
) {
	t.Helper()
	seen, err := db.Instances().LastSeen(ctx, instance.ID, agentinstance.LocalOperator)
	if err != nil || seen == nil {
		t.Fatalf("LastSeen() = %v, %v, want the operator's last visit", seen, err)
	}
	quiet, err := db.Instances().CountQuietCheckIns(ctx, instance.ID, seen)
	if err != nil {
		t.Fatalf("CountQuietCheckIns() error = %v", err)
	}
	reports, err := db.InstanceReports().ListReports(ctx, agentinstance.ReportQuery{
		InstanceID: instance.ID, AfterCreatedAt: *seen, Limit: 100,
	})
	if err != nil {
		t.Fatalf("ListReports(after seen) error = %v", err)
	}
	// The web digest needs at least six check-ins since the last visit to sum them up.
	if len(reports) < 2 || int(quiet)+len(reports) < 6 || !seen.Before(now) {
		t.Fatalf("release-watch away: reports=%d quiet=%d seen=%s, want a While you were away digest",
			len(reports), quiet, seen)
	}
}

func assertMainSessionReports(t *testing.T, ctx context.Context, homeDir string, instance agentinstance.Instance) {
	t.Helper()
	dir := filepath.Join(homeDir, "sessions", instance.MainSessionID)
	events, err := sessiondb.OpenSessionDBReadOnly(ctx, store.SessionDBOwner{
		SessionID: instance.MainSessionID, WorkspaceID: instance.WorkspaceID,
	}, store.SessionDBFile(dir))
	if err != nil {
		t.Fatalf("OpenSessionDB(main) error = %v", err)
	}
	t.Cleanup(func() {
		if err := events.Close(context.WithoutCancel(ctx)); err != nil {
			t.Errorf("Close(main) error = %v", err)
		}
	})
	rows, err := events.Query(ctx, store.EventQuery{Limit: 500})
	if err != nil {
		t.Fatalf("ListEvents(main) error = %v", err)
	}
	reports, users := 0, 0
	for _, row := range rows {
		switch {
		case row.Type == transcript.ReportEventType:
			reports++
		case strings.Contains(row.Content, "Draft a reproduction note") || strings.Contains(row.Content, "#431"):
			users++
		}
	}
	if reports != 3 || users == 0 {
		t.Fatalf("main session reports=%d pick turns=%d, want 3 published reports and the turns picks started",
			reports, users)
	}
}

func assertAgentNotes(t *testing.T, homeDir string, instance agentinstance.Instance) {
	t.Helper()
	notes, err := agentmemory.NewStore(homeDir).Read(agentmemory.Key{
		ProfileID: store.DefaultProfileID, Origin: "workspace-" + instance.WorkspaceID, Agent: agentIssueTriager,
	})
	if err != nil {
		t.Fatalf("Read(notes) error = %v", err)
	}
	if strings.Count(notes.Content, "\n- ") != 2 || !strings.Contains(notes.Content, "coverage") {
		t.Fatalf("issue-triager notes = %q, want its three notes once after a replace", notes.Content)
	}
}
