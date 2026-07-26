package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func TestDiscordProgressFullTurnDelivery(t *testing.T) {
	t.Run("Should keep progress separate from the textual ack through the final answer", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 45, 0, 0, time.UTC)
		timer := &discordManualProgressTimer{}
		api := &discordAPIFake{postedMessageIDs: []string{"answer-1", "progress-1"}}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.now = func() time.Time { return now }
		provider.progressDispatcherOptions = []bridgesdk.ProgressDispatcherOption{
			bridgesdk.WithProgressSchedule(timer.Schedule),
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-discord": {
				instanceID: "brg-discord",
				managed:    testDiscordManagedInstance("brg-discord"),
			},
		}, nil)
		session := &bridgesdk.Session{}

		start := testDiscordTextRequest(
			"delivery-full-turn",
			1,
			bridgepkg.DeliveryEventTypeStart,
			"Working…",
			false,
			"",
		)
		startAck, err := provider.handleBridgesDeliver(context.Background(), session, start)
		if err != nil {
			t.Fatalf("handleBridgesDeliver(start) error = %v", err)
		}
		if got, want := startAck.RemoteMessageID, "thread-1:answer-1"; got != want {
			t.Fatalf("start ack remote id = %q, want %q", got, want)
		}

		for index, label := range []string{"Search", "Read", "Summarize"} {
			request := testDiscordProgressRequest(
				"delivery-full-turn",
				int64(index+2),
				bridgepkg.ToolProgress{
					ToolCallID: "tool-call-" + strings.ToLower(label),
					ToolID:     "agh__" + strings.ToLower(label),
					Phase:      bridgepkg.ToolProgressPhaseStarted,
					Label:      label,
					Emoji:      "🔎",
					Index:      index + 1,
				},
			)
			if _, err := provider.handleBridgesProgress(context.Background(), session, request); err != nil {
				t.Fatalf("handleBridgesProgress(%s) error = %v", label, err)
			}
			if index > 0 {
				now = now.Add(1500 * time.Millisecond)
				timer.Run(t)
			}
			now = now.Add(500 * time.Millisecond)
		}

		final := testDiscordTextRequest(
			"delivery-full-turn",
			5,
			bridgepkg.DeliveryEventTypeFinal,
			"Final answer",
			true,
			startAck.RemoteMessageID,
		)
		finalAck, err := provider.handleBridgesDeliver(context.Background(), session, final)
		if err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v", err)
		}
		if got, want := finalAck.RemoteMessageID, startAck.RemoteMessageID; got != want {
			t.Fatalf("final ack remote id = %q, want textual id %q", got, want)
		}
		if got, want := finalAck.ReplaceRemoteMessageID, startAck.RemoteMessageID; got != want {
			t.Fatalf("final replacement id = %q, want %q", got, want)
		}
		if got, want := len(api.updateRequests), 3; got != want {
			t.Fatalf("update requests = %d, want two progress edits plus final text", got)
		}
		if got, want := api.updateRequests[0].MessageID, "progress-1"; got != want {
			t.Fatalf("first progress edit message id = %q, want %q", got, want)
		}
		if got, want := api.updateRequests[1].MessageID, "progress-1"; got != want {
			t.Fatalf("second progress edit message id = %q, want %q", got, want)
		}
		if got, want := api.updateRequests[2], (discordUpdateMessageRequest{
			ChannelID: "thread-1",
			MessageID: "answer-1",
			Content:   "Final answer",
		}); got != want {
			t.Fatalf("final text update = %#v, want %#v", got, want)
		}
		if len(api.typingRequests) == 0 || !api.typingRequests[0].Active {
			t.Fatalf("typing requests = %#v, want active start", api.typingRequests)
		}
		if api.typingRequests[len(api.typingRequests)-1].Active {
			t.Fatalf("typing requests = %#v, want inactive terminal cleanup", api.typingRequests)
		}
		for _, reaction := range api.reactionRequests {
			if got, want := reaction.MessageID, "progress-1"; got != want {
				t.Fatalf("reaction message id = %q, want progress bubble %q", got, want)
			}
		}
		state := provider.deliveryState("brg-discord", "delivery-full-turn")
		if state.Progress != nil {
			t.Fatal("terminal text retained progress dispatcher")
		}
		if got, want := state.RemoteMessageID, "thread-1:answer-1"; got != want {
			t.Fatalf("stored textual remote id = %q, want %q", got, want)
		}
	})

	t.Run("Should acknowledge text after best-effort typing cleanup fails", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{
			postedMessageIDs: []string{"progress-1", "answer-1"},
			typingStopErr:    errors.New("typing cleanup failed"),
		}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-discord": {
				instanceID: "brg-discord",
				managed:    testDiscordManagedInstance("brg-discord"),
			},
		}, nil)
		session := &bridgesdk.Session{}

		progress := testDiscordProgressRequest(
			"delivery-cleanup",
			1,
			bridgepkg.ToolProgress{
				ToolCallID: "tool-call-cleanup",
				ToolID:     "agh__search",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Search",
				Index:      1,
			},
		)
		if _, err := provider.handleBridgesProgress(context.Background(), session, progress); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		final := testDiscordTextRequest(
			"delivery-cleanup",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			"Answer despite cleanup",
			true,
			"",
		)
		ack, err := provider.handleBridgesDeliver(context.Background(), session, final)
		if err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v, want best-effort cleanup", err)
		}
		if got, want := ack.RemoteMessageID, "thread-1:answer-1"; got != want {
			t.Fatalf("final ack remote id = %q, want %q", got, want)
		}
		if err := provider.healthCheck(); err == nil || !strings.Contains(err.Error(), "typing cleanup failed") {
			t.Fatalf("healthCheck() error = %v, want registered cleanup failure", err)
		}
		if got := provider.deliveryState("brg-discord", "delivery-cleanup").Progress; got != nil {
			t.Fatal("terminal cleanup failure retained progress dispatcher")
		}
	})
}

func TestDiscordProgressUsesRuneMessageLength(t *testing.T) {
	t.Run("Should keep a multibyte line in one Discord message below the rune cap", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{postedMessageID: "progress-runes"}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-discord": {
				instanceID: "brg-discord",
				managed:    testDiscordManagedInstance("brg-discord"),
			},
		}, nil)
		label := strings.Repeat("界", 1900)
		request := testDiscordProgressRequest(
			"delivery-runes",
			1,
			bridgepkg.ToolProgress{
				ToolCallID: "tool-call-runes",
				ToolID:     "agh__search",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      label,
				Index:      1,
			},
		)
		if _, err := provider.handleBridgesProgress(context.Background(), &bridgesdk.Session{}, request); err != nil {
			t.Fatalf("handleBridgesProgress() error = %v", err)
		}
		if got, want := len(api.postRequests), 1; got != want {
			t.Fatalf("post requests = %d, want %d", got, want)
		}
		content := api.postRequests[0].Content
		if got := len(content); got <= discordMaxMessageLen {
			t.Fatalf("UTF-8 bytes = %d, want > %d to prove rune measurement", got, discordMaxMessageLen)
		}
		if got, limit := utf8.RuneCountInString(content), discordMaxMessageLen; got > limit {
			t.Fatalf("runes = %d, want <= %d", got, limit)
		}
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			&bridgesdk.Session{},
			testDiscordLifecycleFinalRequest("delivery-runes", 2),
		); err != nil {
			t.Fatalf("handleBridgesProgress(final) error = %v", err)
		}
	})
}

func TestDiscordProgressDispatcherLifecycle(t *testing.T) {
	t.Run("Should retain a failed progress dispatcher for provider retry", func(t *testing.T) {
		t.Parallel()

		api := &discordAPIFake{
			postedMessageID: "progress-retry",
			postErr:         errors.New("progress post failed"),
		}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		apiFactoryCalls := 0
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI {
			apiFactoryCalls++
			return api
		}
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-discord": {
				instanceID: "brg-discord",
				managed:    testDiscordManagedInstance("brg-discord"),
			},
		}, nil)
		session := &bridgesdk.Session{}
		request := testDiscordProgressRequest("delivery-provider-retry", 1, bridgepkg.ToolProgress{
			ToolCallID: "tool-call-retry",
			ToolID:     "agh__search",
			Phase:      bridgepkg.ToolProgressPhaseStarted,
			Label:      "Search",
			Index:      1,
		})

		if _, err := provider.handleBridgesProgress(context.Background(), session, request); err == nil ||
			!strings.Contains(err.Error(), "progress post failed") {
			t.Fatalf("handleBridgesProgress(first) error = %v, want post failure", err)
		}
		before := provider.deliveryState("brg-discord", "delivery-provider-retry").Progress
		if before == nil {
			t.Fatal("failed progress delivery did not retain dispatcher")
		}
		api.postErr = nil
		if _, err := provider.handleBridgesProgress(context.Background(), session, request); err != nil {
			t.Fatalf("handleBridgesProgress(retry) error = %v", err)
		}
		after := provider.deliveryState("brg-discord", "delivery-provider-retry").Progress
		if after != before {
			t.Fatal("provider retry replaced the retained dispatcher")
		}
		if got, want := apiFactoryCalls, 1; got != want {
			t.Fatalf("apiFactory calls = %d, want %d", got, want)
		}
		if got, want := len(api.postAttempts), 2; got != want {
			t.Fatalf("post attempts = %d, want %d", got, want)
		}
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordLifecycleFinalRequest("delivery-provider-retry", 2),
		); err != nil {
			t.Fatalf("handleBridgesProgress(final) error = %v", err)
		}
	})

	t.Run("Should register a scheduled edit failure and recover it on lifecycle flush", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
		timer := &discordManualProgressTimer{}
		api := &discordAPIFake{
			postedMessageID: "progress-async",
			updateErr:       errors.New("scheduled edit failed"),
		}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.now = func() time.Time { return now }
		provider.progressDispatcherOptions = []bridgesdk.ProgressDispatcherOption{
			bridgesdk.WithProgressSchedule(timer.Schedule),
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-discord": {
				instanceID: "brg-discord",
				managed:    testDiscordManagedInstance("brg-discord"),
			},
		}, nil)
		session := &bridgesdk.Session{}

		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordProgressRequest("delivery-async", 1, bridgepkg.ToolProgress{
				ToolCallID: "tool-call-a",
				ToolID:     "agh__search",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Search",
				Index:      1,
			}),
		); err != nil {
			t.Fatalf("handleBridgesProgress(first) error = %v", err)
		}
		now = now.Add(500 * time.Millisecond)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordProgressRequest("delivery-async", 2, bridgepkg.ToolProgress{
				ToolCallID: "tool-call-b",
				ToolID:     "agh__read",
				Phase:      bridgepkg.ToolProgressPhaseCompleted,
				Label:      "Read",
				Index:      2,
			}),
		); err != nil {
			t.Fatalf("handleBridgesProgress(second) error = %v", err)
		}
		now = now.Add(time.Second)
		timer.Run(t)
		state := provider.deliveryState("brg-discord", "delivery-async")
		if state.Progress == nil {
			t.Fatal("scheduled failure removed dispatcher before retry")
		}
		asyncErr := state.Progress.LastAsyncError()
		if asyncErr == nil || !strings.Contains(asyncErr.Error(), "scheduled edit failed") {
			t.Fatalf("LastAsyncError() = %v, want scheduled edit failure", asyncErr)
		}
		if err := provider.healthCheck(); err == nil || !strings.Contains(err.Error(), "scheduled edit failed") {
			t.Fatalf("healthCheck() error = %v, want registered async failure", err)
		}

		api.updateErr = nil
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordLifecycleFinalRequest("delivery-async", 3),
		); err != nil {
			t.Fatalf("handleBridgesProgress(final retry) error = %v", err)
		}
		if got := provider.deliveryState("brg-discord", "delivery-async").Progress; got != nil {
			t.Fatal("successful lifecycle retry retained dispatcher")
		}
	})

	t.Run("Should retain the dispatcher when mandatory lifecycle flush fails", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 13, 5, 0, 0, time.UTC)
		timer := &discordManualProgressTimer{}
		api := &discordAPIFake{
			postedMessageID: "progress-flush",
			updateErr:       errors.New("flush failed"),
		}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.now = func() time.Time { return now }
		provider.progressDispatcherOptions = []bridgesdk.ProgressDispatcherOption{
			bridgesdk.WithProgressSchedule(timer.Schedule),
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-discord": {
				instanceID: "brg-discord",
				managed:    testDiscordManagedInstance("brg-discord"),
			},
		}, nil)
		session := &bridgesdk.Session{}

		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordProgressRequest("delivery-flush", 1, bridgepkg.ToolProgress{
				ToolCallID: "tool-call-a",
				ToolID:     "agh__search",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Search",
				Index:      1,
			}),
		); err != nil {
			t.Fatalf("handleBridgesProgress(first) error = %v", err)
		}
		now = now.Add(500 * time.Millisecond)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordProgressRequest("delivery-flush", 2, bridgepkg.ToolProgress{
				ToolCallID: "tool-call-b",
				ToolID:     "agh__read",
				Phase:      bridgepkg.ToolProgressPhaseCompleted,
				Label:      "Read",
				Index:      2,
			}),
		); err != nil {
			t.Fatalf("handleBridgesProgress(second) error = %v", err)
		}
		before := provider.deliveryState("brg-discord", "delivery-flush").Progress
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordLifecycleFinalRequest("delivery-flush", 3),
		); err == nil || !strings.Contains(err.Error(), "flush failed") {
			t.Fatalf("handleBridgesProgress(final) error = %v, want flush failure", err)
		}
		if got := provider.deliveryState("brg-discord", "delivery-flush").Progress; got != before {
			t.Fatal("failed lifecycle flush replaced or detached dispatcher")
		}

		api.updateErr = nil
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordLifecycleFinalRequest("delivery-flush", 3),
		); err != nil {
			t.Fatalf("handleBridgesProgress(final retry) error = %v", err)
		}
	})

	t.Run("Should close a running dispatcher after releasing the provider mutex", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 13, 10, 0, 0, time.UTC)
		timer := &discordManualProgressTimer{}
		updateStarted := make(chan struct{})
		updateRelease := make(chan struct{})
		api := &discordAPIFake{
			postedMessageID: "progress-stop",
			updateErr:       errors.New("async stop failure"),
			updateStarted:   updateStarted,
			updateBlock:     updateRelease,
		}
		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		provider.now = func() time.Time { return now }
		provider.progressDispatcherOptions = []bridgesdk.ProgressDispatcherOption{
			bridgesdk.WithProgressSchedule(timer.Schedule),
		}
		provider.apiFactory = func(resolvedInstanceConfig) discordAPI { return api }
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-discord": {
				instanceID: "brg-discord",
				managed:    testDiscordManagedInstance("brg-discord"),
			},
		}, nil)
		session := &bridgesdk.Session{}

		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordProgressRequest("delivery-stop", 1, bridgepkg.ToolProgress{
				ToolCallID: "tool-call-a",
				ToolID:     "agh__search",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Search",
				Index:      1,
			}),
		); err != nil {
			t.Fatalf("handleBridgesProgress(first) error = %v", err)
		}
		now = now.Add(500 * time.Millisecond)
		if _, err := provider.handleBridgesProgress(
			context.Background(),
			session,
			testDiscordProgressRequest("delivery-stop", 2, bridgepkg.ToolProgress{
				ToolCallID: "tool-call-b",
				ToolID:     "agh__read",
				Phase:      bridgepkg.ToolProgressPhaseCompleted,
				Label:      "Read",
				Index:      2,
			}),
		); err != nil {
			t.Fatalf("handleBridgesProgress(second) error = %v", err)
		}
		now = now.Add(time.Second)
		callbackDone := make(chan struct{})
		go func() {
			timer.Run(t)
			close(callbackDone)
		}()
		select {
		case <-updateStarted:
		case <-time.After(time.Second):
			t.Fatal("scheduled update did not start")
		}

		provider.mu.Lock()
		stopDone := make(chan struct{})
		go func() {
			provider.lifecycle.Stop()
			close(stopDone)
		}()
		close(updateRelease)
		provider.mu.Unlock()
		for name, done := range map[string]<-chan struct{}{
			"callback": callbackDone,
			"stop":     stopDone,
		} {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("%s deadlocked while closing progress dispatcher", name)
			}
		}
		if got := provider.deliveryState("brg-discord", "delivery-stop").Progress; got != nil {
			t.Fatal("stop retained progress dispatcher")
		}
	})
}

func testDiscordTextRequest(
	deliveryID string,
	seq int64,
	eventType bridgepkg.DeliveryEventType,
	text string,
	final bool,
	remoteID string,
) bridgepkg.DeliveryRequest {
	request := testDiscordProgressRequest(deliveryID, seq, bridgepkg.ToolProgress{})
	request.Event.EventType = eventType
	request.Event.Content = bridgepkg.MessageContent{Text: text}
	request.Event.Final = final
	request.Event.Progress = nil
	if remoteID != "" {
		request.Event.Operation = bridgepkg.DeliveryOperationEdit
		request.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: remoteID}
	}
	return request
}
