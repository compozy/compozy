package globaldb

import (
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
	for _, tc := range []struct{ name, status, kind, key string }{
		{"Should atomically report expiry once", store.PendingInteractionStatusTimedOut, "approval_expired", "interaction:pending-report"},
		{"Should atomically report interruption once", store.PendingInteractionStatusCanceled, "interrupted", "turn:turn-report"},
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
			instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
			registerSubagentSession(t, db, workspace, "child-report", main, store.SubagentSpawnRole, now)
			_, err = db.CreatePendingInteraction(t.Context(), store.PendingInteractionCreate{InteractionID: "pending-report", SessionID: "child-report", Kind: store.PendingInteractionKindPermission, ProviderRequestID: "request-report", TurnID: "turn-report", CreatedAt: now})
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.DB().ExecContext(t.Context(), `CREATE TRIGGER reject_report BEFORE INSERT ON agent_instance_reports BEGIN SELECT RAISE(ABORT, 'injected report failure'); END`)
			if err != nil {
				t.Fatal(err)
			}
			transition := store.PendingInteractionTransition{InteractionID: "pending-report", Status: tc.status, At: now.Add(time.Hour), Resolution: "reject_once", ResolvedBy: "timeout"}
			if tc.status == store.PendingInteractionStatusCanceled {
				transition.Resolution, transition.ResolvedBy = "failed-by-restart", "system"
			}
			if _, err := db.TransitionPendingInteraction(t.Context(), transition); err == nil {
				t.Fatal("expected report insert failure")
			}
			rows, err := db.ListPendingInteractions(t.Context(), "child-report", []string{store.PendingInteractionStatusPending})
			if err != nil || len(rows) != 1 {
				t.Fatalf("rollback rows = %#v, %v", rows, err)
			}
			if _, err := db.DB().ExecContext(t.Context(), `DROP TRIGGER reject_report`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().ExecContext(t.Context(), `CREATE TRIGGER reject_transition BEFORE UPDATE ON session_pending_interactions BEGIN SELECT RAISE(ABORT, 'injected transition failure'); END`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.TransitionPendingInteraction(t.Context(), transition); err == nil {
				t.Fatal("expected transition failure")
			}
			rolledBack, err := db.InstanceReports().ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 10})
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
				child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPendingInteractionReportsAtomicity$")
				child.Env = append(os.Environ(), "COMPOZY_B4_CRASH_DB="+path, "COMPOZY_B4_CRASH_PHASE="+phase, "COMPOZY_B4_CRASH_TRANSITION="+string(encoded))
				output, childErr := child.CombinedOutput()
				var exitErr *exec.ExitError
				if !errors.As(childErr, &exitErr) || exitErr.ExitCode() != 23 {
					t.Fatalf("crash child: %v: %s", childErr, output)
				}
				db, err = OpenGlobalDB(t.Context(), path)
				if err != nil {
					t.Fatal(err)
				}
				if phase == "before" {
					pending, err := db.ListPendingInteractions(t.Context(), "child-report", []string{store.PendingInteractionStatusPending})
					if err != nil || len(pending) != 1 {
						t.Fatalf("pre-commit crash pending = %#v, %v", pending, err)
					}
					continue
				}
				if _, err := db.TransitionPendingInteraction(t.Context(), transition); err != nil {
					t.Fatal(err)
				}
			}
			late, err := db.TransitionPendingInteraction(t.Context(), store.PendingInteractionTransition{
				InteractionID: "pending-report", Status: store.PendingInteractionStatusResolved,
				At: now.Add(2 * time.Hour), Resolution: "allow_once", ResolvedBy: "human",
			})
			if err != nil || late.Outcome != store.PendingInteractionOutcomeAlreadyResolved {
				t.Fatalf("late answer = %#v, %v", late, err)
			}
			reports, err := db.InstanceReports().ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 10})
			if err != nil || len(reports) != 1 || reports[0].Kind != tc.kind || reports[0].SourceKey != tc.key {
				t.Fatalf("reports = %#v, %v", reports, err)
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

		{"Should release reports when canceled", true, store.SessionPromptOperationPrompt, 20, "cancel"},
		{"Should release reports when cleared", true, store.SessionPromptOperationPrompt, 20, "clear"},
		{"Should release reports when replaced", true, store.SessionPromptOperationPrompt, 20, "replace"},
		{"Should release reports when promoted to steer", true, store.SessionPromptOperationPrompt, 20, "steer"},
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
			if tc.operation == store.SessionPromptOperationSteer {
				req.Mode = store.SessionInputQueueModeSteer
			}
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
