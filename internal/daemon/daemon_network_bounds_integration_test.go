//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

func TestDaemonE2ENetworkLiveBoundsCoalesceDepthAndBudget(t *testing.T) {
	t.Run("Should preserve every message while bounding coalesced and exhausted wakes", func(t *testing.T) {
		acpmock.RequireDriver(t)

		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			EnableNetwork: true,
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *aghconfig.Config) {
				cfg.Network.Live.Defaults.MaxWakes = 2
				cfg.Network.Live.Defaults.MaxWakeDepth = 2
				cfg.Network.Live.Defaults.CoalesceWindow = "5s"
			}},
			MockAgents: []e2etest.MockAgentSpec{
				{
					FixturePath:  mockFixturePath(t, "network_bounds_fixture.json"),
					FixtureAgent: "bounds-alpha",
					AgentName:    "bounds-alpha",
				},
				{
					FixturePath:  mockFixturePath(t, "network_bounds_fixture.json"),
					FixtureAgent: "bounds-beta",
					AgentName:    "bounds-beta",
				},
			},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		db, err := globaldb.OpenGlobalDB(ctx, harness.HomePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB(Network bounds) error = %v", err)
		}
		defer func() {
			if closeErr := db.Close(context.Background()); closeErr != nil {
				t.Fatalf("Close(Network bounds DB) error = %v", closeErr)
			}
		}()

		channel := "bounded"
		detail := mustCreateNetworkChannel(t, ctx, harness, channel, "bounds-alpha", "bounds-beta")
		alpha := requireChannelSession(t, detail, "bounds-alpha")
		beta := requireChannelSession(t, detail, "bounds-beta")
		peers := waitForChannelPeerCount(t, ctx, harness, channel, 2)
		alphaPeer := requirePeerIDForSession(t, peers, alpha.ID)
		betaPeer := requirePeerIDForSession(t, peers, beta.ID)
		directID := requireDirectResolveRace(
			t,
			ctx,
			harness,
			channel,
			alpha.ID,
			alphaPeer,
			beta.ID,
			betaPeer,
		)

		directRootID := "bounds_direct_root"
		mustSendBoundsMessage(t, ctx, harness, aghcontract.NetworkSendRequest{
			WorkspaceID: harness.WorkspaceID,
			SessionID:   alpha.ID,
			Channel:     channel,
			Surface:     "direct",
			DirectID:    directID,
			Kind:        "say",
			To:          betaPeer,
			ID:          directRootID,
			Body:        boundsSayBody("initial direct wake"),
		})
		waitForSettledNetworkWakeCount(t, ctx, harness, channel, 1)

		mentionID := "bounds_mention_root"
		mustSendBoundsMessage(t, ctx, harness, aghcontract.NetworkSendRequest{
			WorkspaceID: harness.WorkspaceID,
			SessionID:   beta.ID,
			Channel:     channel,
			Surface:     "thread",
			ThreadID:    "thread_bounds",
			Kind:        "say",
			Mentions:    []string{alphaPeer},
			ID:          mentionID,
			Body:        boundsSayBody("hold the runner while the burst commits"),
		})
		alphaOwner := networkSessionOwnerKey(alpha.ID)
		waitForRuntimeCondition(t, "alpha Network wake claimed", 10*time.Second, func() bool {
			return networkWakeTaskStatus(ctx, db, alphaOwner) == "claimed"
		})

		burstIDs := sendBoundsBurst(
			t,
			ctx,
			harness,
			channel,
			directID,
			alpha.ID,
			betaPeer,
			directRootID,
			10,
		)
		waitForSettledNetworkWakeCount(t, ctx, harness, channel, 3)

		depthReplyID := "bounds_depth_reply"
		mustSendBoundsMessage(t, ctx, harness, aghcontract.NetworkSendRequest{
			WorkspaceID: harness.WorkspaceID,
			SessionID:   beta.ID,
			Channel:     channel,
			Surface:     "direct",
			DirectID:    directID,
			Kind:        "say",
			To:          alphaPeer,
			ReplyTo:     burstIDs[0],
			CausationID: burstIDs[0],
			ID:          depthReplyID,
			Body:        boundsSayBody("depth-bounded reply remains durable"),
		})

		exhaustedID := "bounds_budget_exhausted"
		mustSendBoundsMessage(t, ctx, harness, aghcontract.NetworkSendRequest{
			WorkspaceID: harness.WorkspaceID,
			SessionID:   alpha.ID,
			Channel:     channel,
			Surface:     "direct",
			DirectID:    directID,
			Kind:        "say",
			To:          betaPeer,
			ID:          exhaustedID,
			Body:        boundsSayBody("budget exhausted but still visible"),
		})

		betaOwner := networkSessionOwnerKey(beta.ID)
		waitForRuntimeCondition(t, "visible max_wakes exhaustion", 10*time.Second, func() bool {
			return networkBudgetExhaustedReason(ctx, db, betaOwner) == "max_wakes" &&
				sessionInboxHasMessageID(ctx, harness, alpha.ID, depthReplyID) &&
				sessionInboxHasMessageID(ctx, harness, beta.ID, exhaustedID)
		})
		assertBoundsWakeAccounting(t, ctx, db, harness.WorkspaceID, channel, alphaOwner, betaOwner)
		assertBoundsTranscriptComplete(
			t,
			ctx,
			harness,
			channel,
			directID,
			alpha.ID,
			beta.ID,
			directRootID,
			mentionID,
			burstIDs,
			depthReplyID,
			exhaustedID,
		)
	})
}

func boundsSayBody(text string) json.RawMessage {
	return json.RawMessage(
		`{"text":` + strconv.Quote(text) + `,"intent":"bounds-probe","artifacts":[]}`,
	)
}

func mustSendBoundsMessage(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	request aghcontract.NetworkSendRequest,
) {
	t.Helper()
	message, err := harness.NetworkSend(ctx, request)
	if err != nil {
		t.Fatalf("NetworkSend(%q) error = %v", request.ID, err)
	}
	if message.ID != request.ID {
		t.Fatalf("NetworkSend(%q) response id = %q, want request id", request.ID, message.ID)
	}
}

func sendBoundsBurst(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	channel string,
	directID string,
	senderSessionID string,
	targetPeerID string,
	rootID string,
	count int,
) []string {
	t.Helper()
	type result struct {
		id  string
		err error
	}
	results := make(chan result, count)
	start := make(chan struct{})
	ids := make([]string, count)
	var workers sync.WaitGroup
	workers.Add(count)
	for index := range count {
		id := fmt.Sprintf("bounds_burst_%02d", index)
		ids[index] = id
		go func() {
			defer workers.Done()
			<-start
			message, err := harness.NetworkSend(ctx, aghcontract.NetworkSendRequest{
				WorkspaceID: harness.WorkspaceID,
				SessionID:   senderSessionID,
				Channel:     channel,
				Surface:     "direct",
				DirectID:    directID,
				Kind:        "say",
				To:          targetPeerID,
				ReplyTo:     rootID,
				CausationID: rootID,
				ID:          id,
				Body:        boundsSayBody("coalesced burst " + id),
			})
			if err == nil && message.ID != id {
				err = fmt.Errorf("response id %q does not match request", message.ID)
			}
			results <- result{id: id, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	for item := range results {
		if item.err != nil {
			t.Fatalf("NetworkSend(%q) error = %v", item.id, item.err)
		}
	}
	return ids
}

func networkSessionOwnerKey(sessionID string) string {
	return participation.OwnerKey(participation.OwnerRef{
		Kind: participation.OwnerKindSession,
		ID:   sessionID,
	})
}

func networkWakeTaskStatus(ctx context.Context, db *globaldb.GlobalDB, ownerKey string) string {
	var status string
	err := db.DB().QueryRowContext(
		ctx,
		`SELECT task_runs.status
		 FROM network_live_wakes
		 JOIN task_runs ON task_runs.id = network_live_wakes.task_run_id
		 WHERE network_live_wakes.owner_key = ? AND network_live_wakes.state = 'open'
		 ORDER BY network_live_wakes.reserved_at DESC LIMIT 1`,
		ownerKey,
	).Scan(&status)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(status)
}

func networkBudgetExhaustedReason(ctx context.Context, db *globaldb.GlobalDB, ownerKey string) string {
	var reason string
	if err := db.DB().QueryRowContext(
		ctx,
		`SELECT exhausted_reason FROM network_participation_budgets WHERE owner_key = ?`,
		ownerKey,
	).Scan(&reason); err != nil {
		return ""
	}
	return strings.TrimSpace(reason)
}

func assertBoundsWakeAccounting(
	t testing.TB,
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	channel string,
	alphaOwner string,
	betaOwner string,
) {
	t.Helper()
	usage, err := db.GetNetworkUsage(ctx, store.NetworkUsageQuery{WorkspaceID: workspaceID, Channel: channel})
	if err != nil {
		t.Fatalf("GetNetworkUsage(bounds) error = %v", err)
	}
	if usage.Total.ActualWakeCount != 3 || usage.Total.WakeCount != 3 ||
		usage.Total.InputTokens != 31 || usage.Total.OutputTokens != 9 {
		t.Fatalf("bounds usage = %#v, want three actual wakes with 31/9 tokens", usage)
	}
	assertOwnerWakeCounts(t, ctx, db, alphaOwner, 1, 1)
	assertOwnerWakeCounts(t, ctx, db, betaOwner, 2, 11)
}

func assertOwnerWakeCounts(
	t testing.TB,
	ctx context.Context,
	db *globaldb.GlobalDB,
	ownerKey string,
	wantWakes int,
	wantSources int,
) {
	t.Helper()
	var wakes int
	var sources int
	if err := db.DB().QueryRowContext(
		ctx,
		`SELECT
			(SELECT COUNT(*) FROM network_live_wakes WHERE owner_key = ?),
			(SELECT COUNT(*) FROM network_wake_sources WHERE owner_key = ?)`,
		ownerKey,
		ownerKey,
	).Scan(&wakes, &sources); err != nil {
		t.Fatalf("query owner wake counts for %q error = %v", ownerKey, err)
	}
	if wakes != wantWakes || sources != wantSources {
		t.Fatalf(
			"owner %q wake/source counts = %d/%d, want %d/%d",
			ownerKey,
			wakes,
			sources,
			wantWakes,
			wantSources,
		)
	}
}

func assertBoundsTranscriptComplete(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	channel string,
	directID string,
	alphaSessionID string,
	betaSessionID string,
	directRootID string,
	mentionID string,
	burstIDs []string,
	depthReplyID string,
	exhaustedID string,
) {
	t.Helper()
	alphaMessages := []string{mentionID}
	betaMessages := append([]string{directRootID}, burstIDs...)
	for _, messageID := range alphaMessages {
		if !sessionTranscriptHasNeedle(ctx, harness, alphaSessionID, messageID) {
			t.Fatalf("alpha ACP transcript missing admitted message %q", messageID)
		}
	}
	for _, messageID := range betaMessages {
		if !sessionTranscriptHasNeedle(ctx, harness, betaSessionID, messageID) {
			t.Fatalf("beta ACP transcript missing admitted message %q", messageID)
		}
	}

	threadMessages, err := harness.NetworkThreadMessages(ctx, channel, "thread_bounds")
	if err != nil {
		t.Fatalf("NetworkThreadMessages(bounds) error = %v", err)
	}
	assertNetworkTimelineMessageIDs(t, "thread", threadMessages, []string{mentionID})

	directMessages, err := harness.NetworkDirectRoomMessages(ctx, channel, directID)
	if err != nil {
		t.Fatalf("NetworkDirectRoomMessages(bounds) error = %v", err)
	}
	wantDirectIDs := append([]string{directRootID}, burstIDs...)
	wantDirectIDs = append(wantDirectIDs, depthReplyID, exhaustedID)
	assertNetworkTimelineMessageIDs(t, "direct", directMessages, wantDirectIDs)
}

func assertNetworkTimelineMessageIDs(
	t testing.TB,
	timeline string,
	messages []aghcontract.NetworkConversationMessagePayload,
	wantIDs []string,
) {
	t.Helper()
	present := make(map[string]struct{}, len(messages))
	for _, message := range messages {
		present[message.MessageID] = struct{}{}
	}
	for _, messageID := range wantIDs {
		if _, ok := present[messageID]; !ok {
			t.Fatalf("%s Network timeline missing durable message %q", timeline, messageID)
		}
	}
}
