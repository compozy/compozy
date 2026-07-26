package globaldb

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

const networkStoreTestWorkspaceID = "ws-network-store"

func TestNetworkChannels(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "Should create the network channels schema on open",
			run:  assertOpenGlobalDBCreatesNetworkChannelsSchema,
		},
		{
			name: "Should write and list network channels",
			run:  assertGlobalDBWriteAndListNetworkChannels,
		},
		{
			name: "Should reject duplicate channel creation without replacing metadata",
			run:  assertGlobalDBCreateNetworkChannelRejectsDuplicate,
		},
		{
			name: "Should isolate the same channel name between workspaces",
			run:  assertGlobalDBNetworkChannelsRemainWorkspaceQualified,
		},
		{
			name: "Should order subscriptions without identity filters deterministically",
			run:  assertGlobalDBNetworkSubscriptionsRemainDeterministic,
		},
		{
			name: "Should create channel and subscription atomically without replacing channel metadata",
			run:  assertGlobalDBNetworkSubscriptionChannelCommandIsAtomic,
		},
		{
			name: "Should patch network channels without replacing unspecified fields",
			run:  assertGlobalDBPatchNetworkChannelsPreservesUnspecifiedFields,
		},
		{
			name: "Should reject patches that break channel fanout coupling",
			run:  assertGlobalDBPatchNetworkChannelRejectsInvalidFanoutCoupling,
		},
		{
			name: "Should return sql.ErrNoRows for missing network channels",
			run:  assertGlobalDBGetNetworkChannelNotFound,
		},
		{
			name: "Should delete a network channel",
			run:  assertGlobalDBDeleteNetworkChannel,
		},
		{
			name: "Should cascade network channels when a workspace is deleted",
			run:  assertGlobalDBDeleteWorkspaceCascadesNetworkChannels,
		},
		{
			name: "Should wrap timestamp parse failures when listing network channels",
			run:  assertGlobalDBListNetworkChannelsWrapsTimestampParseFailures,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func assertGlobalDBNetworkSubscriptionChannelCommandIsAtomic(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-network-subscription-command",
		filepath.Join(t.TempDir(), "ws-network-subscription-command"),
	)
	recordedAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	if err := globalDB.RegisterSession(ctx, store.SessionInfo{
		ID:          "sess-subscription-command",
		AgentName:   "worker",
		Provider:    "codex",
		WorkspaceID: workspaceID,
		State:       "active",
		CreatedAt:   recordedAt,
		UpdatedAt:   recordedAt,
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	original := store.NetworkChannelEntry{
		WorkspaceID: workspaceID,
		Channel:     "coord.atomic",
		Purpose:     "Original authority",
		CreatedBy:   "operator:first",
		CreatedAt:   recordedAt,
		UpdatedAt:   recordedAt,
	}
	if err := globalDB.CreateNetworkChannel(ctx, original); err != nil {
		t.Fatalf("CreateNetworkChannel() error = %v", err)
	}
	replacement := original
	replacement.Purpose = "Must not replace"
	replacement.CreatedBy = "operator:second"
	entry := store.NetworkSubscriptionEntry{
		WorkspaceID: workspaceID,
		Channel:     original.Channel,
		SessionID:   "sess-subscription-command",
		Mode:        store.NetworkSubscriptionModeFull,
		CreatedAt:   recordedAt,
		UpdatedAt:   recordedAt,
	}
	if err := globalDB.PutNetworkSubscriptionWithChannel(ctx, replacement, entry); err != nil {
		t.Fatalf("PutNetworkSubscriptionWithChannel(existing) error = %v", err)
	}
	stored, err := globalDB.GetNetworkChannel(ctx, store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     original.Channel,
	})
	if err != nil {
		t.Fatalf("GetNetworkChannel() error = %v", err)
	}
	if stored.Purpose != original.Purpose || stored.CreatedBy != original.CreatedBy {
		t.Fatalf("stored channel = %#v, want original metadata", stored)
	}

	missingChannel := replacement
	missingChannel.Channel = "coord.rollback"
	invalidEntry := entry
	invalidEntry.Channel = missingChannel.Channel
	invalidEntry.SessionID = "sess-missing"
	if err := globalDB.PutNetworkSubscriptionWithChannel(ctx, missingChannel, invalidEntry); err == nil {
		t.Fatal("PutNetworkSubscriptionWithChannel(missing session) error = nil, want rollback")
	}
	if _, err := globalDB.GetNetworkChannel(ctx, store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     missingChannel.Channel,
	}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetNetworkChannel(rolled back) error = %v, want %v", err, sql.ErrNoRows)
	}
}

func assertGlobalDBCreateNetworkChannelRejectsDuplicate(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-network-create-only",
		filepath.Join(t.TempDir(), "ws-network-create-only"),
	)
	original := store.NetworkChannelEntry{
		WorkspaceID: workspaceID,
		Channel:     "ops",
		Purpose:     "Original purpose",
		CreatedBy:   "operator:first",
	}
	if err := globalDB.CreateNetworkChannel(ctx, original); err != nil {
		t.Fatalf("CreateNetworkChannel(first) error = %v", err)
	}

	replacement := original
	replacement.Purpose = "Replacement purpose"
	replacement.CreatedBy = "operator:second"
	if err := globalDB.CreateNetworkChannel(ctx, replacement); !errors.Is(err, store.ErrNetworkChannelExists) {
		t.Fatalf("CreateNetworkChannel(duplicate) error = %v, want ErrNetworkChannelExists", err)
	}

	stored, err := globalDB.GetNetworkChannel(ctx, store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     original.Channel,
	})
	if err != nil {
		t.Fatalf("GetNetworkChannel() error = %v", err)
	}
	if stored.Purpose != original.Purpose || stored.CreatedBy != original.CreatedBy {
		t.Fatalf("stored channel = %#v, want original metadata preserved", stored)
	}
}

func assertGlobalDBNetworkChannelsRemainWorkspaceQualified(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	workspaceA := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-network-isolation-a",
		filepath.Join(t.TempDir(), "ws-network-isolation-a"),
	)
	workspaceB := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-network-isolation-b",
		filepath.Join(t.TempDir(), "ws-network-isolation-b"),
	)
	writtenAt := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	for _, entry := range []store.NetworkChannelEntry{
		{
			WorkspaceID: workspaceA,
			Channel:     "ops",
			Purpose:     "Workspace A operations",
			CreatedAt:   writtenAt,
			UpdatedAt:   writtenAt,
		},
		{
			WorkspaceID: workspaceB,
			Channel:     "ops",
			Purpose:     "Workspace B operations",
			CreatedAt:   writtenAt,
			UpdatedAt:   writtenAt,
		},
	} {
		if err := globalDB.WriteNetworkChannel(ctx, entry); err != nil {
			t.Fatalf("WriteNetworkChannel(%s) error = %v", entry.WorkspaceID, err)
		}
	}

	for _, want := range []store.NetworkChannelEntry{
		{WorkspaceID: workspaceA, Channel: "ops", Purpose: "Workspace A operations"},
		{WorkspaceID: workspaceB, Channel: "ops", Purpose: "Workspace B operations"},
	} {
		got, err := globalDB.GetNetworkChannel(ctx, store.NetworkChannelRef{
			WorkspaceID: want.WorkspaceID,
			Channel:     want.Channel,
		})
		if err != nil {
			t.Fatalf("GetNetworkChannel(%s) error = %v", want.WorkspaceID, err)
		}
		if got.WorkspaceID != want.WorkspaceID || got.Channel != want.Channel || got.Purpose != want.Purpose {
			t.Fatalf("GetNetworkChannel(%s) = %#v, want %#v", want.WorkspaceID, got, want)
		}
		listed, err := globalDB.ListNetworkChannels(ctx, store.NetworkChannelQuery{
			WorkspaceID: want.WorkspaceID,
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ListNetworkChannels(%s) error = %v", want.WorkspaceID, err)
		}
		if len(listed) != 1 || listed[0].WorkspaceID != want.WorkspaceID || listed[0].Purpose != want.Purpose {
			t.Fatalf("ListNetworkChannels(%s) = %#v, want only workspace-qualified row", want.WorkspaceID, listed)
		}
	}

	sessionB := store.SessionInfo{
		ID:          "sess-network-isolation-b",
		AgentName:   "reviewer",
		Provider:    "codex",
		WorkspaceID: workspaceB,
		State:       "active",
		CreatedAt:   writtenAt,
		UpdatedAt:   writtenAt,
	}
	if err := globalDB.RegisterSession(ctx, sessionB); err != nil {
		t.Fatalf("RegisterSession(workspace B) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO network_channel_participants (workspace_id, channel, session_id) VALUES (?, ?, ?)`,
		workspaceA,
		"ops",
		sessionB.ID,
	); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("cross-workspace participant insert error = %v, want foreign key failure", err)
	}
}

func assertGlobalDBNetworkSubscriptionsRemainDeterministic(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-network-subscription-order",
		filepath.Join(t.TempDir(), "ws-network-subscription-order"),
	)
	recordedAt := time.Date(2026, 7, 13, 13, 0, 0, 0, time.UTC)
	if err := globalDB.WriteNetworkChannel(ctx, store.NetworkChannelEntry{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
		Purpose:     "Subscription ordering",
		CreatedAt:   recordedAt,
		UpdatedAt:   recordedAt,
	}); err != nil {
		t.Fatalf("WriteNetworkChannel() error = %v", err)
	}
	for _, session := range []store.SessionInfo{
		{
			ID:          "sess-subscription-a",
			AgentName:   "alpha",
			Provider:    "codex",
			WorkspaceID: workspaceID,
			State:       "active",
			CreatedAt:   recordedAt,
			UpdatedAt:   recordedAt,
		},
		{
			ID:          "sess-subscription-b",
			AgentName:   "beta",
			Provider:    "codex",
			WorkspaceID: workspaceID,
			State:       "active",
			CreatedAt:   recordedAt,
			UpdatedAt:   recordedAt,
		},
	} {
		if err := globalDB.RegisterSession(ctx, session); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", session.ID, err)
		}
	}
	for _, entry := range []store.NetworkSubscriptionEntry{
		{
			WorkspaceID: workspaceID,
			Channel:     "coord.core",
			ThreadID:    "thread_zeta",
			SessionID:   "sess-subscription-a",
			Mode:        store.NetworkSubscriptionModeFull,
		},
		{
			WorkspaceID: workspaceID,
			Channel:     "coord.core",
			ThreadID:    "thread_alpha",
			SessionID:   "sess-subscription-b",
			Mode:        store.NetworkSubscriptionModeFull,
		},
		{
			WorkspaceID: workspaceID,
			Channel:     "coord.core",
			ThreadID:    "thread_alpha",
			SessionID:   "sess-subscription-a",
			Mode:        store.NetworkSubscriptionModeMute,
		},
		{
			WorkspaceID: workspaceID,
			Channel:     "coord.core",
			SessionID:   "sess-subscription-b",
			Mode:        store.NetworkSubscriptionModeFull,
		},
	} {
		if err := globalDB.PutNetworkSubscription(ctx, entry); err != nil {
			t.Fatalf("PutNetworkSubscription(%q, %q) error = %v", entry.ThreadID, entry.SessionID, err)
		}
	}

	entries, err := globalDB.ListNetworkSubscriptions(ctx, store.NetworkSubscriptionQuery{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListNetworkSubscriptions() error = %v", err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.ThreadID+"/"+entry.SessionID)
	}
	want := []string{
		"thread_zeta/sess-subscription-a",
		"thread_alpha/sess-subscription-a",
		"thread_alpha/sess-subscription-b",
		"/sess-subscription-b",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("subscription order = %#v, want %#v", got, want)
	}
	fallback, err := globalDB.ListNetworkSubscriptions(ctx, store.NetworkSubscriptionQuery{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
		ThreadID:    "",
		SessionID:   "sess-subscription-b",
		ExactThread: true,
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("ListNetworkSubscriptions(exact channel fallback) error = %v", err)
	}
	if len(fallback) != 1 || fallback[0].ThreadID != "" ||
		fallback[0].SessionID != "sess-subscription-b" {
		t.Fatalf("exact channel fallback = %#v, want sess-subscription-b channel row", fallback)
	}
}

func assertOpenGlobalDBCreatesNetworkChannelsSchema(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)

	assertTablesPresent(t, globalDB.db, "network_channels")
	assertTableColumns(t, globalDB.db, "network_channels", []string{
		"workspace_id",
		"channel",
		"purpose",
		"created_by",
		"created_at",
		"updated_at",
		"fanout_policy",
		"coordinator_peer_id",
	})
	hasWorkspaceFK, err := tableHasForeignKey(testutil.Context(t), globalDB.db, "network_channels", "workspaces")
	if err != nil {
		t.Fatalf("tableHasForeignKey(network_channels, workspaces) error = %v", err)
	}
	if !hasWorkspaceFK {
		t.Fatal("network_channels is missing a workspaces foreign key")
	}
	for _, foreignKey := range []struct {
		table       string
		fromColumns []string
	}{
		{table: "network_channel_participants", fromColumns: []string{"workspace_id", "session_id"}},
		{table: "network_direct_rooms", fromColumns: []string{"workspace_id", "session_a"}},
		{table: "network_direct_rooms", fromColumns: []string{"workspace_id", "session_b"}},
		{table: "network_subscriptions", fromColumns: []string{"workspace_id", "session_id"}},
		{table: "network_thread_participants", fromColumns: []string{"workspace_id", "session_id"}},
		{table: "network_thread_session_token_stats", fromColumns: []string{"workspace_id", "session_id"}},
		{table: "network_work", fromColumns: []string{"workspace_id", "opened_by_session_id"}},
		{table: "network_work", fromColumns: []string{"workspace_id", "target_session_id"}},
	} {
		hasSessionFK, foreignKeyErr := tableHasCompositeForeignKey(
			testutil.Context(t),
			globalDB.db,
			foreignKey.table,
			"sessions",
			foreignKey.fromColumns,
			[]string{"workspace_id", "id"},
		)
		if foreignKeyErr != nil {
			t.Fatalf(
				"tableHasCompositeForeignKey(%s, %v) error = %v",
				foreignKey.table,
				foreignKey.fromColumns,
				foreignKeyErr,
			)
		}
		if !hasSessionFK {
			t.Fatalf("table %q is missing sessions%v composite foreign key", foreignKey.table, foreignKey.fromColumns)
		}
	}
	assertIndexesPresent(t, globalDB.db, "network_channels", "idx_network_channels_workspace_updated_at")
}

func assertGlobalDBWriteAndListNetworkChannels(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-alpha",
		filepath.Join(t.TempDir(), "ws-alpha"),
	)
	recordedAt := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return recordedAt }

	first := store.NetworkChannelEntry{
		Channel:     " coord.core ",
		WorkspaceID: workspaceID,
		Purpose:     "Cross-agent coordination",
		CreatedBy:   "codex",
	}
	if err := globalDB.WriteNetworkChannel(testutil.Context(t), first); err != nil {
		t.Fatalf("WriteNetworkChannel(first) error = %v", err)
	}
	if err := globalDB.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		Channel:     "coord.core",
		WorkspaceID: workspaceID,
		Purpose:     "Updated purpose",
		CreatedBy:   "claude",
		UpdatedAt:   recordedAt.Add(time.Minute),
	}); err != nil {
		t.Fatalf("WriteNetworkChannel(update) error = %v", err)
	}
	if err := globalDB.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		Channel:     "ops.alerts",
		WorkspaceID: workspaceID,
		Purpose:     "Operational alerts",
		CreatedBy:   "gemini",
		CreatedAt:   recordedAt.Add(2 * time.Minute),
		UpdatedAt:   recordedAt.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("WriteNetworkChannel(second) error = %v", err)
	}

	entry, err := globalDB.GetNetworkChannel(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
	})
	if err != nil {
		t.Fatalf("GetNetworkChannel() error = %v", err)
	}
	if got, want := entry.Channel, "coord.core"; got != want {
		t.Fatalf("entry.Channel = %q, want %q", got, want)
	}
	if got, want := entry.Purpose, "Updated purpose"; got != want {
		t.Fatalf("entry.Purpose = %q, want %q", got, want)
	}
	if got, want := entry.CreatedBy, "codex"; got != want {
		t.Fatalf("entry.CreatedBy = %q, want %q", got, want)
	}
	if got, want := entry.CreatedAt, recordedAt; !got.Equal(want) {
		t.Fatalf("entry.CreatedAt = %s, want %s", got, want)
	}

	entries, err := globalDB.ListNetworkChannels(testutil.Context(t), store.NetworkChannelQuery{
		WorkspaceID: workspaceID,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListNetworkChannels() error = %v", err)
	}
	if got, want := len(entries), 2; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}
	if got, want := entries[0].Channel, "ops.alerts"; got != want {
		t.Fatalf("entries[0].Channel = %q, want %q", got, want)
	}
}

func assertGlobalDBPatchNetworkChannelsPreservesUnspecifiedFields(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-alpha",
		filepath.Join(t.TempDir(), "ws-alpha"),
	)
	recordedAt := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	if err := globalDB.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		Channel:           "coord.core",
		WorkspaceID:       workspaceID,
		Purpose:           "Original purpose",
		FanoutPolicy:      store.NetworkFanoutPolicyCoordinator,
		CoordinatorPeerID: "reviewer.sess-a",
		CreatedBy:         "codex",
		CreatedAt:         recordedAt,
		UpdatedAt:         recordedAt,
	}); err != nil {
		t.Fatalf("WriteNetworkChannel() error = %v", err)
	}

	purpose := "Pair reviews"
	if err := globalDB.PatchNetworkChannel(
		testutil.Context(t),
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: "coord.core"},
		store.NetworkChannelPatch{Purpose: &purpose, UpdatedAt: recordedAt.Add(time.Minute)},
	); err != nil {
		t.Fatalf("PatchNetworkChannel(purpose) error = %v", err)
	}
	entry, err := globalDB.GetNetworkChannel(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
	})
	if err != nil {
		t.Fatalf("GetNetworkChannel(after purpose patch) error = %v", err)
	}
	if entry.Purpose != "Pair reviews" ||
		entry.FanoutPolicy != store.NetworkFanoutPolicyCoordinator ||
		entry.CoordinatorPeerID != "reviewer.sess-a" {
		t.Fatalf("entry after purpose patch = %#v", entry)
	}

	fanoutPolicy := store.NetworkFanoutPolicyCapabilityMatch
	coordinatorPeerID := ""
	if err := globalDB.PatchNetworkChannel(
		testutil.Context(t),
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: "coord.core"},
		store.NetworkChannelPatch{
			FanoutPolicy:      &fanoutPolicy,
			CoordinatorPeerID: &coordinatorPeerID,
			UpdatedAt:         recordedAt.Add(2 * time.Minute),
		},
	); err != nil {
		t.Fatalf("PatchNetworkChannel(policy) error = %v", err)
	}
	entry, err = globalDB.GetNetworkChannel(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
	})
	if err != nil {
		t.Fatalf("GetNetworkChannel(after policy patch) error = %v", err)
	}
	if entry.Purpose != "Pair reviews" ||
		entry.FanoutPolicy != store.NetworkFanoutPolicyCapabilityMatch ||
		entry.CoordinatorPeerID != "" {
		t.Fatalf("entry after policy patch = %#v", entry)
	}
}

func assertGlobalDBPatchNetworkChannelRejectsInvalidFanoutCoupling(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-alpha",
		filepath.Join(t.TempDir(), "ws-alpha"),
	)
	recordedAt := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	if err := globalDB.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		Channel:           "coord.core",
		WorkspaceID:       workspaceID,
		Purpose:           "Original purpose",
		FanoutPolicy:      store.NetworkFanoutPolicyCoordinator,
		CoordinatorPeerID: "reviewer.sess-a",
		CreatedBy:         "codex",
		CreatedAt:         recordedAt,
		UpdatedAt:         recordedAt,
	}); err != nil {
		t.Fatalf("WriteNetworkChannel() error = %v", err)
	}

	coordinatorPeerID := ""
	err := globalDB.PatchNetworkChannel(
		testutil.Context(t),
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: "coord.core"},
		store.NetworkChannelPatch{
			CoordinatorPeerID: &coordinatorPeerID,
			UpdatedAt:         recordedAt.Add(time.Minute),
		},
	)
	if err == nil || !strings.Contains(err.Error(), "coordinator_peer_id is required") {
		t.Fatalf("PatchNetworkChannel(clear coordinator) error = %v, want coordinator peer validation", err)
	}
	entry, err := globalDB.GetNetworkChannel(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
	})
	if err != nil {
		t.Fatalf("GetNetworkChannel(after invalid patch) error = %v", err)
	}
	if entry.FanoutPolicy != store.NetworkFanoutPolicyCoordinator ||
		entry.CoordinatorPeerID != "reviewer.sess-a" {
		t.Fatalf("entry after invalid patch = %#v, want original fanout coupling", entry)
	}
}

func assertGlobalDBGetNetworkChannelNotFound(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	_, err := globalDB.GetNetworkChannel(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: networkStoreTestWorkspaceID,
		Channel:     "missing",
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetNetworkChannel(missing) error = %v, want sql.ErrNoRows", err)
	}
}

func assertGlobalDBDeleteNetworkChannel(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-alpha",
		filepath.Join(t.TempDir(), "ws-alpha"),
	)
	if err := globalDB.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		Channel:     " coord.core ",
		WorkspaceID: workspaceID,
		Purpose:     "Cross-agent coordination",
		CreatedBy:   "codex",
	}); err != nil {
		t.Fatalf("WriteNetworkChannel() error = %v", err)
	}
	if err := globalDB.DeleteNetworkChannel(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
	}); err != nil {
		t.Fatalf("DeleteNetworkChannel() error = %v", err)
	}
	if _, err := globalDB.GetNetworkChannel(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
	}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetNetworkChannel(after delete) error = %v, want sql.ErrNoRows", err)
	}
}

func assertGlobalDBDeleteWorkspaceCascadesNetworkChannels(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-alpha",
		filepath.Join(t.TempDir(), "ws-alpha"),
	)
	if err := globalDB.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		Channel:     "coord.core",
		WorkspaceID: workspaceID,
		Purpose:     "Cross-agent coordination",
		CreatedBy:   "codex",
	}); err != nil {
		t.Fatalf("WriteNetworkChannel() error = %v", err)
	}

	if err := globalDB.DeleteWorkspace(testutil.Context(t), workspaceID); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}
	if _, err := globalDB.GetNetworkChannel(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: workspaceID,
		Channel:     "coord.core",
	}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetNetworkChannel(after workspace delete) error = %v, want sql.ErrNoRows", err)
	}
}

func assertGlobalDBListNetworkChannelsWrapsTimestampParseFailures(t *testing.T) {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"ws-alpha",
		filepath.Join(t.TempDir(), "ws-alpha"),
	)
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO network_channels (
			channel,
			workspace_id,
			purpose,
			created_by,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?)`,
		"coord.core",
		workspaceID,
		"Cross-agent coordination",
		"codex",
		"not-a-timestamp",
		store.FormatTimestamp(time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)),
	); err != nil {
		t.Fatalf("ExecContext(insert invalid network channel) error = %v", err)
	}

	_, err := globalDB.ListNetworkChannels(testutil.Context(t), store.NetworkChannelQuery{
		WorkspaceID: workspaceID,
	})
	if err == nil {
		t.Fatal("ListNetworkChannels(invalid timestamp) error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse network channel created_at") {
		t.Fatalf("ListNetworkChannels(invalid timestamp) error = %v, want wrapped timestamp parse context", err)
	}
}
