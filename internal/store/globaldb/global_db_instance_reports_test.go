package globaldb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestInstanceSkillReviewLedger(t *testing.T) {
	t.Run("Should propose an inert draft and link its report [UT-040]", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances
 (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at)
 VALUES ('proposal-instance',?,'coder','active','human','operator','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z')`, store.DefaultProfileID)
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		scanRoot := filepath.Join(root, "skills")
		service := agentinstance.SkillProposalService{
			Store:     db.SkillReviews(),
			DraftRoot: filepath.Join(root, "drafts"),
			ScanRoots: []string{scanRoot},
		}
		instance := agentinstance.Instance{
			ID:        "proposal-instance",
			ProfileID: store.DefaultProfileID,
			AgentName: "coder",
		}
		input := agentinstance.SkillProposalInput{
			Name:        "review-changes",
			Description: "Review changes",
			Body:        "Read the diff and report findings.",
		}
		review, err := service.Propose(ctx, workspaceaccess.ActorRef{Operator: true}, instance, scanRoot, input)
		if err != nil {
			t.Fatal(err)
		}
		if review.State != "pending" {
			t.Fatalf("state=%s", review.State)
		}
		if _, err := os.Stat(scanRoot); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("proposal created a scan root: %v", err)
		}
		if _, err := agentinstance.ReadSkillDraft(review.DraftPath, review.Digest); err != nil {
			t.Fatal(err)
		}
		var payload string
		if err := db.DB().
			QueryRowContext(ctx, "SELECT payload FROM agent_instance_reports WHERE source_key = ?", "skill_review:"+review.ID).
			Scan(&payload); err != nil {
			t.Fatal(err)
		}
		var report struct {
			SkillReviewID string `json:"skill_review_id"`
		}
		if err := json.Unmarshal([]byte(payload), &report); err != nil || report.SkillReviewID != review.ID {
			t.Fatalf("report link: %+v %v", report, err)
		}
		retry, err := service.Propose(ctx, workspaceaccess.ActorRef{Operator: true}, instance, scanRoot, input)
		if err != nil || retry.ID != review.ID {
			t.Fatalf("retry: %+v %v", retry, err)
		}
		entries, err := os.ReadDir(service.DraftRoot)
		if err != nil || len(entries) != 1 {
			t.Fatalf("draft retention: %d %v", len(entries), err)
		}
		if err := service.Reject(
			ctx,
			workspaceaccess.ActorRef{},
			instance,
			review.ID,
			review.Digest,
			"operator",
		); !errors.Is(
			err,
			agentinstance.ErrSkillReviewOperatorOnly,
		) {
			t.Fatalf("agent rejection: %v", err)
		}
		operator := workspaceaccess.ActorRef{Operator: true}
		foreign := instance
		foreign.WorkspaceID = "foreign-workspace"
		if _, _, err := service.Review(
			ctx,
			operator,
			foreign,
			review.ID,
		); !errors.Is(
			err,
			agentinstance.ErrSkillReviewNotFound,
		) {
			t.Fatalf("foreign review: %v", err)
		}
		if err := service.Reject(
			ctx,
			operator,
			instance,
			review.ID,
			"stale-digest",
			"operator",
		); !errors.Is(
			err,
			agentinstance.ErrSkillReviewStale,
		) {
			t.Fatalf("stale rejection: %v", err)
		}
		if err := service.Reject(ctx, operator, instance, review.ID, review.Digest, "operator"); err != nil {
			t.Fatal(err)
		}
		rejected, err := service.Propose(ctx, operator, instance, scanRoot, input)
		if err != nil || rejected.State != "rejected" {
			t.Fatalf("rejected reproposal: %+v %v", rejected, err)
		}
		rows, err := service.Reviews(ctx, operator, instance, agentinstance.SkillReviewQuery{Limit: 10})
		if err != nil || len(rows) != 1 || rows[0].State != "rejected" {
			t.Fatalf("reviews: %+v %v", rows, err)
		}
		input.Body = "Read the diff and verify the changed behavior."
		changed, err := service.Propose(ctx, operator, instance, scanRoot, input)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.Approve(
			ctx,
			workspaceaccess.ActorRef{},
			instance,
			changed.ID,
			changed.Digest,
			"operator",
			scanRoot,
		); !errors.Is(
			err,
			agentinstance.ErrSkillReviewOperatorOnly,
		) {
			t.Fatalf("agent approval: %v", err)
		}
		if err := service.Approve(
			ctx,
			operator,
			instance,
			changed.ID,
			"old-digest",
			"operator",
			scanRoot,
		); !errors.Is(
			err,
			agentinstance.ErrSkillReviewStale,
		) {
			t.Fatalf("stale approval: %v", err)
		}
		if err := service.Approve(
			ctx,
			operator,
			instance,
			changed.ID,
			changed.Digest,
			"operator",
			scanRoot,
		); err != nil {
			t.Fatal(err)
		}
		draft, err := agentinstance.ReadSkillDraft(changed.DraftPath, changed.Digest)
		if err != nil {
			t.Fatal(err)
		}
		installed, err := os.ReadFile(filepath.Join(scanRoot, input.Name, "SKILL.md"))
		if err != nil || string(installed) != draft.Files[0].Content {
			t.Fatalf("installed contents: %v", err)
		}
		if err := service.Approve(
			ctx,
			operator,
			instance,
			changed.ID,
			changed.Digest,
			"operator",
			scanRoot,
		); err != nil {
			t.Fatalf("approval retry: %v", err)
		}
		approved, err := db.SkillReviews().GetReview(ctx, changed.ID)
		if err != nil || approved.State != "approved" {
			t.Fatalf("approval ledger: %+v %v", approved, err)
		}
	})
	t.Run("Should never leave an approved review without its installed skill", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances
 (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at)
 VALUES ('rollback-instance',?,'coder','active','human','operator','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z')`, store.DefaultProfileID)
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		scanRoot := filepath.Join(root, "skills")
		decideErr := errors.New("ledger write failed")
		failing := agentinstance.SkillProposalService{
			Store:     failingDecideStore{SkillReviewStore: db.SkillReviews(), err: decideErr},
			DraftRoot: filepath.Join(root, "drafts"),
			ScanRoots: []string{scanRoot},
		}
		instance := agentinstance.Instance{
			ID: "rollback-instance", ProfileID: store.DefaultProfileID, AgentName: "coder",
		}
		operator := workspaceaccess.ActorRef{Operator: true}
		review, err := failing.Propose(ctx, operator, instance, scanRoot, agentinstance.SkillProposalInput{
			Name: "rollback-skill", Description: "Rollback", Body: "Check the rollback.",
		})
		if err != nil {
			t.Fatal(err)
		}
		err = failing.Approve(ctx, operator, instance, review.ID, review.Digest, "operator", scanRoot)
		if !errors.Is(err, decideErr) {
			t.Fatalf("Approve() error = %v, want the ledger failure", err)
		}
		if _, statErr := os.Stat(filepath.Join(scanRoot, "rollback-skill")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("skill left installed after the decision failed: %v", statErr)
		}
		pending, err := db.SkillReviews().GetReview(ctx, review.ID)
		if err != nil || pending.State != "pending" {
			t.Fatalf("review after failed decision = %+v, %v; want pending", pending, err)
		}
	})

	t.Run("Should atomically create reviews and reports and reject stale decisions", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		var repo agentinstance.SkillReviewStore = db.SkillReviews()
		ctx := t.Context()
		input := agentinstance.SkillReview{
			ID:         "review",
			InstanceID: "review-instance",
			ProfileID:  store.DefaultProfileID,
			SkillName:  "skill",
			Digest:     "digest",
			DraftPath:  "/drafts/skill",
		}
		report := agentinstance.ReportInput{
			SourceKey: "turn:skill-review",
			Kind:      "proposal",
			Payload: []byte(
				`{"skill_review_id":"review","proposals":[{"title":"Review skill","body":"Inspect proposed contents"}]}`,
			),
		}
		if _, err := repo.CreateReview(ctx, input, report); err == nil {
			t.Fatal("missing report instance must fail")
		}
		if _, err := repo.GetReview(ctx, input.ID); !errors.Is(err, agentinstance.ErrSkillReviewNotFound) {
			t.Fatalf("rollback: %v", err)
		}
		_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances
 (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at)
 VALUES ('review-instance',?,'coder','active','human','operator','2026-11-03T00:00:00Z','2026-11-03T00:00:00Z')`, store.DefaultProfileID)
		if err != nil {
			t.Fatal(err)
		}
		created, err := repo.CreateReview(ctx, input, report)
		if err != nil || created.State != "pending" {
			t.Fatalf("create: %+v %v", created, err)
		}
		input.ID = "retry"
		retried, err := repo.CreateReview(ctx, input, report)
		if err != nil || retried.ID != created.ID {
			t.Fatalf("retry: %+v %v", retried, err)
		}
		if err := repo.DecideReview(
			ctx,
			created.ID,
			"old-digest",
			"approved",
			"operator",
		); !errors.Is(
			err,
			agentinstance.ErrSkillReviewStale,
		) {
			t.Fatalf("stale: %v", err)
		}
		if err := repo.DecideReview(ctx, created.ID, created.Digest, "rejected", "operator"); err != nil {
			t.Fatal(err)
		}
		rows, err := repo.ListReviews(
			ctx,
			agentinstance.SkillReviewQuery{ProfileID: store.DefaultProfileID, State: "rejected", Limit: 10},
		)
		if err != nil || len(rows) != 1 || rows[0].DecidedAt == nil {
			t.Fatalf("list: %+v %v", rows, err)
		}
		var count int
		if err := db.DB().
			QueryRowContext(ctx, "SELECT count(*) FROM agent_instance_reports WHERE instance_id = ?", input.InstanceID).
			Scan(&count); err != nil ||
			count != 1 {
			t.Fatalf("reports: %d %v", count, err)
		}
	})
	t.Run("Should cap pending reviews and retain rejected content [UT-041 UT-044]", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		q := sqlcgen.New(db.DB())
		ctx := t.Context()
		input := sqlcgen.InsertAgentSkillReviewParams{
			InstanceID: "review-instance", ProfileID: store.DefaultProfileID,
			SkillName: "review-skill", DraftPath: "/drafts/skill", CreatedAt: "2026-11-03T00:00:00Z",
		}
		for i := range 6 {
			input.ID = fmt.Sprintf("review-%d", i)
			input.Digest = fmt.Sprintf("digest-%d", i)
			count, err := q.InsertAgentSkillReview(ctx, input)
			want := int64(1)
			if i == 5 {
				want = 0
			}
			if err != nil || count != want {
				t.Fatalf("insert %d: count=%d err=%v", i, count, err)
			}
		}
		count, err := q.DecideAgentSkillReview(ctx, sqlcgen.DecideAgentSkillReviewParams{
			ID: "review-0", Digest: "digest-0", State: "rejected", DecidedBy: "operator",
		})
		if err != nil || count != 1 {
			t.Fatalf("reject: count=%d err=%v", count, err)
		}
		input.ID, input.Digest = "retry", "digest-0"
		count, err = q.InsertAgentSkillReview(ctx, input)
		if err != nil || count != 0 {
			t.Fatalf("rejected retry: count=%d err=%v", count, err)
		}
		input.ID, input.Digest = "changed", "changed-digest"
		count, err = q.InsertAgentSkillReview(ctx, input)
		if err != nil || count != 1 {
			t.Fatalf("changed content: count=%d err=%v", count, err)
		}
		row, err := q.GetAgentSkillReview(ctx, "review-0")
		if err != nil || row.State != "rejected" {
			t.Fatalf("retained rejection: state=%s err=%v", row.State, err)
		}
	})
}

func TestInstanceReports(t *testing.T) {
	t.Run("Should preserve report recovery acknowledgement across migration and reopen", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00171_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
		_, seedErr := prefix.ExecContext(ctx, `INSERT INTO agent_instances
   (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at)
   VALUES ('recovery-instance',?,'coder','active','human','operator','2026-11-01T00:00:00Z','2026-11-01T00:00:00Z')`, store.DefaultProfileID)
		if seedErr == nil {
			_, seedErr = prefix.ExecContext(ctx, `INSERT INTO agent_instance_reports
    (id,instance_id,source_key,kind,payload,state,created_at)
    VALUES ('recovery-report','recovery-instance','occ:recovery','failure','{"reason":"uncertain"}','indeterminate','2026-11-01T00:00:00Z')`)
		}
		if err := errors.Join(seedErr, prefix.Close()); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		var payload string
		var noted int
		readErr := upgraded.DB().
			QueryRowContext(ctx, "SELECT payload,recovery_noted FROM agent_instance_reports WHERE id='recovery-report'").
			Scan(&payload, &noted)
		if readErr != nil || payload != `{"reason":"uncertain"}` || noted != 0 {
			t.Errorf("migrated payload=%q noted=%d error=%v", payload, noted, readErr)
		}
		ackErr := upgraded.MarkIndeterminateReportNoted(ctx, "recovery-report")
		if err := errors.Join(ackErr, upgraded.Close(ctx)); err != nil {
			t.Fatal(err)
		}
		reopened, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := reopened.DB().
			QueryRowContext(ctx, "SELECT recovery_noted FROM agent_instance_reports WHERE id='recovery-report'").
			Scan(&noted); err != nil ||
			noted != 1 {
			t.Fatalf("reopened acknowledgement=%d error=%v", noted, err)
		}
	})

	for _, retention := range []time.Duration{time.Hour, 72 * time.Hour} {
		t.Run(
			fmt.Sprintf("Should retain active reports and honor terminal cutoff for %s", retention),
			func(t *testing.T) {
				t.Parallel()
				db, workspace, main, now := subagentFixture(t)
				instance, err := db.Instances().
					Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
				if err != nil {
					t.Fatal(err)
				}
				cutoff := now.Add(-max(retention, 48*time.Hour))
				for _, row := range []struct {
					id, state string
					at        time.Time
				}{
					{"pending", "pending", cutoff.Add(-time.Hour)}, {"reserved", "reserved", cutoff.Add(-time.Hour)},
					{"delivered", "delivered", cutoff.Add(-time.Second)}, {"indeterminate", "indeterminate", cutoff.Add(-time.Second)},
					{"boundary", "delivered", cutoff}, {"recent", "delivered", now},
				} {
					_, err := db.DB().
						ExecContext(t.Context(), `INSERT INTO agent_instance_reports(id,instance_id,source_key,kind,payload,state,created_at,delivered_at) VALUES (?,?,?,'failure','{}',?,?,?)`, row.id, instance.ID, "occ:"+row.id, row.state, store.FormatTimestamp(cutoff.Add(-24*time.Hour)), store.FormatTimestamp(row.at))
					if err != nil {
						t.Fatal(err)
					}
				}
				for range 2 {
					count, err := db.InstanceReports().SweepTerminalReports(t.Context(), now, retention, 1)
					if err != nil || count != 1 {
						t.Fatalf("sweep = %d, %v", count, err)
					}
				}
				count, err := db.InstanceReports().SweepTerminalReports(t.Context(), now, retention, 100)
				if err != nil || count != 0 {
					t.Fatalf("repeat sweep = %d, %v", count, err)
				}
				rows, err := db.InstanceReports().
					ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 100})
				if err != nil || len(rows) != 4 {
					t.Fatalf("retained = %#v, %v", rows, err)
				}
				for _, row := range rows {
					if row.ID == "delivered" || row.ID == "indeterminate" {
						t.Fatalf("expired report retained: %s", row.ID)
					}
				}
			},
		)
	}

	t.Run("Should read the newest bounded reports without changing chronological pages", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		start := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		for _, id := range []string{"recent", "foreign"} {
			_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instances
                (id,profile_id,agent_name,state,created_by_kind,created_by_id,created_at,updated_at)
                VALUES (?,?,?,'active','human','operator',?,?)`, id, store.DefaultProfileID, id,
				store.FormatTimestamp(start), store.FormatTimestamp(start))
			if err != nil {
				t.Fatal(err)
			}
			for i := 29; i >= 0; i-- {
				at := start.Add(time.Duration(i/2) * time.Minute)
				if id == "foreign" {
					at = at.Add(24 * time.Hour)
				}
				state := "pending"
				if i%2 == 0 {
					state = "delivered"
				}
				reportID := fmt.Sprintf("%s-%02d", id, i)
				_, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instance_reports
                    (id,instance_id,source_key,kind,payload,state,created_at)
                    VALUES (?,?,?,'failure','{"reason":"failed","detail":"Details"}',?,?)`,
					reportID, id, "occ:"+reportID, state, store.FormatTimestamp(at))
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		repo := db.InstanceReports()
		recent, err := repo.RecentReports(ctx, "recent", 20)
		if err != nil || len(recent) != 20 {
			t.Fatalf("recent=%+v error=%v", recent, err)
		}
		for i, report := range recent {
			if report.ID != fmt.Sprintf("recent-%02d", 29-i) || report.InstanceID != "recent" {
				t.Fatalf("recent row %d=%+v", i, report)
			}
		}
		page, err := repo.ListReports(ctx, agentinstance.ReportQuery{InstanceID: "recent", Limit: 20})
		if err != nil || len(page) != 20 {
			t.Fatalf("page=%+v error=%v", page, err)
		}
		for i, report := range page {
			if report.ID != fmt.Sprintf("recent-%02d", i) {
				t.Fatalf("chronological row %d=%+v", i, report)
			}
		}
		for _, limit := range []int{0, 101} {
			if _, err := repo.RecentReports(ctx, "recent", limit); err == nil {
				t.Fatalf("accepted limit=%d", limit)
			}
		}
		if _, err := repo.RecentReports(ctx, "", 20); err == nil {
			t.Fatal("accepted empty instance")
		}
	})

	t.Run("Should persist deduplicated reports and fence publication across main session changes", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		db, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "reports", "sess-reports")
		_, successor := registerHeartbeatWorkspaceAndSession(t, db, "reports-successor", "sess-reports-successor")
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		db.now = func() time.Time { return now }
		_, err = db.DB().
			ExecContext(ctx, `INSERT INTO agent_instances (id,profile_id,agent_name,workspace_id,state,main_session_id,created_by_kind,created_by_id,created_at,updated_at) VALUES (?,?,?,?,'active',?,'human','',?,?)`, "ainst-reports", store.DefaultProfileID, "coder", workspace, main, store.FormatTimestamp(now), store.FormatTimestamp(now))
		if err != nil {
			t.Fatal(err)
		}
		repo := db.InstanceReports()
		input := agentinstance.ReportInput{
			SourceKey: "occ:reports-1",
			Kind:      "failure",
			Payload:   []byte(`{"reason":"failed","detail":"provider exited"}`),
		}
		first, err := repo.AppendReport(ctx, "ainst-reports", input)
		if err != nil {
			t.Fatal(err)
		}
		retry, err := repo.AppendReport(ctx, "ainst-reports", input)
		if err != nil {
			t.Fatal(err)
		}
		if retry.ID != first.ID {
			t.Fatalf("retry ID %s != %s", retry.ID, first.ID)
		}
		input.SourceKey = "occ:reports-2"
		if _, err := repo.AppendReport(ctx, "ainst-reports", input); err != nil {
			t.Fatal(err)
		}
		page, err := repo.ListReports(ctx, agentinstance.ReportQuery{InstanceID: "ainst-reports", Limit: 1})
		if err != nil || len(page) != 1 {
			t.Fatalf("page: %v %v", page, err)
		}
		next, err := repo.ListReports(
			ctx,
			agentinstance.ReportQuery{
				InstanceID:     "ainst-reports",
				Limit:          1,
				AfterCreatedAt: page[0].CreatedAt,
				AfterID:        page[0].ID,
			},
		)
		if err != nil || len(next) != 1 || next[0].ID == page[0].ID {
			t.Fatalf("next: %v %v", next, err)
		}
		if err := repo.MarkReportPublished(ctx, first.ID, main, 1); err != nil {
			t.Fatal(err)
		}
		pending, err := repo.UnpublishedReports(ctx, "", 10)
		if err != nil || len(pending) != 1 {
			t.Fatalf("pending: %v %v", pending, err)
		}
		if _, err := db.DB().
			ExecContext(ctx, `UPDATE agent_instances SET main_session_id=? WHERE id=?`, successor, "ainst-reports"); err != nil {
			t.Fatal(err)
		}
		if err := repo.MarkReportPublished(ctx, first.ID, main, 99); err != nil {
			t.Fatal(err)
		}
		pending, err = repo.UnpublishedReports(ctx, "", 10)
		if err != nil || len(pending) != 2 {
			t.Fatalf("rotated: %v %v", pending, err)
		}
		for _, r := range pending {
			if r.MainSessionID != successor {
				t.Fatalf("publication target: %s", r.MainSessionID)
			}
			if r.Report.ID == first.ID && r.Report.EventSeq != 1 {
				t.Fatal("stale publication overwrote event sequence")
			}
		}
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		reports, err := reopened.InstanceReports().
			ListReports(ctx, agentinstance.ReportQuery{InstanceID: "ainst-reports", Limit: 10})
		if err != nil || len(reports) != 2 {
			t.Fatalf("reopened: %v %v", reports, err)
		}
	})
}

func TestPendingInteractionReportsAtomicity(t *testing.T) {
	if path := os.Getenv("COMPOZY_B4_CRASH_DB"); path != "" {
		db, err := OpenGlobalDB(t.Context(), path)
		if err != nil {
			t.Fatal(err)
		}
		if os.Getenv("COMPOZY_B4_CRASH_PHASE") == "after" {
			var transition store.PendingInteractionTransition
			if err := json.Unmarshal([]byte(os.Getenv("COMPOZY_B4_CRASH_TRANSITION")), &transition); err != nil {
				t.Fatal(err)
			}
			if _, err := db.TransitionPendingInteraction(t.Context(), transition); err != nil {
				t.Fatal(err)
			}
		}
		// Simulate process loss: no database Close, checkpoint, or deferred cleanup.
		os.Exit(23)
	}
	t.Parallel()
	for _, tc := range []struct{ name, status, kind, key, expiredBy, cause string }{
		{"Should atomically report expiry once", store.PendingInteractionStatusTimedOut, "approval_expired", "interaction:pending-report", "timeout", ""},
		{"Should preserve provider expiry across crashes", store.PendingInteractionStatusTimedOut, "approval_expired", "interaction:pending-report", "provider", ""},
		{"Should atomically report interruption once", store.PendingInteractionStatusCanceled, "interrupted", "turn:turn-report", "", "daemon_restart"},
		{"Should report provider interruption on resume", store.PendingInteractionStatusOrphaned, "interrupted", "turn:turn-report", "", "provider_interrupted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), GlobalDatabaseName)
			db, err := OpenGlobalDB(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "pending-report", "main-report")
			instance, err := db.Instances().
				Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
			registerSubagentSession(t, db, workspace, "child-report", main, store.SubagentSpawnRole, now)
			toolID := ""
			if tc.expiredBy == "timeout" {
				toolID = "compozy__terminal_exec"
			}
			_, err = db.CreatePendingInteraction(
				t.Context(),
				store.PendingInteractionCreate{
					InteractionID: "pending-report",
					SessionID:     "child-report",
					Title:         "Read workspace files",
					Payload: store.PendingInteractionPayload{
						ToolID:   toolID,
						Action:   "session/request_permission",
						Resource: "make build",
						ToolKind: "execute",
					},
					Kind:              store.PendingInteractionKindPermission,
					ProviderRequestID: "request-report",
					TurnID:            "turn-report",
					CreatedAt:         now,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.DB().
				ExecContext(t.Context(), `CREATE TRIGGER reject_report BEFORE INSERT ON agent_instance_reports BEGIN SELECT RAISE(ABORT, 'injected report failure'); END`)
			if err != nil {
				t.Fatal(err)
			}
			transition := store.PendingInteractionTransition{
				InteractionID: "pending-report",
				Status:        tc.status,
				At:            now.Add(time.Hour),
				Resolution:    "reject_once",
				ResolvedBy:    "timeout",
			}
			if tc.expiredBy == "provider" {
				transition.Resolution, transition.ResolvedBy = "expired-by-provider", "provider"
			}
			if tc.status == store.PendingInteractionStatusOrphaned {
				transition.Resolution, transition.ResolvedBy = "", ""
			}
			if tc.status == store.PendingInteractionStatusCanceled {
				transition.Resolution, transition.ResolvedBy = "failed-by-restart", "system"
			}
			if _, err := db.TransitionPendingInteraction(t.Context(), transition); err == nil {
				t.Fatal("expected report insert failure")
			}
			rows, err := db.ListPendingInteractions(
				t.Context(),
				"child-report",
				[]string{store.PendingInteractionStatusPending},
			)
			if err != nil || len(rows) != 1 {
				t.Fatalf("rollback rows = %#v, %v", rows, err)
			}
			if _, err := db.DB().ExecContext(t.Context(), `DROP TRIGGER reject_report`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().
				ExecContext(t.Context(), `CREATE TRIGGER reject_transition BEFORE UPDATE ON session_pending_interactions BEGIN SELECT RAISE(ABORT, 'injected transition failure'); END`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.TransitionPendingInteraction(t.Context(), transition); err == nil {
				t.Fatal("expected transition failure")
			}
			rolledBack, err := db.InstanceReports().
				ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 10})
			if err != nil || len(rolledBack) != 0 {
				t.Fatalf("report escaped rollback: %#v, %v", rolledBack, err)
			}
			if _, err := db.DB().ExecContext(t.Context(), `DROP TRIGGER reject_transition`); err != nil {
				t.Fatal(err)
			}

			for _, phase := range []string{"before", "after", "after"} {
				if err := db.Close(t.Context()); err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(transition)
				if err != nil {
					t.Fatal(err)
				}
				child := exec.CommandContext(
					t.Context(),
					os.Args[0],
					"-test.run=^TestPendingInteractionReportsAtomicity$",
				)
				child.Env = append(
					os.Environ(),
					"COMPOZY_B4_CRASH_DB="+path,
					"COMPOZY_B4_CRASH_PHASE="+phase,
					"COMPOZY_B4_CRASH_TRANSITION="+string(encoded),
				)
				output, childErr := child.CombinedOutput()
				exitErr, ok := errors.AsType[*exec.ExitError](childErr)
				if !ok || exitErr.ExitCode() != 23 {
					t.Fatalf("crash child: %v: %s", childErr, output)
				}
				db, err = OpenGlobalDB(t.Context(), path)
				if err != nil {
					t.Fatal(err)
				}
				if phase == "before" {
					pending, err := db.ListPendingInteractions(
						t.Context(),
						"child-report",
						[]string{store.PendingInteractionStatusPending},
					)
					if err != nil || len(pending) != 1 {
						t.Fatalf("pre-commit crash pending = %#v, %v", pending, err)
					}
					continue
				}
				if _, err := db.TransitionPendingInteraction(t.Context(), transition); err != nil {
					t.Fatal(err)
				}
			}
			if tc.status != store.PendingInteractionStatusOrphaned {
				late, err := db.TransitionPendingInteraction(t.Context(), store.PendingInteractionTransition{
					InteractionID: "pending-report", Status: store.PendingInteractionStatusResolved,
					At: now.Add(2 * time.Hour), Resolution: "allow_once", ResolvedBy: "human",
				})
				if err != nil || late.Outcome != store.PendingInteractionOutcomeAlreadyResolved {
					t.Fatalf("late answer = %#v, %v", late, err)
				}
			}

			reports, err := db.InstanceReports().
				ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 10})
			if err != nil || len(reports) != 1 || reports[0].Kind != tc.kind || reports[0].SourceKey != tc.key {
				t.Fatalf("reports = %#v, %v", reports, err)
			}
			if tc.expiredBy != "" {
				var payload struct {
					Action    string `json:"action"`
					Resource  string `json:"resource"`
					ExpiredBy string `json:"expired_by"`
					Tool      string `json:"tool"`
				}
				if err := json.Unmarshal(reports[0].Payload, &payload); err != nil {
					t.Fatal(err)
				}
				wantTool := "Read workspace files"
				if toolID != "" {
					wantTool = toolID
				}
				if payload.Tool != wantTool {
					t.Fatalf("tool = %q", payload.Tool)
				}
				// The tool kind names the action; the ACP operation never reaches the report.
				if payload.Action != "execute" || payload.Resource != "make build" {
					t.Fatalf("expiry details: %+v", payload)
				}
				if payload.ExpiredBy != tc.expiredBy {
					t.Fatalf("expired_by = %q, want %q", payload.ExpiredBy, tc.expiredBy)
				}
			}
			if tc.cause != "" {
				var payload struct {
					Cause string `json:"cause"`
				}
				if err := json.Unmarshal(reports[0].Payload, &payload); err != nil {
					t.Fatal(err)
				}
				if payload.Cause != tc.cause {
					t.Fatalf("cause = %q, want %q", payload.Cause, tc.cause)
				}
			}
			rows, err = db.ListPendingInteractions(t.Context(), "child-report", []string{tc.status})
			if err != nil || len(rows) != 1 {
				t.Fatalf("transition rows = %#v, %v", rows, err)
			}
		})
	}
}

func TestInstanceReportPromptReservation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		human     bool
		operation string
		want      int
		exit      string
	}{
		{"Should reserve at most twenty published reports for a human prompt", true, store.SessionPromptOperationPrompt, 20, ""},
		{"Should not reserve reports for agent prompts", false, store.SessionPromptOperationPrompt, 0, ""},
		{"Should not reserve reports for steering", true, store.SessionPromptOperationSteer, 0, ""},
		{"Should release direct reservations when the prompt becomes a live steer", true, store.SessionPromptOperationPrompt, 20, "live-steer"},

		{"Should release reports when canceled", true, store.SessionPromptOperationPrompt, 20, "cancel"},
		{"Should release reports when cleared", true, store.SessionPromptOperationPrompt, 20, "clear"},
		{"Should release reports when replaced", true, store.SessionPromptOperationPrompt, 20, "replace"},
		{"Should release reports when promoted to steer", true, store.SessionPromptOperationPrompt, 20, "steer"},
		{"Should release reports when a task invalidates queued input", true, store.SessionPromptOperationPrompt, 20, "task-cancel"},
		{"Should release reports when stopped", true, store.SessionPromptOperationPrompt, 20, "stop"},
		{"Should deliver reports once at provider dispatch", true, store.SessionPromptOperationPrompt, 20, "dispatch"},
		{"Should fence uncertain report delivery", true, store.SessionPromptOperationPrompt, 20, "indeterminate"},
		{"Should release reports after start failure", true, store.SessionPromptOperationPrompt, 20, "start-failure"},
		{"Should release reservations on restart", true, store.SessionPromptOperationPrompt, 20, "restart"},
		{"Should fence dispatch interrupted by restart", true, store.SessionPromptOperationPrompt, 20, "restart-dispatched"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			ctx := t.Context()
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "reserve", "sess-reserve")
			now := time.Now().UTC()
			instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, WorkspaceID: workspace, AgentName: "coder",
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, 10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().
				ExecContext(ctx, "UPDATE agent_instances SET state='active', main_session_id=? WHERE id=?", main, instance.ID); err != nil {
				t.Fatal(err)
			}
			for i := range 23 {
				report, err := db.InstanceReports().AppendReport(ctx, instance.ID, agentinstance.ReportInput{
					SourceKey: fmt.Sprintf(
						"occ:reserve-%d",
						i,
					),
					Kind:    "failure",
					Payload: []byte(`{"reason":"failed","detail":"test"}`),
				})
				if err != nil {
					t.Fatal(err)
				}
				if i < 22 {
					if err := db.InstanceReports().MarkReportPublished(ctx, report.ID, main, int64(i+1)); err != nil {
						t.Fatal(err)
					}
				}
			}
			req := promptAdmissionRequest(workspace, main, "reports", now)
			req.HumanIngress, req.Operation = tc.human, tc.operation
			if tc.operation == store.SessionPromptOperationSteer || tc.exit == "live-steer" {
				req.Mode = store.SessionInputQueueModeSteer
			}
			req.DirectDispatch = tc.exit == "live-steer"
			first, created, err := db.ClaimSessionPromptAdmission(ctx, req)
			if err != nil || !created {
				t.Fatalf("admission: %#v %v %v", first, created, err)
			}
			reports, err := db.InstanceReports().
				ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, State: "reserved", Limit: 100})
			if err != nil || len(reports) != tc.want {
				t.Fatalf("reserved: %d, want %d: %v", len(reports), tc.want, err)
			}
			for _, report := range reports {
				if report.PromptAdmissionID != first.ID || report.EventSeq == 0 {
					t.Fatalf("invalid reservation: %#v", report)
				}
			}
			replayed, createdAgain, replayErr := db.ClaimSessionPromptAdmission(ctx, req)
			if replayErr != nil || createdAgain || replayed.ID != first.ID {
				t.Fatalf("replay: %#v %v %v", replayed, createdAgain, replayErr)
			}
			again, err := db.InstanceReports().
				ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, State: "reserved", Limit: 100})
			if err != nil || len(again) != tc.want {
				t.Fatalf("replayed reservation: %d, want %d: %v", len(again), tc.want, err)
			}

			if tc.exit == "live-steer" {
				_, _, _, err := db.StageAdmittedSessionSteer(
					ctx,
					req,
					store.SessionInputQueueInsert{
						ID:           "live-reports",
						Delivery:     store.SessionInputDeliveryInterruptThenPrompt,
						SessionID:    main,
						Text:         req.AuthoredText,
						Mode:         store.SessionInputQueueModeSteer,
						TargetTurnID: "turn-live",
						QueueCap:     10,
						Now:          now,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				pending, err := db.InstanceReports().
					ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, State: "pending", Limit: 100})
				if err != nil || len(pending) != 23 {
					t.Fatalf("steer pending=%d err=%v", len(pending), err)
				}
				return
			}
			if tc.exit == "dispatch" || tc.exit == "indeterminate" || tc.exit == "start-failure" {
				if err := db.CommitSessionPromptDispatch(ctx, workspace, main, req.IdempotencyKey, now); err != nil {
					t.Fatal(err)
				}
				wantState, wantCount := "delivered", 20
				if tc.exit != "start-failure" {
					delivered, err := db.DispatchSessionReports(ctx, workspace, main, []string{req.MessageID}, now)
					if err != nil || len(delivered) != 20 {
						t.Fatalf("dispatch: %d %v", len(delivered), err)
					}
					duplicate, err := db.DispatchSessionReports(ctx, workspace, main, []string{req.MessageID}, now)
					if err != nil || len(duplicate) != 0 {
						t.Fatalf("duplicate dispatch: %d %v", len(duplicate), err)
					}
				}
				if tc.exit != "dispatch" {
					if err := db.MarkSessionPromptAdmissionIndeterminate(
						ctx,
						workspace,
						main,
						req.IdempotencyKey,
						"driver failed",
						now,
					); err != nil {
						t.Fatal(err)
					}
					wantState = "indeterminate"
					if tc.exit == "start-failure" {
						wantState, wantCount = "pending", 23
					}
				}
				rows, err := db.InstanceReports().
					ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, State: wantState, Limit: 100})
				if err != nil || len(rows) != wantCount {
					t.Fatalf("state %s: %d, want %d: %v", wantState, len(rows), wantCount, err)
				}
				return
			}
			if tc.exit == "restart" || tc.exit == "restart-dispatched" {
				state, count := "pending", 23
				if tc.exit == "restart-dispatched" {
					if err := db.CommitSessionPromptDispatch(
						ctx,
						workspace,
						main,
						req.IdempotencyKey,
						now,
					); err != nil {
						t.Fatal(err)
					}
					if _, err := db.DispatchSessionReports(
						ctx,
						workspace,
						main,
						[]string{req.MessageID},
						now,
					); err != nil {
						t.Fatal(err)
					}
					state, count = "indeterminate", 20
				}
				if _, err := db.FailStaleDispatchingSessionInputs(ctx, now); err != nil {
					t.Fatal(err)
				}
				rows, err := db.InstanceReports().
					ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, State: state, Limit: 100})
				if err != nil || len(rows) != count {
					t.Fatalf("restart: %d %v", len(rows), err)
				}
				return
			}
			if tc.exit != "" {
				_, entry, _, _, err := db.EnqueueAdmittedSessionInput(ctx, req, store.SessionInputQueueInsert{
					ID:        "queued-reports",
					SessionID: main,
					Text:      req.AuthoredText,
					Mode:      store.SessionInputQueueModeQueue,
					QueueCap:  10,
					Now:       now,
				})
				if err != nil {
					t.Fatal(err)
				}
				replacement := store.SessionInputQueueInsert{
					ID:           "replacement",
					Text:         req.AuthoredText,
					TargetTurnID: "turn-live",
					QueueCap:     10,
					Now:          now,
				}
				switch tc.exit {
				case "cancel":
					_, err = db.CancelSessionInput(ctx, main, entry.ID, now)
				case "clear":
					_, err = db.ClearSessionInputs(
						ctx,
						store.SessionInputClearRequest{
							SessionID: main,
							ActorKind: "human",
							ActorID:   "operator",
							Now:       now,
						},
					)
				case "replace":
					_, _, err = db.ReplaceSessionInput(ctx, main, entry.ID, replacement)
				case "steer":
					_, _, err = db.PromoteSessionInputToSteer(ctx, main, entry.ID, replacement)
				case "task-cancel":
					err = db.withTaskMutationTransactionForTest(
						ctx,
						"cancel task input",
						func(tx *taskMutationTxStore) error {
							_, err := tx.InvalidateForceRunInputs(ctx, main, now)
							return err
						},
					)
				case "stop":
					_, err = db.CancelPendingSessionInputs(ctx, main, entry.SessionGeneration+1, now)
				}
				if err != nil {
					t.Fatal(err)
				}
				pending, err := db.InstanceReports().
					ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, State: "pending", Limit: 100})
				if err != nil || len(pending) != 23 {
					t.Fatalf("released reports: %d, want 23: %v", len(pending), err)
				}
				for _, report := range pending {
					if report.PromptAdmissionID != "" {
						t.Fatalf("retained admission: %s", report.PromptAdmissionID)
					}
				}
			}
		})
	}
}

func TestInstanceReportReservationUnderSteerDefault(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		direct bool
		want   int
	}{
		{"Should reserve for a prompt starting a turn on an idle session", true, 1},
		{"Should leave reports for a prompt that steers a running turn", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			ctx := t.Context()
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "steer-default", "sess-steer-default")
			instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, WorkspaceID: workspace, AgentName: "coder",
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, 10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().
				ExecContext(ctx, "UPDATE agent_instances SET state='active', main_session_id=? WHERE id=?", main, instance.ID); err != nil {
				t.Fatal(err)
			}
			report, err := db.InstanceReports().AppendReport(ctx, instance.ID, agentinstance.ReportInput{
				SourceKey: "occ:steer-default", Kind: "failure", Payload: []byte(`{"reason":"failed","detail":"test"}`),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.InstanceReports().MarkReportPublished(ctx, report.ID, main, 1); err != nil {
				t.Fatal(err)
			}
			req := promptAdmissionRequest(workspace, main, "steer-default", time.Now().UTC())
			req.HumanIngress, req.Operation = true, store.SessionPromptOperationPrompt
			req.Mode, req.DirectDispatch = store.SessionInputQueueModeSteer, tc.direct
			if _, created, err := db.ClaimSessionPromptAdmission(ctx, req); err != nil || !created {
				t.Fatalf("admission: %v %v", created, err)
			}
			reserved, err := db.InstanceReports().
				ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, State: "reserved", Limit: 10})
			if err != nil || len(reserved) != tc.want {
				t.Fatalf("reserved: %d, want %d: %v", len(reserved), tc.want, err)
			}
			if tc.direct {
				_, _, created, err := db.StageAdmittedSessionSteer(ctx, req, store.SessionInputQueueInsert{
					ID: "fallback-steer", SessionID: main, Text: req.AuthoredText,
					TargetTurnID: "turn-live", Mode: store.SessionInputQueueModeSteer,
					Delivery: store.SessionInputDeliveryInterruptThenPrompt,
					QueueCap: 10, Now: req.Now,
				})
				if err != nil || !created {
					t.Fatalf("stage fallback steer: created=%v error=%v", created, err)
				}
				pending, err := db.InstanceReports().ListReports(ctx, agentinstance.ReportQuery{
					InstanceID: instance.ID, State: "pending", Limit: 10,
				})
				if err != nil || len(pending) != 1 || pending[0].PromptAdmissionID != "" {
					t.Fatalf("released fallback reports: %#v, %v", pending, err)
				}
			}
		})
	}
}

// failingDecideStore records nothing when a decision is written, standing in for a ledger
// failure after the skill was published.
type failingDecideStore struct {
	agentinstance.SkillReviewStore
	err error
}

func (s failingDecideStore) DecideReview(context.Context, string, string, string, string) error {
	return s.err
}

func TestCheckInActivityReports(t *testing.T) {
	for _, mode := range []string{"quiet", "canceled", "deduped", "done"} {
		canceled := mode == "canceled"
		t.Run(
			fmt.Sprintf("Should retain audited activity after %s [IT-003]", mode),
			func(t *testing.T) {
				t.Parallel()
				db := openTestGlobalDB(t)
				workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "activity", "sess-activity")
				repo := db.Instances()
				instance, err := repo.Adopt(
					t.Context(),
					agentinstance.Instance{
						ProfileID:         store.DefaultProfileID,
						AgentName:         "coder",
						WorkspaceID:       workspace,
						PermissionCeiling: "{}",
						CreatedByKind:     "human",
					},
					main,
					10,
				)
				if err != nil {
					t.Fatal(err)
				}
				now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
				refs := []source.Ref{{ID: "activity-source", Script: "poll"}}
				states, err := repo.SourceStates(t.Context(), instance.ID, refs, false, now)
				if err != nil {
					t.Fatal(err)
				}
				batch := source.Result{
					Items: []source.Item{{Key: "one", Title: "First"}, {Key: "two", Title: "Second"}},
					State: []byte(`{"cursor":2}`),
				}
				polled := []heartbeat.PolledSource{
					{Ref: refs[0], RefDigest: states[0].RefDigest, Generation: states[0].Generation, Result: batch},
				}
				admitted, err := repo.Admit(
					t.Context(),
					heartbeat.AdmitRequest{
						Sources:       polled,
						InstanceID:    instance.ID,
						OccurrenceID:  "activity-occ",
						Now:           now,
						NextWakeAt:    now.Add(time.Hour),
						MaxConcurrent: 2,
					},
				)
				if err != nil || !admitted {
					t.Fatalf("admit=%v err=%v", admitted, err)
				}
				policy := toolspkg.CheckInPolicy{
					InstanceID:    instance.ID,
					OccurrenceID:  "activity-occ",
					MaxToolCalls:  5,
					MaxWriteCalls: 5,
				}
				decision, err := repo.Begin(
					t.Context(),
					toolspkg.CheckInAttempt{
						ToolID:  "mcp__fixture__write",
						Backend: toolspkg.BackendMCP,
						Access:  toolspkg.AccessWrite,
					},
					policy,
				)
				if err != nil || !decision.Allowed {
					t.Fatalf("begin=%+v err=%v", decision, err)
				}
				reportInput := (*agentinstance.ReportInput)(nil)
				finishResult := "quiet"
				if mode == "done" || mode == "deduped" {
					if err := repo.FinishToolCall(
						t.Context(),
						"activity-occ",
						decision.Seq,
						"ok",
						"",
						time.Millisecond,
					); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "deduped" {
					reportInput = &agentinstance.ReportInput{
						SourceKey: "occ:previous",
						Kind:      "proposal",
						Payload: []byte(
							`{"proposals":[{"title":"Repeat","body":"Repeat this work","item_keys":["one"]}]}`,
						),
					}
					if _, err := db.InstanceReports().AppendReport(t.Context(), instance.ID, *reportInput); err != nil {
						t.Fatal(err)
					}
					finishResult = "reported"
				}
				if canceled {
					if _, err := db.DB().
						ExecContext(t.Context(), "UPDATE agent_instance_wake_state SET occurrence_cancelled=1 WHERE instance_id=?", instance.ID); err != nil { //nolint:misspell // Persisted SQL column.
						t.Fatal(err)
					}
				}
				result, err := repo.Finish(
					t.Context(),
					heartbeat.FinishRequest{
						InstanceID:   instance.ID,
						OccurrenceID: "activity-occ",
						Source:       "instance_check",
						Digest:       "completed-digest",
						Result:       finishResult,
						Report:       reportInput,
						Reason:       "quiet",
						Now:          now,
						ExpiresAt:    now.Add(time.Hour),
					},
				)
				if err != nil || !result.Changed {
					t.Fatalf("finish=%+v err=%v", result, err)
				}
				calls, err := repo.CheckInToolCalls(t.Context(), instance.ID, "activity-occ")
				wantCall, wantEffect := "unknown", "maybe"
				if mode == "done" || mode == "deduped" {
					wantCall, wantEffect = "ok", "done"
				}
				if err != nil || len(calls) != 1 || calls[0].Outcome != wantCall || calls[0].Effect != wantEffect {
					t.Fatalf("calls=%+v err=%v", calls, err)
				}
				reports, err := db.InstanceReports().
					ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 10})
				activities := 0
				for _, report := range reports {
					if report.SourceKey == "occ:activity-occ" && report.Kind == "activity" {
						activities++
					}
				}
				if err != nil || activities != 1 {
					t.Fatalf("reports=%+v err=%v", reports, err)
				}
				var outcome string
				if err := db.DB().
					QueryRowContext(t.Context(), "SELECT result FROM agent_heartbeat_wake_events WHERE occurrence_id='activity-occ'").
					Scan(&outcome); err != nil {
					t.Fatal(err)
				}
				want := "quiet"
				if canceled {
					want = string(heartbeat.WakeResultCancelled)
				}
				states, err = repo.SourceStates(t.Context(), instance.ID, refs, false, now)
				if err != nil {
					t.Fatal(err)
				}
				wake, err := repo.WakeState(t.Context(), instance.ID)
				if err != nil {
					t.Fatal(err)
				}
				if canceled && (len(states[0].State) != 0 || wake.LastDigest != "") {
					t.Fatalf("canceled source committed: %+v %+v", states, wake)
				}
				if !canceled &&
					(!bytes.Equal(states[0].State, batch.State) || wake.LastDigest != "completed-digest" || wake.QuietStreak != 0) {
					t.Fatalf("completed state=%+v wake=%+v", states, wake)
				}
				repeat, err := repo.Finish(
					t.Context(),
					heartbeat.FinishRequest{
						InstanceID:   instance.ID,
						OccurrenceID: "activity-occ",
						Source:       "instance_check",
						Result:       "quiet",
						Now:          now,
						ExpiresAt:    now.Add(time.Hour),
					},
				)
				if err != nil || repeat.Changed {
					t.Fatalf("repeat=%+v err=%v", repeat, err)
				}
				if _, err := repo.CheckInOccurrence(
					t.Context(),
					instance.ID,
					"missing",
				); !errors.Is(
					err,
					agentinstance.ErrNotFound,
				) {
					t.Fatalf("missing occurrence: %v", err)
				}
				if outcome != want {
					t.Fatalf("outcome=%s want=%s", outcome, want)
				}
			},
		)
	}
}

func TestInstanceProposalPicks(t *testing.T) {
	for _, mode := range []string{"auto", "operator", "unapproved", "limit", "queue_full", "no_main", "unattended", "canceled"} {
		t.Run("Should enforce pick transaction and lifecycle for "+mode+" [IT-003 IT-004]", func(t *testing.T) {
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "picks", "sess-picks")
			repo := db.Instances()
			instance, err := repo.Adopt(
				t.Context(),
				agentinstance.Instance{
					ProfileID:         store.DefaultProfileID,
					AgentName:         "coder",
					WorkspaceID:       workspace,
					PermissionCeiling: "{}",
					CreatedByKind:     "human",
				},
				main,
				10,
			)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			actions := []heartbeat.ActionDecl{
				{ID: "report", Title: "Report", Instructions: "Write a report.", AutoAccept: true},
			}
			if mode == "operator" {
				actions[0].AutoAccept = false
			}
			digest, err := heartbeat.AuthorityDigest(heartbeat.CheckInsPolicy{}, heartbeat.AuthorityActions(actions))
			if err != nil {
				t.Fatal(err)
			}
			if mode != "unapproved" {
				if err := repo.ApproveAuthority(t.Context(), instance.ProfileID, dir, digest, "cli", now); err != nil {
					t.Fatal(err)
				}
			}
			admitted, err := repo.Admit(
				t.Context(),
				heartbeat.AdmitRequest{
					InstanceID:    instance.ID,
					OccurrenceID:  "pick-occ",
					Now:           now,
					NextWakeAt:    now.Add(time.Hour),
					MaxConcurrent: 2,
				},
			)
			if err != nil || !admitted {
				t.Fatalf("admit=%v err=%v", admitted, err)
			}
			limit, queueCap, unattended := 20, 10, int64(300)
			if mode == "operator" {
				limit = 0
			}
			if mode == "limit" {
				for i := range 20 {
					report, err := db.InstanceReports().
						AppendReport(t.Context(), instance.ID, agentinstance.ReportInput{SourceKey: fmt.Sprintf("occ:seed-%d", i), Kind: "failure", Payload: []byte(`{"reason":"seed","detail":"Historical pick"}`)})
					if err != nil {
						t.Fatal(err)
					}
					_, err = db.DB().
						ExecContext(t.Context(), `INSERT INTO agent_instance_proposal_picks(report_id,proposal_index,instance_id,actor,target_session_id,picked_at) VALUES(?,0,?,'auto_accept',?,?)`, report.ID, instance.ID, main, store.FormatTimestamp(now.Add(-time.Hour)))
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "queue_full" {
				queueCap = 1
				_, _, err := db.EnqueueSessionInput(
					t.Context(),
					store.SessionInputQueueInsert{
						ID:        "full",
						SessionID: main,
						Text:      "Existing",
						Mode:      "queue",
						Delivery:  "after_turn",
						QueueCap:  1,
						Now:       now,
						OwnerKind: "synthetic",
						TurnID:    "full-turn",
						SyntheticPrompt: &store.SessionInputSyntheticPrompt{
							RunID:    "full-run",
							Metadata: []byte(`{"kind":"fixture"}`),
						},
					},
				)
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unattended" {
				unattended = 1
			}
			if mode == "canceled" {
				if _, err := repo.Pause(t.Context(), instance.ID, "manual"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "no_main" {
				if _, err := db.DB().
					ExecContext(t.Context(), "UPDATE agent_instances SET main_session_id=NULL WHERE id=?", instance.ID); err != nil {
					t.Fatal(err)
				}
			}
			outcome := heartbeat.TriageReplyOutcome(
				`{"proposals":[{"title":"Report this","body":"Evidence","item_keys":[],"action":"report"}]}`,
				heartbeat.CheckDigest{},
				2,
				actions...)
			req := heartbeat.FinishRequest{
				InstanceID:          instance.ID,
				OccurrenceID:        "pick-occ",
				Source:              "timer",
				Result:              outcome.Result,
				Reason:              outcome.Reason,
				Report:              outcome.Report,
				Now:                 now,
				ExpiresAt:           now.Add(time.Hour),
				Actions:             actions,
				AuthorityDigest:     digest,
				DefinitionDir:       dir,
				AutoAcceptPerDay:    limit,
				QueueCap:            queueCap,
				UnattendedTurnLimit: unattended,
			}
			result, err := repo.Finish(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "auto" {
				want = 1
			}
			if len(result.Picks) != want {
				t.Fatalf("picks=%+v", result.Picks)
			}
			retry, err := repo.Finish(t.Context(), req)
			if err != nil || len(retry.Picks) != 0 {
				t.Fatalf("retry=%+v err=%v", retry, err)
			}
			if mode == "auto" {
				admitted, err := repo.Admit(
					t.Context(),
					heartbeat.AdmitRequest{
						InstanceID:    instance.ID,
						OccurrenceID:  "pick-next",
						Now:           now.Add(time.Minute),
						NextWakeAt:    now.Add(time.Hour),
						MaxConcurrent: 2,
						ManualCheck:   true,
					},
				)
				if err != nil || !admitted {
					t.Fatalf("next admit=%v err=%v", admitted, err)
				}
				next := req
				next.OccurrenceID = "pick-next"
				next.Now = now.Add(time.Minute)
				second, err := repo.Finish(t.Context(), next)
				if err != nil || len(second.Picks) != 0 {
					t.Fatalf("deduped finish=%+v err=%v", second, err)
				}
			}
			if mode == "canceled" {
				return
			}
			reports, err := db.InstanceReports().
				ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 100})
			wantReports := 1
			if mode == "limit" {
				wantReports = 21
			}
			if mode == "unattended" {
				wantReports = 2
			}
			if err != nil || len(reports) != wantReports {
				t.Fatalf("reports=%+v err=%v", reports, err)
			}
			var proposalReport agentinstance.Report
			for _, report := range reports {
				if report.Kind == "proposal" {
					proposalReport = report
					break
				}
			}
			if err := db.InstanceReports().
				ProjectPicks(t.Context(), &proposalReport, actionSnapshots(actions)...); err != nil {
				t.Fatal(err)
			}
			var projection struct {
				Proposals []agentinstance.ProposalPayload `json:"proposals"`
			}
			if err := json.Unmarshal(proposalReport.Payload, &projection); err != nil {
				t.Fatal(err)
			}
			expectedStatus := map[string]string{"auto": "picked", "limit": "limit", "queue_full": "queue_full", "unapproved": "unapproved", "no_main": "no_main", "unattended": "inactive", "operator": "none"}[mode]
			if expectedStatus != "" && projection.Proposals[0].AutoAcceptStatus != expectedStatus {
				t.Fatalf("projection=%+v", projection)
			}
			if mode == "queue_full" || mode == "limit" {
				if _, err := db.DB().
					ExecContext(t.Context(), "UPDATE session_input_queue SET status='canceled' WHERE session_id=?", main); err != nil {
					t.Fatal(err)
				}
				reader := db.InstanceReports()
				reader.now = func() time.Time { return now.Add(25 * time.Hour) }
				reread, err := reader.ListReports(
					t.Context(),
					agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 100},
				)
				if err != nil {
					t.Fatal(err)
				}
				for _, report := range reread {
					if report.ID == proposalReport.ID {
						var again struct {
							Proposals []agentinstance.ProposalPayload `json:"proposals"`
						}
						if err := json.Unmarshal(report.Payload, &again); err != nil {
							t.Fatal(err)
						}
						if again.Proposals[0].AutoAcceptStatus != expectedStatus {
							t.Fatalf("skip lost: %+v", again)
						}
					}
				}
			}
			if mode == "operator" {
				pickReq := agentinstance.PickRequest{
					InstanceID:    instance.ID,
					ReportID:      reports[0].ID,
					Actor:         "operator",
					ProposalIndex: 0,
					QueueCap:      10,
				}
				pick, err := repo.PickProposal(t.Context(), pickReq)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := repo.PickProposal(t.Context(), pickReq); !errors.Is(err, agentinstance.ErrAlreadyPicked) {
					t.Fatalf("repeat=%v", err)
				}
				if err := db.InstanceReports().ProjectPicks(t.Context(), &proposalReport); err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(proposalReport.Payload, []byte(`"auto_accept_status":"picked"`)) {
					t.Fatal("operator pick mislabeled auto")
				}
				entry, err := db.GetSessionInputQueueEntry(t.Context(), main, pick.QueueEntryID)
				if err != nil || entry.OriginClass != "human" {
					t.Fatalf("entry=%+v err=%v", entry, err)
				}
			}
			if mode == "auto" {
				metrics, err := repo.Metrics(t.Context(), instance.ProfileID, instance.WorkspaceID)
				if err != nil || metrics.Picks["auto_accept"] != 1 {
					t.Fatalf("pick metrics=%+v %v", metrics, err)
				}
				checks, err := repo.ListCheckIns(t.Context(), instance.ID, "", 100)
				if err != nil {
					t.Fatal(err)
				}
				var autoCount int64
				for _, check := range checks {
					autoCount += check.Proposals.AutoAccepted
					if check.OccurrenceID == "" {
						continue
					}
					one, err := repo.CheckInOccurrence(t.Context(), instance.ID, check.OccurrenceID)
					if err != nil || one.Proposals != check.Proposals {
						t.Fatalf("occurrence read=%+v list=%+v err=%v", one.Proposals, check.Proposals, err)
					}
				}
				if autoCount != 1 {
					t.Fatalf("check-in proposals=%+v", checks)
				}
				pick := result.Picks[0]
				entry, err := db.GetSessionInputQueueEntry(t.Context(), main, pick.QueueEntryID)
				if err != nil || entry.OwnerKind != "synthetic" || entry.OriginClass != "synthetic" ||
					entry.MessageID != "pick:"+reports[0].ID+":0" {
					t.Fatalf("entry=%+v err=%v", entry, err)
				}
				if _, err := repo.Pause(t.Context(), instance.ID, "manual"); err != nil {
					t.Fatal(err)
				}
				entry, err = db.GetSessionInputQueueEntry(t.Context(), main, pick.QueueEntryID)
				if err != nil || entry.Status != "canceled" {
					t.Fatalf("entry=%+v err=%v", entry, err)
				}
				if err := db.InstanceReports().ProjectPicks(t.Context(), &proposalReport); err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(
					proposalReport.Payload,
					[]byte(`"state":"cancelled"`), //nolint:misspell // projected wire enum
				) {
					t.Fatalf("paused pick=%s", proposalReport.Payload)
				}
			}
		})
	}
}
