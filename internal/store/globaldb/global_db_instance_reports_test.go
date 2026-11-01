package globaldb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

func TestInstanceReports(t *testing.T) {
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
		_, err = db.DB().ExecContext(ctx, `INSERT INTO agent_instances (id,profile_id,agent_name,workspace_id,state,main_session_id,created_by_kind,created_by_id,created_at,updated_at) VALUES (?,?,?,?,'active',?,'human','',?,?)`, "ainst-reports", store.DefaultProfileID, "coder", workspace, main, store.FormatTimestamp(now), store.FormatTimestamp(now))
		if err != nil {
			t.Fatal(err)
		}
		repo := db.InstanceReports()
		input := agentinstance.ReportInput{SourceKey: "occ:reports-1", Kind: "failure", Payload: []byte(`{"reason":"failed","detail":"provider exited"}`)}
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
		next, err := repo.ListReports(ctx, agentinstance.ReportQuery{InstanceID: "ainst-reports", Limit: 1, AfterCreatedAt: page[0].CreatedAt, AfterID: page[0].ID})
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
		if _, err := db.DB().ExecContext(ctx, `UPDATE agent_instances SET main_session_id=? WHERE id=?`, successor, "ainst-reports"); err != nil {
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
		reports, err := reopened.InstanceReports().ListReports(ctx, agentinstance.ReportQuery{InstanceID: "ainst-reports", Limit: 10})
		if err != nil || len(reports) != 2 {
			t.Fatalf("reopened: %v %v", reports, err)
		}
	})
}
