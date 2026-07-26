package globaldb

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/notifications"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBBridgeTaskSubscriptionStore(t *testing.T) {
	t.Parallel()

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

		loaded, err := globalDB.GetBridgeTaskSubscription(ctx, subscription.SubscriptionID)
		if err != nil {
			t.Fatalf("GetBridgeTaskSubscription() error = %v", err)
		}
		assertBridgeTaskSubscriptionEqual(t, loaded, subscription)

		byTask, err := globalDB.ListBridgeTaskSubscriptions(ctx, bridges.BridgeTaskSubscriptionQuery{
			TaskID: taskRecord.ID,
		})
		if err != nil {
			t.Fatalf("ListBridgeTaskSubscriptions(by task) error = %v", err)
		}
		if len(byTask) != 1 || byTask[0].SubscriptionID != subscription.SubscriptionID {
			t.Fatalf("ListBridgeTaskSubscriptions(by task) = %#v, want subscription", byTask)
		}

		byBridge, err := globalDB.ListBridgeTaskSubscriptions(ctx, bridges.BridgeTaskSubscriptionQuery{
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
		loaded, err = globalDB.GetBridgeTaskSubscription(ctx, subscription.SubscriptionID)
		if err != nil {
			t.Fatalf("GetBridgeTaskSubscription(updated) error = %v", err)
		}
		assertBridgeTaskSubscriptionEqual(t, loaded, updated)

		if err := globalDB.DeleteBridgeTaskSubscription(ctx, subscription.SubscriptionID); err != nil {
			t.Fatalf("DeleteBridgeTaskSubscription() error = %v", err)
		}
		if _, err := globalDB.GetBridgeTaskSubscription(ctx, subscription.SubscriptionID); !errors.Is(
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
			DeliveryID:   "notif:sub-task-terminal-stale-cursor:11",
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
		if _, err := globalDB.GetBridgeTaskSubscription(ctx, subscription.SubscriptionID); !errors.Is(
			err,
			bridges.ErrBridgeTaskSubscriptionNotFound,
		) {
			t.Fatalf("GetBridgeTaskSubscription(after delete) error = %v, want ErrBridgeTaskSubscriptionNotFound", err)
		}
		active, err := globalDB.ListBridgeTaskSubscriptions(
			ctx,
			bridges.BridgeTaskSubscriptionQuery{TaskID: taskRecord.ID},
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
		if staleCursor.LastSequence != 11 || staleCursor.LastDeliveryID != "notif:sub-task-terminal-stale-cursor:11" {
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
}

func bridgeInstanceForSubscriptionTest(id string) bridges.BridgeInstance {
	now := bridgeTaskSubscriptionTestTime()
	return bridges.BridgeInstance{
		ID:            id,
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
