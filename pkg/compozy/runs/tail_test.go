package runs

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestReplayPagesEventsInOrder(t *testing.T) {
	run := &Run{
		summary: RunSummary{RunID: "run-replay"},
		client: &stubDaemonRunReader{
			eventPages: []remoteRunEventPage{
				{
					Events: []events.Event{
						testEvent("run-replay", 1, events.EventKindRunStarted),
						testEvent("run-replay", 2, events.EventKindJobStarted),
					},
					NextCursor: remoteCursor(time.Unix(2, 0).UTC(), 2),
					HasMore:    true,
				},
				{
					Events: []events.Event{
						testEvent("run-replay", 3, events.EventKindRunCompleted),
					},
					NextCursor: remoteCursor(time.Unix(3, 0).UTC(), 3),
				},
			},
		},
	}

	gotEvents, gotErrs := collectReplay(run, 2)
	if len(gotErrs) != 0 {
		t.Fatalf("Replay() unexpected errors: %v", gotErrs)
	}
	if seqs := collectedSeqs(gotEvents); !slices.Equal(seqs, []uint64{2, 3}) {
		t.Fatalf("Replay() seqs = %v, want [2 3]", seqs)
	}
}

func TestReplayReportsIncompatibleSchemaVersion(t *testing.T) {
	run := &Run{
		summary: RunSummary{RunID: "run-replay-schema"},
		client: &stubDaemonRunReader{
			eventPages: []remoteRunEventPage{{
				Events: []events.Event{{
					SchemaVersion: "99.0",
					RunID:         "run-replay-schema",
					Seq:           1,
					Timestamp:     time.Unix(1, 0).UTC(),
					Kind:          events.EventKindRunStarted,
				}},
			}},
		},
	}

	_, gotErrs := collectReplay(run, 0)
	if len(gotErrs) != 1 || !errors.Is(gotErrs[0], ErrIncompatibleSchemaVersion) {
		t.Fatalf("Replay() errors = %v, want ErrIncompatibleSchemaVersion", gotErrs)
	}
}

func TestTailReplaysHistoryThenFollowsStreamWithoutDuplicates(t *testing.T) {
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	reader := &stubDaemonRunReader{
		snapshot: RemoteRunSnapshot{
			Status:     publicRunStatusRunning,
			NextCursor: remoteCursor(now.Add(2*time.Second), 2),
		},
		eventPages: []remoteRunEventPage{{
			Events: []events.Event{
				{
					SchemaVersion: events.SchemaVersion,
					RunID:         "run-tail",
					Seq:           1,
					Timestamp:     now,
					Kind:          events.EventKindRunStarted,
				},
				{
					SchemaVersion: events.SchemaVersion,
					RunID:         "run-tail",
					Seq:           2,
					Timestamp:     now.Add(time.Second),
					Kind:          events.EventKindJobStarted,
				},
			},
			NextCursor: remoteCursor(now.Add(2*time.Second), 2),
		}},
		streams: []RemoteRunStream{
			newBufferedRemoteRunStream(
				RemoteRunStreamItem{Event: &events.Event{
					SchemaVersion: events.SchemaVersion,
					RunID:         "run-tail",
					Seq:           3,
					Timestamp:     now.Add(2 * time.Second),
					Kind:          events.EventKindRunCompleted,
				}},
			),
		},
	}
	run := &Run{
		summary: RunSummary{RunID: "run-tail"},
		client:  reader,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eventsCh, errsCh := run.Tail(ctx, 2)
	got := collectTailEvents(t, eventsCh, errsCh, 2, time.Second)
	if seqs := collectedSeqs(got); !slices.Equal(seqs, []uint64{2, 3}) {
		t.Fatalf("Tail() seqs = %v, want [2 3]", seqs)
	}
	if len(reader.streamOpen) != 1 || reader.streamOpen[0].Sequence != 2 {
		t.Fatalf("streamOpen = %#v, want cursor seq=2", reader.streamOpen)
	}
}

func TestTailNilRunReturnsErrorAndClosedChannels(t *testing.T) {
	var run *Run
	eventsCh, errsCh := run.Tail(context.Background(), 0)

	select {
	case err, ok := <-errsCh:
		if !ok || err == nil || !strings.Contains(err.Error(), "nil run") {
			t.Fatalf("Tail() error = %v, want nil run error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for nil-run tail error")
	}

	waitForEventChannelClose(t, eventsCh, "events", time.Second)
	waitForEventChannelClose(t, errsCh, "errors", time.Second)
}
