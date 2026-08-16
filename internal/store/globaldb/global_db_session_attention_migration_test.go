package globaldb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBSessionAttentionMigration(t *testing.T) {
	t.Parallel()

	t.Run("Should add canonical attention state and preserve it across reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00065_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open prefix before 00065 error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		now := store.FormatTimestamp(time.Date(2026, 8, 15, 20, 45, 0, 0, time.UTC))
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO workspaces (
			id, root_dir, add_dirs, name, created_at, updated_at
		) VALUES ('ws-attention-migration', ?, '[]', 'attention-migration', ?, ?)`,
			t.TempDir(), now, now); err != nil {
			t.Fatalf("seed workspace before 00065 error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO sessions (
			id, name, agent_name, workspace_id, session_type, state, created_at, updated_at
		) VALUES ('sess-attention-migration', 'Attention migration', 'coder',
			'ws-attention-migration', 'user', 'active', ?, ?)`, now, now); err != nil {
			t.Fatalf("seed session before 00065 error = %v", err)
		}
		columns, err := tableColumns(ctx, prefixDB, "sessions")
		if err != nil {
			t.Fatalf("tableColumns(sessions before 00065) error = %v", err)
		}
		if _, exists := columns["attention_revision"]; exists {
			t.Fatal("sessions.attention_revision exists before 00065")
		}
		pendingExists, err := tableExists(ctx, prefixDB, "session_pending_interactions")
		if err != nil {
			t.Fatalf("tableExists(session_pending_interactions before 00065) error = %v", err)
		}
		if pendingExists {
			t.Fatal("session_pending_interactions exists before 00065")
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close prefix before 00065 error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade through 00065 error = %v", err)
		}
		upgradedClosed := false
		t.Cleanup(func() {
			if upgradedClosed {
				return
			}
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close upgraded database error = %v", closeErr)
			}
		})
		assertTableHasColumns(t, upgraded.db, "sessions", []string{
			"pending_permission_count",
			"pending_clarify_count",
			"attention_revision",
			"last_settled_revision",
			"last_seen_revision",
			"last_seen_at",
			"attention_changed_at",
		})
		assertTableHasColumns(t, upgraded.db, "session_pending_interactions", []string{
			"interaction_id", "session_id", "kind", "provider_request_id", "status",
		})
		assertIndexesPresent(
			t,
			upgraded.db,
			"session_pending_interactions",
			"idx_session_pending_interactions_session_status",
			"uq_session_pending_interactions_active_provider_request",
		)
		attention, err := upgraded.GetSessionAttention(ctx, "sess-attention-migration")
		if err != nil {
			t.Fatalf("GetSessionAttention(after migration) error = %v", err)
		}
		if attention != (store.SessionAttention{}) {
			t.Fatalf("migrated attention defaults = %#v, want zero projection", attention)
		}
		commit, err := upgraded.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
			InteractionID: "interaction-migration", SessionID: "sess-attention-migration",
			Kind: store.PendingInteractionKindClarify, ProviderRequestID: "request-migration",
			Title: "Migration question", CreatedAt: time.Date(2026, 8, 15, 20, 46, 0, 0, time.UTC),
		})
		if err != nil || commit.After.PendingClarifyCount != 1 || commit.After.AttentionRevision != 1 {
			t.Fatalf("CreatePendingInteraction(after migration) = %#v, error = %v", commit, err)
		}
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(after migration) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("close upgraded database before reopen error = %v", err)
		}
		upgradedClosed = true

		reopened, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("reopen migrated database error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close reopened database error = %v", closeErr)
			}
		})
		interactions, err := reopened.ListPendingInteractions(ctx, "sess-attention-migration", nil)
		if err != nil {
			t.Fatalf("ListPendingInteractions(after reopen) error = %v", err)
		}
		if len(interactions) != 1 || interactions[0].InteractionID != "interaction-migration" {
			t.Fatalf("interactions after reopen = %#v, want durable canonical row", interactions)
		}
		reopenedAttention, err := reopened.GetSessionAttention(ctx, "sess-attention-migration")
		if err != nil {
			t.Fatalf("GetSessionAttention(after reopen) error = %v", err)
		}
		if reopenedAttention.PendingClarifyCount != 1 || reopenedAttention.AttentionRevision != 1 {
			t.Fatalf("attention after reopen = %#v, want durable projection", reopenedAttention)
		}
	})
}
