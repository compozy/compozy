package globaldb

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBNotificationCursorStore(t *testing.T) {
	t.Parallel()

	t.Run("Should advance and read a cursor", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		key := notificationCursorTestKey()
		now := notificationCursorTestTime()

		cursor, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:             key,
			LastSequence:    7,
			DeliveryID:      "delivery-7",
			LastDeliveredAt: now.Add(-time.Minute),
			Now:             now,
		})
		if err != nil {
			t.Fatalf("Advance() error = %v", err)
		}
		if cursor.LastSequence != 7 || cursor.LastDeliveryID != "delivery-7" {
			t.Fatalf("cursor = %#v, want sequence 7 delivery-7", cursor)
		}

		stored, err := service.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if stored.LastSequence != cursor.LastSequence || stored.LastDeliveryID != cursor.LastDeliveryID {
			t.Fatalf("stored cursor = %#v, want %#v", stored, cursor)
		}
	})

	t.Run("Should persist identical opaque components independently by scope", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		now := notificationCursorTestTime()
		globalKey := notifications.CursorKey{
			Scope:      notifications.ScopeRef{Kind: notifications.ScopeKindGlobal},
			ConsumerID: "consumer:terminal",
			StreamName: "task:events",
			SubjectID:  "subject:terminal",
		}
		workspaceKey := globalKey
		workspaceKey.Scope = notifications.ScopeRef{
			Kind:        notifications.ScopeKindWorkspace,
			WorkspaceID: "global",
		}

		for _, advance := range []notifications.AdvanceCursor{
			{Key: globalKey, LastSequence: 7, DeliveryID: "delivery-global", Now: now},
			{Key: workspaceKey, LastSequence: 11, DeliveryID: "delivery-workspace", Now: now},
		} {
			if _, err := service.Advance(ctx, advance); err != nil {
				t.Fatalf("Advance(%#v) error = %v", advance.Key.Scope, err)
			}
		}
		if _, err := service.Reset(ctx, notifications.ResetCursor{
			Key:          workspaceKey,
			LastSequence: 1,
			Reason:       "workspace replay",
			Now:          now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("Reset(workspace) error = %v", err)
		}

		globalCursor, err := service.Get(ctx, globalKey)
		if err != nil {
			t.Fatalf("Get(global) error = %v", err)
		}
		workspaceCursor, err := service.Get(ctx, workspaceKey)
		if err != nil {
			t.Fatalf("Get(workspace) error = %v", err)
		}
		if globalCursor.LastSequence != 7 || globalCursor.LastDeliveryID != "delivery-global" {
			t.Fatalf("global cursor = %#v, want unchanged global delivery progress", globalCursor)
		}
		if workspaceCursor.LastSequence != 1 || workspaceCursor.LastDeliveryID != "" {
			t.Fatalf("workspace cursor = %#v, want independently reset workspace progress", workspaceCursor)
		}
		for _, query := range []notifications.CursorQuery{
			{Scope: globalKey.Scope},
			{Scope: workspaceKey.Scope},
			{},
		} {
			cursors, err := service.List(ctx, query)
			if err != nil {
				t.Fatalf("List(%#v) error = %v", query.Scope, err)
			}
			want := 1
			if query.Scope.Kind == "" {
				want = 2
			}
			if len(cursors) != want {
				t.Fatalf("List(%#v) returned %d cursors, want %d", query.Scope, len(cursors), want)
			}
		}
	})

	t.Run("Should retain whitespace-only opaque cursor components", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		key := notifications.CursorKey{
			Scope: notifications.ScopeRef{
				Kind:        notifications.ScopeKindWorkspace,
				WorkspaceID: " ",
			},
			ConsumerID: " ",
			StreamName: " ",
			SubjectID:  " ",
		}
		wantDeliveryID := " "

		if _, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:          key,
			LastSequence: 1,
			DeliveryID:   wantDeliveryID,
			Now:          notificationCursorTestTime(),
		}); err != nil {
			t.Fatalf("Advance() error = %v", err)
		}
		stored, err := service.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got, want := stored.Key, key; got != want {
			t.Fatalf("stored cursor key = %#v, want %#v", got, want)
		}
		if got, want := stored.LastDeliveryID, wantDeliveryID; got != want {
			t.Fatalf("stored delivery ID = %q, want exact opaque ID %q", got, want)
		}
		listed, err := service.List(ctx, notifications.CursorQuery{
			Scope:      key.Scope,
			ConsumerID: key.ConsumerID,
			StreamName: key.StreamName,
			SubjectID:  key.SubjectID,
		})
		if err != nil {
			t.Fatalf("List(opaque key) error = %v", err)
		}
		if len(listed) != 1 || listed[0].Key != key {
			t.Fatalf("List(opaque key) = %#v, want exact stored key", listed)
		}
	})

	t.Run("Should discard ambiguous legacy cursor progress during scoped migration", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		legacyDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00039_schema.sql"),
		)
		if err != nil {
			t.Fatalf("openGlobalMigrationPrefixDatabase() error = %v", err)
		}
		ctx := testutil.Context(t)
		if _, err := legacyDB.ExecContext(
			ctx,
			`INSERT INTO notification_cursors (
				consumer_id, stream_name, subject_id, last_sequence, last_delivery_id,
				last_delivered_at, last_error, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"preset:terminal:#ops:incident",
			"task.run_completed",
			"global:evt:terminal",
			7,
			"notif:sub-1:7",
			store.FormatTimestamp(notificationCursorTestTime()),
			"",
			store.FormatTimestamp(notificationCursorTestTime()),
		); err != nil {
			t.Fatalf("seed legacy notification cursor error = %v", err)
		}
		if err := legacyDB.Close(); err != nil {
			t.Fatalf("legacyDB.Close() error = %v", err)
		}

		globalDB, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("openGlobalMigrationUpgrade() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := globalDB.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("GlobalDB.Close() error = %v", closeErr)
			}
		})
		var count int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_cursors`).
			Scan(&count); err != nil {
			t.Fatalf("count scoped notification cursors error = %v", err)
		}
		if count != 0 {
			t.Fatalf("migrated notification cursor count = %d, want hard-dropped legacy progress", count)
		}
	})

	t.Run("Should refresh idempotent replay diagnostics and updated timestamp", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		key := notificationCursorTestKey()
		firstTime := notificationCursorTestTime()

		first, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:          key,
			LastSequence: 11,
			DeliveryID:   "delivery-11",
			Now:          firstTime,
		})
		if err != nil {
			t.Fatalf("Advance(first) error = %v", err)
		}
		if _, err := service.RecordError(ctx, notifications.CursorError{
			Key:       key,
			LastError: "bridge delivery failed",
			Now:       firstTime.Add(30 * time.Minute),
		}); err != nil {
			t.Fatalf("RecordError() error = %v", err)
		}
		second, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:          key,
			LastSequence: 11,
			DeliveryID:   "delivery-11",
			Now:          firstTime.Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("Advance(replay) error = %v", err)
		}

		if got, want := second.LastError, ""; got != want {
			t.Fatalf("idempotent replay LastError = %q, want empty", got)
		}
		if !second.UpdatedAt.Equal(firstTime.Add(time.Hour).UTC()) {
			t.Fatalf("idempotent replay UpdatedAt = %s, want %s", second.UpdatedAt, firstTime.Add(time.Hour).UTC())
		}
		if !second.LastDeliveredAt.Equal(first.LastDeliveredAt) {
			t.Fatalf("idempotent replay LastDeliveredAt = %s, want %s", second.LastDeliveredAt, first.LastDeliveredAt)
		}
	})

	t.Run("Should reject non monotonic advances", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		key := notificationCursorTestKey()
		now := notificationCursorTestTime()

		if _, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:          key,
			LastSequence: 15,
			DeliveryID:   "delivery-15",
			Now:          now,
		}); err != nil {
			t.Fatalf("Advance(seed) error = %v", err)
		}
		if _, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:          key,
			LastSequence: 15,
			DeliveryID:   "delivery-15b",
			Now:          now.Add(time.Minute),
		}); !errors.Is(err, notifications.ErrNonMonotonicCursor) {
			t.Fatalf("Advance(forked replay) error = %v, want ErrNonMonotonicCursor", err)
		}
		if _, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:          key,
			LastSequence: 14,
			DeliveryID:   "delivery-14",
			Now:          now.Add(2 * time.Minute),
		}); !errors.Is(err, notifications.ErrNonMonotonicCursor) {
			t.Fatalf("Advance(backward) error = %v, want ErrNonMonotonicCursor", err)
		}
	})

	t.Run("Should reset a cursor only with an explicit reason", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		key := notificationCursorTestKey()
		now := notificationCursorTestTime()

		if _, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:          key,
			LastSequence: 21,
			DeliveryID:   "delivery-21",
			Now:          now,
		}); err != nil {
			t.Fatalf("Advance(seed) error = %v", err)
		}
		if _, err := service.Reset(ctx, notifications.ResetCursor{
			Key:          key,
			LastSequence: 2,
			Reason:       "operator replay after bridge repair",
			Now:          now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("Reset() error = %v", err)
		}

		cursor, err := service.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get(after reset) error = %v", err)
		}
		if cursor.LastSequence != 2 || cursor.LastDeliveryID != "" || !cursor.LastDeliveredAt.IsZero() {
			t.Fatalf("cursor after reset = %#v, want sequence 2 with cleared delivery metadata", cursor)
		}
		if _, err := service.Reset(ctx, notifications.ResetCursor{
			Key:          key,
			LastSequence: 0,
			Now:          now.Add(2 * time.Minute),
		}); !errors.Is(err, notifications.ErrResetReasonRequired) {
			t.Fatalf("Reset(without reason) error = %v, want ErrResetReasonRequired", err)
		}
	})

	t.Run("Should record errors without advancing delivery progress", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		key := notificationCursorTestKey()
		now := notificationCursorTestTime()

		if _, err := service.Advance(ctx, notifications.AdvanceCursor{
			Key:          key,
			LastSequence: 31,
			DeliveryID:   "delivery-31",
			Now:          now,
		}); err != nil {
			t.Fatalf("Advance(seed) error = %v", err)
		}
		cursor, err := service.RecordError(ctx, notifications.CursorError{
			Key:       key,
			LastError: "bridge delivery failed",
			Now:       now.Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("RecordError() error = %v", err)
		}
		if cursor.LastSequence != 31 || cursor.LastDeliveryID != "delivery-31" {
			t.Fatalf("cursor after RecordError = %#v, want original delivery progress", cursor)
		}
		if cursor.LastError != "bridge delivery failed" {
			t.Fatalf("cursor.LastError = %q, want diagnostic", cursor.LastError)
		}
	})

	t.Run("Should create a cursor when recording the first error", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		key := notificationCursorTestKey()
		now := notificationCursorTestTime()

		cursor, err := service.RecordError(ctx, notifications.CursorError{
			Key:       key,
			LastError: "bridge delivery failed",
			Now:       now,
		})
		if err != nil {
			t.Fatalf("RecordError() error = %v", err)
		}
		if cursor.LastSequence != 0 || cursor.LastDeliveryID != "" {
			t.Fatalf("cursor after first RecordError = %#v, want zero delivery progress", cursor)
		}
		if cursor.LastError != "bridge delivery failed" {
			t.Fatalf("cursor.LastError = %q, want diagnostic", cursor.LastError)
		}
		if !cursor.UpdatedAt.Equal(now.UTC()) {
			t.Fatalf("cursor.UpdatedAt = %s, want %s", cursor.UpdatedAt, now.UTC())
		}
	})

	t.Run("Should list cursors with stable filters", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		service := notifications.NewService(globalDB)
		now := notificationCursorTestTime()
		inputs := []notifications.AdvanceCursor{
			{
				Key: notifications.CursorKey{
					Scope:      notifications.ScopeRef{Kind: notifications.ScopeKindGlobal},
					ConsumerID: "consumer-a",
					StreamName: "task_events",
					SubjectID:  "task-a",
				},
				LastSequence: 1,
				DeliveryID:   "delivery-a",
				Now:          now,
			},
			{
				Key: notifications.CursorKey{
					Scope:      notifications.ScopeRef{Kind: notifications.ScopeKindWorkspace, WorkspaceID: "ws-list"},
					ConsumerID: "consumer-b",
					StreamName: "task_events",
					SubjectID:  "task-b",
				},
				LastSequence: 2,
				DeliveryID:   "delivery-b",
				Now:          now,
			},
		}
		for _, input := range inputs {
			if _, err := service.Advance(ctx, input); err != nil {
				t.Fatalf("Advance(%q) error = %v", input.Key.ConsumerID, err)
			}
		}

		cursors, err := service.List(ctx, notifications.CursorQuery{StreamName: "task_events", Limit: 1})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(cursors) != 1 || cursors[0].Key.ConsumerID != "consumer-a" {
			t.Fatalf("List() = %#v, want first stable task_events cursor", cursors)
		}
	})
}

func notificationCursorTestKey() notifications.CursorKey {
	return notifications.CursorKey{
		Scope:      notifications.ScopeRef{Kind: notifications.ScopeKindGlobal},
		ConsumerID: "sub-1",
		StreamName: "task_events",
		SubjectID:  "task-1",
	}
}

func notificationCursorTestTime() time.Time {
	return time.Date(2026, 5, 5, 15, 0, 0, 0, time.UTC)
}
