package runs

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

func TestWatchRemoteReconnectsAfterOverflowWithoutDuplicatingEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 23, 0, 0, 0, time.UTC)
	client := &stubRemoteStreamClient{
		streams: []RemoteRunStream{
			newBufferedRemoteRunStream(
				RemoteRunStreamItem{Event: &eventspkg.Event{
					RunID:     "run-remote-001",
					Kind:      eventspkg.EventKindRunStarted,
					Seq:       1,
					Timestamp: now,
				}},
				RemoteRunStreamItem{OverflowCursor: &RemoteCursor{
					Timestamp: now,
					Sequence:  1,
				}},
			),
			newBufferedRemoteRunStream(
				RemoteRunStreamItem{Event: &eventspkg.Event{
					RunID:     "run-remote-001",
					Kind:      eventspkg.EventKindJobCompleted,
					Seq:       2,
					Timestamp: now.Add(time.Second),
				}},
				RemoteRunStreamItem{Event: &eventspkg.Event{
					RunID:     "run-remote-001",
					Kind:      eventspkg.EventKindRunCompleted,
					Seq:       3,
					Timestamp: now.Add(2 * time.Second),
				}},
			),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eventsCh, errsCh := WatchRemote(ctx, client, "run-remote-001")
	var got []uint64
	for event := range eventsCh {
		got = append(got, event.Seq)
	}
	for err := range errsCh {
		if err != nil {
			t.Fatalf("unexpected watch error: %v", err)
		}
	}

	if !reflect.DeepEqual(got, []uint64{1, 2, 3}) {
		t.Fatalf("unexpected watched sequences: got %v want [1 2 3]", got)
	}
	if len(client.openAfter) != 2 {
		t.Fatalf("expected two stream opens, got %d", len(client.openAfter))
	}
	if client.openAfter[0].Sequence != 0 {
		t.Fatalf("expected initial stream open from zero cursor, got %#v", client.openAfter[0])
	}
	if client.openAfter[1].Sequence != 1 {
		t.Fatalf("expected reconnect from last delivered cursor, got %#v", client.openAfter[1])
	}
}

func TestWatchRemoteStopsAfterHeartbeatEOFWhenSnapshotIsTerminal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 23, 5, 0, 0, time.UTC)
	client := &stubRemoteStreamClient{
		streams: []RemoteRunStream{
			newBufferedRemoteRunStream(
				RemoteRunStreamItem{HeartbeatCursor: &RemoteCursor{
					Timestamp: now,
					Sequence:  4,
				}},
			),
		},
		snapshot: RemoteRunSnapshot{
			Status: "completed",
			NextCursor: &RemoteCursor{
				Timestamp: now,
				Sequence:  4,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eventsCh, errsCh := WatchRemote(ctx, client, "run-remote-002")
	for range eventsCh {
		t.Fatal("expected no replayed events for heartbeat-only stream")
	}
	for err := range errsCh {
		if err != nil {
			t.Fatalf("unexpected watch error: %v", err)
		}
	}

	if len(client.openAfter) != 1 {
		t.Fatalf("expected no reconnect once snapshot confirmed terminal state, got %d opens", len(client.openAfter))
	}
	if client.snapshotCalls == 0 {
		t.Fatal("expected terminal EOF path to consult the snapshot")
	}
}

func TestWatchRemoteRejectsMissingRunIDAndNilClient(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, errsCh := WatchRemote(ctx, nil, "run-remote-003")
	var err error
	for item := range errsCh {
		err = item
	}
	if err == nil || err.Error() != "watch remote run: daemon client is required" {
		t.Fatalf("unexpected nil-client error: %v", err)
	}

	_, errsCh = WatchRemote(ctx, &stubRemoteStreamClient{}, "   ")
	err = nil
	for item := range errsCh {
		err = item
	}
	if err == nil || err.Error() != "watch remote run: run id is required" {
		t.Fatalf("unexpected missing-run-id error: %v", err)
	}
}

func TestWatchRemoteReportsInitialOpenFailure(t *testing.T) {
	t.Parallel()

	client := &stubRemoteStreamClient{
		openErr: errors.New("boom"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, errsCh := WatchRemote(ctx, client, "run-remote-004")
	var err error
	for item := range errsCh {
		err = item
	}
	if err == nil || err.Error() != "open remote run stream: boom" {
		t.Fatalf("unexpected initial open error: %v", err)
	}
}

func TestShouldStopRemoteWatchRequiresTerminalSnapshot(t *testing.T) {
	t.Parallel()

	client := &stubRemoteStreamClient{
		snapshot: RemoteRunSnapshot{Status: "running"},
	}
	if shouldStopRemoteWatch(context.Background(), client, "run-remote-005", RemoteCursor{}) {
		t.Fatal("expected non-terminal snapshot to keep reconnecting")
	}
	if isTerminalRemoteRunStatus("running") {
		t.Fatal("expected running status to be non-terminal")
	}
	items, errs := remoteRunStreamChannels(nil)
	if items != nil || errs != nil {
		t.Fatalf("expected nil channels for nil stream, got items=%v errs=%v", items, errs)
	}
}

type stubRemoteStreamClient struct {
	mu            sync.Mutex
	streams       []RemoteRunStream
	openAfter     []RemoteCursor
	openErr       error
	snapshot      RemoteRunSnapshot
	snapshotErr   error
	snapshotCalls int
}

func (c *stubRemoteStreamClient) GetRunSnapshot(_ context.Context, _ string) (RemoteRunSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshotCalls++
	if c.snapshotErr != nil {
		return RemoteRunSnapshot{}, c.snapshotErr
	}
	return c.snapshot, nil
}

func (c *stubRemoteStreamClient) OpenRunStream(
	_ context.Context,
	_ string,
	after RemoteCursor,
) (RemoteRunStream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.openAfter = append(c.openAfter, after)
	if c.openErr != nil {
		return nil, c.openErr
	}
	if len(c.streams) == 0 {
		return nil, errors.New("unexpected stream request")
	}
	stream := c.streams[0]
	c.streams = c.streams[1:]
	return stream, nil
}
