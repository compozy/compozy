package globaldb

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/notifications"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBBridgeTaskSubscriptionStore(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate profile-scoped reads and label aggregate owners", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := bridgeTaskSubscriptionTestTime()
		foreignProfileID := "ffffffffffffffffffffffffff"
		if _, err := globalDB.db.ExecContext(ctx, `
			INSERT INTO profiles (id, name, color, emoji, state, created_at)
			VALUES (?, 'subscription-foreign', '#E8572A', '🧪', 'active', ?)`,
			foreignProfileID,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert foreign subscription profile error = %v", err)
		}

		ownedTask := taskRecordForTest("task-subscription-owned")
		foreignTask := taskRecordForTest("task-subscription-foreign")
		foreignTask.ProfileID = foreignProfileID
		ownedInstance := bridgeInstanceForSubscriptionTest("brg-subscription-owned")
		foreignInstance := bridgeInstanceForSubscriptionTest("brg-subscription-foreign")
		foreignInstance.ProfileID = foreignProfileID
		for _, taskRecord := range []taskpkg.Task{ownedTask, foreignTask} {
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask(%s) error = %v", taskRecord.ID, err)
			}
		}
		for _, instance := range []bridges.BridgeInstance{ownedInstance, foreignInstance} {
			if err := globalDB.InsertBridgeInstance(ctx, instance); err != nil {
				t.Fatalf("InsertBridgeInstance(%s) error = %v", instance.ID, err)
			}
		}

		owned := bridgeTaskSubscriptionForGlobalDBTest("sub-profile-owned", ownedTask.ID, ownedInstance.ID)
		foreign := bridgeTaskSubscriptionForGlobalDBTest(
			"sub-profile-foreign",
			foreignTask.ID,
			foreignInstance.ID,
		)
		foreign.ProfileID = foreignProfileID
		for _, subscription := range []bridges.BridgeTaskSubscription{owned, foreign} {
			if err := globalDB.PutBridgeTaskSubscription(ctx, subscription); err != nil {
				t.Fatalf("PutBridgeTaskSubscription(%s) error = %v", subscription.SubscriptionID, err)
			}
		}

		if _, err := globalDB.GetBridgeTaskSubscription(
			ctx,
			store.ReadScope{ProfileID: store.DefaultProfileID},
			foreign.SubscriptionID,
		); !errors.Is(err, bridges.ErrBridgeTaskSubscriptionNotFound) {
			t.Fatalf("GetBridgeTaskSubscription(foreign) error = %v, want not found", err)
		}
		scoped, err := globalDB.ListBridgeTaskSubscriptions(ctx, bridges.BridgeTaskSubscriptionQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
		})
		if err != nil {
			t.Fatalf("ListBridgeTaskSubscriptions(scoped) error = %v", err)
		}
		if len(scoped) != 1 || scoped[0].SubscriptionID != owned.SubscriptionID {
			t.Fatalf("ListBridgeTaskSubscriptions(scoped) = %#v, want owned row", scoped)
		}

		if _, err := globalDB.db.ExecContext(ctx, `
			UPDATE profiles SET state = 'archived', archived_at = ? WHERE id = ?`,
			store.FormatTimestamp(now.Add(time.Minute)),
			foreignProfileID,
		); err != nil {
			t.Fatalf("archive foreign subscription profile error = %v", err)
		}
		aggregate, err := globalDB.ListBridgeTaskSubscriptions(ctx, bridges.BridgeTaskSubscriptionQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil {
			t.Fatalf("ListBridgeTaskSubscriptions(aggregate) error = %v", err)
		}
		if len(aggregate) != 2 {
			t.Fatalf("ListBridgeTaskSubscriptions(aggregate) = %#v, want two rows", aggregate)
		}
		var foreignAggregate bridges.BridgeTaskSubscription
		for _, subscription := range aggregate {
			if subscription.ProfileID == foreignProfileID {
				foreignAggregate = subscription
			}
		}
		if foreignAggregate.ProfileName != "subscription-foreign" ||
			foreignAggregate.ProfileColor != "#E8572A" ||
			foreignAggregate.ProfileEmoji != "🧪" ||
			!foreignAggregate.ProfileArchived {
			t.Fatalf("foreign aggregate owner = %#v, want archived profile labels", foreignAggregate)
		}
	})

	t.Run("Should create, update, list, and delete bridge task subscriptions", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = bridgeTaskSubscriptionTestTime

		taskRecord := taskRecordForTest("task-bridge-subscription")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		instance := bridgeInstanceForSubscriptionTest("brg-task-subscription")
		if err := globalDB.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}

		subscription := bridgeTaskSubscriptionForGlobalDBTest(
			"sub-task-terminal",
			taskRecord.ID,
			instance.ID,
		)
		if err := globalDB.PutBridgeTaskSubscription(ctx, subscription); err != nil {
			t.Fatalf("PutBridgeTaskSubscription() error = %v", err)
		}

		loaded, err := globalDB.GetBridgeTaskSubscription(
			ctx,
			store.ReadScope{AllProfiles: true},
			subscription.SubscriptionID,
		)
		if err != nil {
			t.Fatalf("GetBridgeTaskSubscription() error = %v", err)
		}
		assertBridgeTaskSubscriptionEqual(t, loaded, subscription)

		byTask, err := globalDB.ListBridgeTaskSubscriptions(ctx, bridges.BridgeTaskSubscriptionQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
			TaskID:    taskRecord.ID,
		})
		if err != nil {
			t.Fatalf("ListBridgeTaskSubscriptions(by task) error = %v", err)
		}
		if len(byTask) != 1 || byTask[0].SubscriptionID != subscription.SubscriptionID {
			t.Fatalf("ListBridgeTaskSubscriptions(by task) = %#v, want subscription", byTask)
		}

		byBridge, err := globalDB.ListBridgeTaskSubscriptions(ctx, bridges.BridgeTaskSubscriptionQuery{
			ReadScope:        store.ReadScope{AllProfiles: true},
			BridgeInstanceID: instance.ID,
			Scope:            bridges.ScopeGlobal,
			Limit:            1,
		})
		if err != nil {
			t.Fatalf("ListBridgeTaskSubscriptions(by bridge) error = %v", err)
		}
		if len(byBridge) != 1 || byBridge[0].SubscriptionID != subscription.SubscriptionID {
			t.Fatalf("ListBridgeTaskSubscriptions(by bridge) = %#v, want subscription", byBridge)
		}

		updated := subscription
		updated.PeerID = "peer-updated"
		updated.ThreadID = "thread-updated"
		updated.UpdatedAt = subscription.UpdatedAt.Add(time.Hour)
		if err := globalDB.PutBridgeTaskSubscription(ctx, updated); err != nil {
			t.Fatalf("PutBridgeTaskSubscription(update) error = %v", err)
		}
		loaded, err = globalDB.GetBridgeTaskSubscription(
			ctx,
			store.ReadScope{AllProfiles: true},
			subscription.SubscriptionID,
		)
		if err != nil {
			t.Fatalf("GetBridgeTaskSubscription(updated) error = %v", err)
		}
		assertBridgeTaskSubscriptionEqual(t, loaded, updated)

		if err := globalDB.DeleteBridgeTaskSubscription(ctx, subscription.SubscriptionID); err != nil {
			t.Fatalf("DeleteBridgeTaskSubscription() error = %v", err)
		}
		if _, err := globalDB.GetBridgeTaskSubscription(
			ctx,
			store.ReadScope{AllProfiles: true},
			subscription.SubscriptionID,
		); !errors.Is(
			err,
			bridges.ErrBridgeTaskSubscriptionNotFound,
		) {
			t.Fatalf("GetBridgeTaskSubscription(after delete) error = %v, want ErrBridgeTaskSubscriptionNotFound", err)
		}
	})

	t.Run("Should remove active subscriptions while preserving stale cursor diagnostics", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = bridgeTaskSubscriptionTestTime

		taskRecord := taskRecordForTest("task-bridge-subscription-stale-cursor")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		instance := bridgeInstanceForSubscriptionTest("brg-task-subscription-stale-cursor")
		if err := globalDB.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}

		subscription := bridgeTaskSubscriptionForGlobalDBTest(
			"sub-task-terminal-stale-cursor",
			taskRecord.ID,
			instance.ID,
		)
		if err := globalDB.PutBridgeTaskSubscription(ctx, subscription); err != nil {
			t.Fatalf("PutBridgeTaskSubscription() error = %v", err)
		}
		cursorService := notifications.NewService(globalDB)
		advanced, err := cursorService.Advance(ctx, notifications.AdvanceCursor{
			Key:          subscription.CursorKey(),
			LastSequence: 11,
			DeliveryID:   "nd1_test_stale_delivery_11",
			Now:          bridgeTaskSubscriptionTestTime(),
		})
		if err != nil {
			t.Fatalf("Advance() error = %v", err)
		}
		if advanced.LastSequence != 11 {
			t.Fatalf("advanced cursor = %#v, want sequence 11", advanced)
		}

		if err := globalDB.DeleteBridgeTaskSubscription(ctx, subscription.SubscriptionID); err != nil {
			t.Fatalf("DeleteBridgeTaskSubscription() error = %v", err)
		}
		if _, err := globalDB.GetBridgeTaskSubscription(
			ctx,
			store.ReadScope{AllProfiles: true},
			subscription.SubscriptionID,
		); !errors.Is(
			err,
			bridges.ErrBridgeTaskSubscriptionNotFound,
		) {
			t.Fatalf("GetBridgeTaskSubscription(after delete) error = %v, want ErrBridgeTaskSubscriptionNotFound", err)
		}
		active, err := globalDB.ListBridgeTaskSubscriptions(
			ctx,
			bridges.BridgeTaskSubscriptionQuery{
				ReadScope: store.ReadScope{AllProfiles: true}, TaskID: taskRecord.ID,
			},
		)
		if err != nil {
			t.Fatalf("ListBridgeTaskSubscriptions(after delete) error = %v", err)
		}
		if len(active) != 0 {
			t.Fatalf("ListBridgeTaskSubscriptions(after delete) = %#v, want none", active)
		}

		staleCursor, err := cursorService.Get(ctx, subscription.CursorKey())
		if err != nil {
			t.Fatalf("Get(stale cursor) error = %v", err)
		}
		if staleCursor.LastSequence != 11 || staleCursor.LastDeliveryID != "nd1_test_stale_delivery_11" {
			t.Fatalf("stale cursor = %#v, want preserved diagnostics", staleCursor)
		}

		if err := globalDB.PutBridgeTaskSubscription(ctx, subscription); err != nil {
			t.Fatalf("PutBridgeTaskSubscription(recreate) error = %v", err)
		}
		recreatedCursor, err := cursorService.Get(ctx, subscription.CursorKey())
		if err != nil {
			t.Fatalf("Get(recreated cursor) error = %v", err)
		}
		if recreatedCursor.LastSequence != 11 {
			t.Fatalf("recreated cursor = %#v, want stale sequence preserved", recreatedCursor)
		}
	})

	t.Run("Should retain whitespace-distinct opaque subscription identities", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = bridgeTaskSubscriptionTestTime

		taskRecord := taskRecordForTest("task-opaque")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		instance := bridgeInstanceForSubscriptionTest(" brg-opaque ")
		if err := globalDB.InsertBridgeInstance(ctx, instance); err != nil {
			t.Fatalf("InsertBridgeInstance() error = %v", err)
		}
		subscription := bridgeTaskSubscriptionForGlobalDBTest(
			" sub-opaque ",
			taskRecord.ID,
			instance.ID,
		)
		subscription.PeerID = " peer-opaque "
		subscription.ThreadID = " thread-opaque "
		subscription.GroupID = " group-opaque "
		subscription.CreatedBy.Ref = " actor-opaque "

		if err := globalDB.PutBridgeTaskSubscription(ctx, subscription); err != nil {
			t.Fatalf("PutBridgeTaskSubscription() error = %v", err)
		}
		loaded, err := globalDB.GetBridgeTaskSubscription(
			ctx,
			store.ReadScope{AllProfiles: true},
			subscription.SubscriptionID,
		)
		if err != nil {
			t.Fatalf("GetBridgeTaskSubscription() error = %v", err)
		}
		assertBridgeTaskSubscriptionEqual(t, loaded, subscription)

		listed, err := globalDB.ListBridgeTaskSubscriptions(ctx, bridges.BridgeTaskSubscriptionQuery{
			ReadScope:        store.ReadScope{AllProfiles: true},
			TaskID:           taskRecord.ID,
			BridgeInstanceID: instance.ID,
			Scope:            bridges.ScopeGlobal,
		})
		if err != nil {
			t.Fatalf("ListBridgeTaskSubscriptions() error = %v", err)
		}
		if len(listed) != 1 {
			t.Fatalf("ListBridgeTaskSubscriptions() returned %d subscriptions, want 1", len(listed))
		}
		assertBridgeTaskSubscriptionEqual(t, listed[0], subscription)
	})
}

func bridgeInstanceForSubscriptionTest(id string) bridges.BridgeInstance {
	now := bridgeTaskSubscriptionTestTime()
	return bridges.BridgeInstance{
		ID:            id,
		ProfileID:     store.DefaultProfileID,
		Scope:         bridges.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "telegram-extension",
		DisplayName:   "Telegram",
		Source:        bridges.BridgeInstanceSourceDynamic,
		Enabled:       true,
		Status:        bridges.BridgeStatusReady,
		DMPolicy:      bridges.BridgeDMPolicyOpen,
		RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func bridgeTaskSubscriptionForGlobalDBTest(
	subscriptionID string,
	taskID string,
	bridgeInstanceID string,
) bridges.BridgeTaskSubscription {
	now := bridgeTaskSubscriptionTestTime()
	return bridges.BridgeTaskSubscription{
		SubscriptionID:   subscriptionID,
		ProfileID:        store.DefaultProfileID,
		ProfileName:      "default",
		ProfileColor:     "#8E8EB5",
		ProfileIcon:      "circle",
		TaskID:           taskID,
		BridgeInstanceID: bridgeInstanceID,
		Scope:            bridges.ScopeGlobal,
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		DeliveryMode:     bridges.DeliveryModeReply,
		CreatedBy: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindDaemon,
			Ref:  "task-terminal-notifier",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func bridgeTaskSubscriptionTestTime() time.Time {
	return time.Date(2026, 5, 5, 18, 0, 0, 0, time.UTC)
}

func assertBridgeTaskSubscriptionEqual(
	t *testing.T,
	got bridges.BridgeTaskSubscription,
	want bridges.BridgeTaskSubscription,
) {
	t.Helper()

	got = got.Normalize()
	want = want.Normalize()
	if got.SubscriptionID != want.SubscriptionID ||
		got.ProfileID != want.ProfileID ||
		got.ProfileName != want.ProfileName ||
		got.ProfileColor != want.ProfileColor ||
		got.ProfileIcon != want.ProfileIcon ||
		got.ProfileEmoji != want.ProfileEmoji ||
		got.ProfileArchived != want.ProfileArchived ||
		got.TaskID != want.TaskID ||
		got.BridgeInstanceID != want.BridgeInstanceID ||
		got.Scope != want.Scope ||
		got.WorkspaceID != want.WorkspaceID ||
		got.PeerID != want.PeerID ||
		got.ThreadID != want.ThreadID ||
		got.GroupID != want.GroupID ||
		got.DeliveryMode != want.DeliveryMode ||
		got.CreatedBy != want.CreatedBy ||
		!got.CreatedAt.Equal(want.CreatedAt) ||
		!got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("bridge task subscription = %#v, want %#v", got, want)
	}
}
