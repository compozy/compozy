package core

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
	"github.com/gin-gonic/gin"
)

type streamTestFlushWriter struct {
	bytes.Buffer
}

func (streamTestFlushWriter) Flush() {}

type streamTestFailWriter struct {
	err error
}

func (w streamTestFailWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func (streamTestFailWriter) Flush() {}

type rawStreamSubscriberStub struct {
	sessionManagerStub
	after  []int64
	events <-chan store.SessionEvent
}

func (s *rawStreamSubscriberStub) SubscribeSessionEvents(
	_ context.Context,
	_ string,
	afterSequence int64,
) (<-chan store.SessionEvent, func(), error) {
	s.after = append(s.after, afterSequence)
	return s.events, func() {}, nil
}

type wakeStreamSubscriberStub struct {
	*rawStreamSubscriberStub
	wakeCalls int
	wakes     <-chan store.SessionEvent
}

func (s *wakeStreamSubscriberStub) SubscribeSessionEventWakes(
	context.Context,
	string,
) (<-chan store.SessionEvent, func(), error) {
	s.wakeCalls++
	return s.wakes, func() {}, nil
}

func streamTestSessionInfo(id string) *session.Info {
	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	return &session.Info{
		ID:              id,
		Name:            "demo",
		AgentName:       "coder",
		WorkspaceID:     "ws-workspace",
		Workspace:       "/workspace",
		State:           session.StateActive,
		TranscriptEpoch: 3,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestTranscriptStreamErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("Should emit one terminal error and cancel the live subscription", func(t *testing.T) {
		t.Parallel()

		initializeErr := errors.New("projection unavailable")
		handlers := &BaseHandlers{Sessions: sessionManagerStub{
			transcriptPage: func(
				context.Context,
				string,
				transcript.PageQuery,
			) (transcript.Page, error) {
				return transcript.Page{}, initializeErr
			},
		}}
		gin.SetMode(gin.TestMode)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequestWithContext(context.Background(), "GET", "/stream", http.NoBody)
		writer := &streamTestFlushWriter{}
		cancelCalls := 0

		handlers.streamTranscriptSessionEvents(
			ctx,
			writer,
			"sess-a",
			streamTestSessionInfo("sess-a"),
			store.EventQuery{Limit: 2},
			nil,
			sessionStreamOptions{frameMode: contract.SessionStreamFrameTranscript},
			sessionEventStreamSubscription{
				events: make(chan store.SessionEvent),
				cancel: func() { cancelCalls++ },
			},
		)

		if cancelCalls != 1 {
			t.Fatalf("subscription cancel calls = %d, want 1", cancelCalls)
		}
		if got := strings.Count(writer.String(), "event: error"); got != 1 {
			t.Fatalf("terminal error frames = %d, want 1; body=%s", got, writer.String())
		}
		for _, fragment := range []string{"initialize transcript stream", "projection unavailable"} {
			if !strings.Contains(writer.String(), fragment) {
				t.Fatalf("terminal error body missing %q: %s", fragment, writer.String())
			}
		}
	})

	t.Run("Should log a terminal error write failure", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		handlers := &BaseHandlers{Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
		handlers.writeTranscriptStreamError(
			streamTestFailWriter{err: errors.New("stream sink closed")},
			errors.New("projection unavailable"),
		)
		for _, fragment := range []string{
			`"msg":"api: failed to emit sse message"`,
			`"event":"error"`,
			"stream sink closed",
		} {
			if !strings.Contains(logs.String(), fragment) {
				t.Fatalf("SSE failure log missing %q: %s", fragment, logs.String())
			}
		}
	})

	t.Run("Should close without a warning or error frame when the session is removed", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		stopped := streamTestSessionInfo("sess-a")
		stopped.State = session.StateStopped
		events := make(chan store.SessionEvent, 1)
		events <- store.SessionEvent{Type: session.EventTypeSessionStopped}
		handlers := &BaseHandlers{
			Logger: slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
			Sessions: sessionManagerStub{
				status: func(context.Context, string) (*session.Info, error) {
					return stopped, nil
				},
				transcriptChanges: func(
					context.Context,
					string,
					transcript.ChangeQuery,
				) (transcript.ChangePage, error) {
					return transcript.ChangePage{}, session.ErrSessionNotFound
				},
			},
		}
		gin.SetMode(gin.TestMode)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequestWithContext(context.Background(), "GET", "/stream", http.NoBody)
		writer := &streamTestFlushWriter{}

		handlers.pushAndStreamSessionTranscript(
			ctx,
			writer,
			"sess-a",
			streamTestSessionInfo("sess-a"),
			transcriptStreamState{cursor: 7, generation: 2, epoch: 3},
			2,
			sessionEventStreamSubscription{events: events, cancel: func() {}},
		)

		if strings.Contains(writer.String(), "event: error") {
			t.Fatalf("stream emitted an error frame after deletion: %s", writer.String())
		}
		if strings.Contains(logs.String(), `"level":"WARN"`) {
			t.Fatalf("stream logged expected deletion at warning level: %s", logs.String())
		}
	})
}

func TestSessionStreamFallbackCancelsSubscriptionBeforePolling(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		event *store.SessionEvent
	}{
		{name: "Should cancel raw subscription before polling after closed subscription"},
		{
			name:  "Should cancel raw subscription before polling after sequence gap",
			event: &store.SessionEvent{Sequence: 3},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			streamDone := make(chan struct{})
			handlers := &BaseHandlers{PollInterval: time.Hour}
			handlers.SetStreamDone(streamDone)
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequestWithContext(context.Background(), "GET", "/stream", http.NoBody)
			writer := &streamTestFlushWriter{}

			events := make(chan store.SessionEvent, 1)
			if testCase.event != nil {
				events <- *testCase.event
			}
			close(events)
			canceled := make(chan struct{})
			var cancelOnce sync.Once
			subscription := sessionEventStreamSubscription{
				events: events,
				cancel: func() {
					cancelOnce.Do(func() { close(canceled) })
				},
			}

			done := make(chan struct{})
			go func() {
				defer close(done)
				handlers.pushAndStreamSessionEvents(
					ctx,
					writer,
					"sess-a",
					streamTestSessionInfo("sess-a"),
					store.EventQuery{},
					1,
					subscription,
				)
			}()

			waitForStreamTestSignal(t, canceled, "subscription cancel")
			close(streamDone)
			waitForStreamTestSignal(t, done, "stream fallback return")
		})
	}
}

func TestSubscribeSessionEventStreamMode(t *testing.T) {
	t.Parallel()

	t.Run("Should use wake-only subscriptions for transcript streams", func(t *testing.T) {
		t.Parallel()

		wakes := make(chan store.SessionEvent)
		manager := &wakeStreamSubscriberStub{
			rawStreamSubscriberStub: &rawStreamSubscriberStub{},
			wakes:                   wakes,
		}
		handlers := &BaseHandlers{Sessions: manager}

		subscription, err := handlers.subscribeSessionEventStream(
			context.Background(),
			"sess-a",
			99,
			contract.SessionStreamFrameTranscript,
		)
		if err != nil {
			t.Fatalf("subscribeSessionEventStream() error = %v", err)
		}
		if !subscription.active() || subscription.events != wakes {
			t.Fatal("transcript subscription is inactive or did not use wake channel")
		}
		if manager.wakeCalls != 1 || len(manager.after) != 0 {
			t.Fatalf(
				"wake/raw calls = %d/%#v, want one wake and no filtered subscription",
				manager.wakeCalls,
				manager.after,
			)
		}
	})

	t.Run("Should fall back to transcript polling when only filtered subscriptions exist", func(t *testing.T) {
		t.Parallel()

		manager := &rawStreamSubscriberStub{}
		handlers := &BaseHandlers{Sessions: manager}

		subscription, err := handlers.subscribeSessionEventStream(
			context.Background(),
			"sess-a",
			99,
			contract.SessionStreamFrameTranscript,
		)
		if err != nil {
			t.Fatalf("subscribeSessionEventStream() error = %v", err)
		}
		if subscription.active() || len(manager.after) != 0 {
			t.Fatalf(
				"transcript subscription = %#v raw calls=%#v, want inactive polling fallback",
				subscription,
				manager.after,
			)
		}
	})

	t.Run("Should preserve filtered subscriptions for raw streams", func(t *testing.T) {
		t.Parallel()

		events := make(chan store.SessionEvent)
		manager := &rawStreamSubscriberStub{events: events}
		handlers := &BaseHandlers{Sessions: manager}

		subscription, err := handlers.subscribeSessionEventStream(
			context.Background(),
			"sess-a",
			99,
			contract.SessionStreamFrameRaw,
		)
		if err != nil {
			t.Fatalf("subscribeSessionEventStream() error = %v", err)
		}
		if !subscription.active() || subscription.events != events {
			t.Fatal("raw subscription is inactive or did not use filtered channel")
		}
		if len(manager.after) != 1 || manager.after[0] != 99 {
			t.Fatalf("raw subscription cursors = %#v, want [99]", manager.after)
		}
	})
}

func TestTranscriptPushTreatsRawSequenceAsWakeOnly(t *testing.T) {
	t.Parallel()

	streamDone := make(chan struct{})
	statusCalls := 0
	pageCalls := 0
	handlers := &BaseHandlers{
		Sessions: sessionManagerStub{
			status: func(_ context.Context, id string) (*session.Info, error) {
				statusCalls++
				info := streamTestSessionInfo(id)
				info.TranscriptEpoch = 4
				return info, nil
			},
			transcriptPage: func(
				context.Context,
				string,
				transcript.PageQuery,
			) (transcript.Page, error) {
				pageCalls++
				close(streamDone)
				return transcript.Page{
					Entries:     []transcript.Entry{{StartSequence: 1, Sequence: 1}},
					Generation:  5,
					MaxSequence: 1,
				}, nil
			},
			transcriptChanges: func(
				context.Context,
				string,
				transcript.ChangeQuery,
			) (transcript.ChangePage, error) {
				t.Fatal("TranscriptChanges() called before epoch reset snapshot")
				return transcript.ChangePage{}, nil
			},
		},
		PollInterval: time.Hour,
	}
	handlers.SetStreamDone(streamDone)
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequestWithContext(context.Background(), "GET", "/stream", http.NoBody)
	eventWake := make(chan store.SessionEvent, 1)
	eventWake <- store.SessionEvent{Sequence: 1}
	writer := &streamTestFlushWriter{}

	handlers.pushAndStreamSessionTranscript(
		ctx,
		writer,
		"sess-a",
		streamTestSessionInfo("sess-a"),
		transcriptStreamState{cursor: 10, generation: 4, epoch: 3},
		2,
		sessionEventStreamSubscription{events: eventWake, cancel: func() {}},
	)

	if statusCalls != 1 || pageCalls != 1 {
		t.Fatalf("status/page calls = %d/%d, want one refresh from lower raw sequence", statusCalls, pageCalls)
	}
	for _, fragment := range []string{
		"event: transcript_snapshot",
		`"epoch":4`,
		`"generation":5`,
		`"reset":true`,
		`"reason":"epoch_mismatch"`,
	} {
		if !strings.Contains(writer.String(), fragment) {
			t.Fatalf("stream body missing %q: %s", fragment, writer.String())
		}
	}
}

func TestTranscriptReconnectFence(t *testing.T) {
	t.Parallel()

	epoch := int64(3)
	generation := int64(4)
	testCases := []struct {
		name               string
		cursor             int64
		expectedEpoch      *int64
		expectedGeneration *int64
		want               string
	}{
		{
			name:   "Should allow an initial subscription without reconnect fences",
			cursor: 0,
		},
		{
			name:   "Should reset a reconnect missing expected identity",
			cursor: 7,
			want:   contract.TranscriptSnapshotReasonFenceMissing,
		},
		{
			name:               "Should reset an epoch mismatch",
			cursor:             7,
			expectedEpoch:      int64TestPointer(2),
			expectedGeneration: &generation,
			want:               contract.TranscriptSnapshotReasonEpochMismatch,
		},
		{
			name:               "Should reset a generation mismatch",
			cursor:             7,
			expectedEpoch:      &epoch,
			expectedGeneration: int64TestPointer(3),
			want:               contract.TranscriptSnapshotReasonGenerationMismatch,
		},
		{
			name:               "Should reset a cursor beyond the projection max",
			cursor:             13,
			expectedEpoch:      &epoch,
			expectedGeneration: &generation,
			want:               contract.TranscriptSnapshotReasonSequenceReset,
		},
		{
			name:               "Should preserve loaded history for a valid reconnect",
			cursor:             7,
			expectedEpoch:      &epoch,
			expectedGeneration: &generation,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := transcriptReconnectResetReason(
				testCase.cursor,
				testCase.expectedEpoch,
				testCase.expectedGeneration,
				epoch,
				generation,
				12,
			)
			if got != testCase.want {
				t.Fatalf("transcriptReconnectResetReason() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestInitializeTranscriptStreamResetsMismatchedReconnect(t *testing.T) {
	t.Parallel()

	pageCalls := 0
	changeCalls := 0
	handlers := &BaseHandlers{Sessions: sessionManagerStub{
		transcriptPage: func(
			context.Context,
			string,
			transcript.PageQuery,
		) (transcript.Page, error) {
			pageCalls++
			return transcript.Page{
				Entries:     []transcript.Entry{{StartSequence: 7, Sequence: 7}},
				Generation:  4,
				MaxSequence: 7,
			}, nil
		},
		transcriptChanges: func(
			context.Context,
			string,
			transcript.ChangeQuery,
		) (transcript.ChangePage, error) {
			changeCalls++
			return transcript.ChangePage{Generation: 4, MaxSequence: 7, NextAfter: 5}, nil
		},
	}}
	writer := &streamTestFlushWriter{}
	wantEpoch := int64(2)
	wantGeneration := int64(4)

	state, err := handlers.initializeTranscriptStream(
		context.Background(),
		writer,
		"sess-a",
		streamTestSessionInfo("sess-a"),
		5,
		2,
		sessionStreamOptions{
			expectedEpoch:      &wantEpoch,
			expectedGeneration: &wantGeneration,
		},
		nil,
	)
	if err != nil {
		t.Fatalf("initializeTranscriptStream() error = %v", err)
	}
	if state.cursor != 7 || state.generation != 4 || state.epoch != 3 {
		t.Fatalf("stream state = %#v, want cursor/generation/epoch 7/4/3", state)
	}
	if pageCalls != 1 || changeCalls != 1 {
		t.Fatalf("page/change calls = %d/%d, want one fence read and one reset snapshot", pageCalls, changeCalls)
	}
	for _, fragment := range []string{
		"event: transcript_snapshot",
		"id: 7",
		`"reset":true`,
		`"reason":"epoch_mismatch"`,
	} {
		if !strings.Contains(writer.String(), fragment) {
			t.Fatalf("stream body missing %q: %s", fragment, writer.String())
		}
	}
	if strings.Contains(writer.String(), "event: transcript_delta") {
		t.Fatalf("stream body contains a delta during reset: %s", writer.String())
	}
}

func TestWriteGoalSnapshotChangedEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should emit the latest indexed Goal signal without a transcript delta", func(t *testing.T) {
		t.Parallel()

		handlers := &BaseHandlers{Sessions: sessionManagerStub{
			latestEvent: func(
				_ context.Context,
				sessionID string,
				eventType string,
			) (*store.SessionEvent, error) {
				if sessionID != "sess-a" || eventType != session.EventTypeGoalSnapshotChanged {
					t.Fatalf("LatestSessionEventByType(%q, %q), want sess-a Goal snapshot", sessionID, eventType)
				}
				return &store.SessionEvent{
					Sequence: 12,
					Type:     session.EventTypeGoalSnapshotChanged,
					Content:  `{"run_id":"run-1","status":"running"}`,
				}, nil
			},
		}}
		writer := &streamTestFlushWriter{}

		if err := handlers.writeGoalSnapshotChangedEvents(
			context.Background(),
			writer,
			"sess-a",
			11,
			nil,
		); err != nil {
			t.Fatalf("writeGoalSnapshotChangedEvents() error = %v", err)
		}

		body := writer.String()
		for _, fragment := range []string{
			"id: 12",
			"event: goal_snapshot_changed",
			`data: {"run_id":"run-1","status":"running"}`,
		} {
			if !strings.Contains(body, fragment) {
				t.Fatalf("Goal snapshot stream body missing %q: %s", fragment, body)
			}
		}
		if strings.Contains(body, "event: transcript_delta") {
			t.Fatalf("Goal snapshot signal was emitted as a transcript delta: %s", body)
		}
	})
}

func TestWriteTranscriptChangePages(t *testing.T) {
	t.Parallel()

	t.Run("Should drain repairs and advance through an empty visible change page", func(t *testing.T) {
		t.Parallel()

		calls := make([]transcript.ChangeQuery, 0, 2)
		handlers := &BaseHandlers{Sessions: sessionManagerStub{
			transcriptChanges: func(
				_ context.Context,
				_ string,
				query transcript.ChangeQuery,
			) (transcript.ChangePage, error) {
				calls = append(calls, query)
				switch len(calls) {
				case 1:
					return transcript.ChangePage{
						Entries: []transcript.Entry{{
							StartSequence: 2,
							Sequence:      9,
							Message: transcript.UIMessage{
								ID:   "historical-tool-entry",
								Role: transcript.UIRoleAssistant,
							},
						}},
						Generation:  4,
						MaxSequence: 12,
						HasMore:     true,
						NextAfter:   9,
					}, nil
				default:
					return transcript.ChangePage{
						Entries:     []transcript.Entry{},
						Generation:  4,
						MaxSequence: 12,
						NextAfter:   9,
					}, nil
				}
			},
			events: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
				t.Fatal("Events() called for transcript changes")
				return nil, nil
			},
		}}
		writer := &streamTestFlushWriter{}

		cursor, generation, err := handlers.writeTranscriptChangePages(
			context.Background(),
			writer,
			"sess-a",
			streamTestSessionInfo("sess-a"),
			7,
			2,
			nil,
		)
		if err != nil {
			t.Fatalf("writeTranscriptChangePages() error = %v", err)
		}
		if cursor != 12 || generation != 4 {
			t.Fatalf("cursor/generation = %d/%d, want 12/4", cursor, generation)
		}
		if len(calls) != 2 || calls[0].AfterSequence != 7 || calls[1].AfterSequence != 9 {
			t.Fatalf("TranscriptChanges() calls = %#v, want cursors 7 then 9", calls)
		}
		body := writer.String()
		if got, want := strings.Count(body, "event: transcript_delta"), 2; got != want {
			t.Fatalf("transcript delta frames = %d, want %d\n%s", got, want, body)
		}
		for _, fragment := range []string{
			"id: 9",
			"id: 12",
			`"start_sequence":2`,
			`"entries":[]`,
		} {
			if !strings.Contains(body, fragment) {
				t.Fatalf("stream body missing %q: %s", fragment, body)
			}
		}
	})

	t.Run("Should reject a generation change after a zero-generation page", func(t *testing.T) {
		t.Parallel()

		calls := 0
		handlers := &BaseHandlers{Sessions: sessionManagerStub{
			transcriptChanges: func(
				context.Context,
				string,
				transcript.ChangeQuery,
			) (transcript.ChangePage, error) {
				calls++
				if calls == 1 {
					return transcript.ChangePage{
						Entries:     []transcript.Entry{{StartSequence: 8, Sequence: 8}},
						Generation:  0,
						MaxSequence: 9,
						HasMore:     true,
						NextAfter:   8,
					}, nil
				}
				return transcript.ChangePage{
					Entries:     []transcript.Entry{{StartSequence: 9, Sequence: 9}},
					Generation:  1,
					MaxSequence: 9,
					NextAfter:   9,
				}, nil
			},
		}}

		_, _, err := handlers.writeTranscriptChangePages(
			context.Background(),
			&streamTestFlushWriter{},
			"sess-a",
			streamTestSessionInfo("sess-a"),
			7,
			2,
			nil,
		)
		if !errors.Is(err, transcript.ErrProjectionIncompatible) {
			t.Fatalf("writeTranscriptChangePages() error = %v, want projection incompatibility", err)
		}
	})
}

func int64TestPointer(value int64) *int64 {
	return new(value)
}

func waitForStreamTestSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-signal:
	case <-timer.C:
		t.Fatalf("timed out waiting for %s", label)
	}
}
