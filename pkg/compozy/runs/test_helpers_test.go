package runs

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

type stubDaemonRunReader struct {
	mu sync.Mutex

	openSummary   RunSummary
	openErr       error
	openCalls     []string
	openWorkspace []string

	listSummaries  [][]RunSummary
	listErrs       []error
	listCalls      []ListOptions
	listWorkspaces []string

	snapshot     RemoteRunSnapshot
	snapshotErr  error
	snapshotByID map[string]RemoteRunSnapshot

	eventPages   []remoteRunEventPage
	eventPageErr error
	eventCalls   []RemoteCursor

	streams    []RemoteRunStream
	streamErr  error
	streamOpen []RemoteCursor
}

func (s *stubDaemonRunReader) OpenRun(_ context.Context, workspaceRoot string, runID string) (RunSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openWorkspace = append(s.openWorkspace, workspaceRoot)
	s.openCalls = append(s.openCalls, runID)
	if s.openErr != nil {
		return RunSummary{}, s.openErr
	}
	return s.openSummary, nil
}

func (s *stubDaemonRunReader) ListRuns(
	_ context.Context,
	workspaceRoot string,
	opts ListOptions,
) ([]RunSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listWorkspaces = append(s.listWorkspaces, workspaceRoot)
	s.listCalls = append(s.listCalls, opts)
	if len(s.listErrs) > 0 {
		err := s.listErrs[0]
		s.listErrs = s.listErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(s.listSummaries) == 0 {
		return nil, nil
	}
	result := append([]RunSummary(nil), s.listSummaries[0]...)
	if len(s.listSummaries) > 1 {
		s.listSummaries = s.listSummaries[1:]
	}
	return result, nil
}

func (s *stubDaemonRunReader) GetRunSnapshot(_ context.Context, runID string) (RemoteRunSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshotErr != nil {
		return RemoteRunSnapshot{}, s.snapshotErr
	}
	if s.snapshotByID != nil {
		if snapshot, ok := s.snapshotByID[runID]; ok {
			return snapshot, nil
		}
	}
	return s.snapshot, nil
}

func (s *stubDaemonRunReader) ListRunEvents(
	_ context.Context,
	_ string,
	after RemoteCursor,
	_ int,
) (remoteRunEventPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventCalls = append(s.eventCalls, after)
	if s.eventPageErr != nil {
		return remoteRunEventPage{}, s.eventPageErr
	}
	if len(s.eventPages) == 0 {
		return remoteRunEventPage{}, nil
	}
	page := s.eventPages[0]
	if len(s.eventPages) > 1 {
		s.eventPages = s.eventPages[1:]
	}
	return page, nil
}

func (s *stubDaemonRunReader) OpenRunStream(
	_ context.Context,
	_ string,
	after RemoteCursor,
) (RemoteRunStream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamOpen = append(s.streamOpen, after)
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	if len(s.streams) == 0 {
		return nil, errors.New("unexpected stream request")
	}
	stream := s.streams[0]
	s.streams = s.streams[1:]
	return stream, nil
}

func withStubDaemonRunReader(t *testing.T, reader daemonRunReader) {
	t.Helper()
	previous := resolveRunsDaemonReader
	resolveRunsDaemonReader = func() (daemonRunReader, error) {
		return reader, nil
	}
	t.Cleanup(func() {
		resolveRunsDaemonReader = previous
	})
}

func collectReplay(run *Run, fromSeq uint64) ([]events.Event, []error) {
	var (
		gotEvents []events.Event
		gotErrs   []error
	)
	for ev, err := range run.Replay(fromSeq) {
		if err != nil {
			gotErrs = append(gotErrs, err)
			continue
		}
		gotEvents = append(gotEvents, ev)
	}
	return gotEvents, gotErrs
}

func collectTailEvents(
	t *testing.T,
	eventsCh <-chan events.Event,
	errsCh <-chan error,
	want int,
	timeout time.Duration,
) []events.Event {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	got := make([]events.Event, 0, want)
	for len(got) < want {
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d tail events, got %d", want, len(got))
		case err, ok := <-errsCh:
			if ok && err != nil {
				t.Fatalf("unexpected tail error: %v", err)
			}
		case ev, ok := <-eventsCh:
			if !ok {
				t.Fatalf("tail events channel closed after %d events, want %d", len(got), want)
			}
			got = append(got, ev)
		}
	}
	return got
}

func awaitRunEvent(t *testing.T, eventsCh <-chan RunEvent, errsCh <-chan error, timeout time.Duration) RunEvent {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-deadline.C:
			t.Fatal("timed out waiting for run event")
		case err, ok := <-errsCh:
			if ok && err != nil {
				t.Fatalf("unexpected watch error: %v", err)
			}
		case event, ok := <-eventsCh:
			if !ok {
				t.Fatal("run events channel closed before event arrived")
			}
			return event
		}
	}
}

func collectedSeqs(items []events.Event) []uint64 {
	seqs := make([]uint64, 0, len(items))
	for i := range items {
		seqs = append(seqs, items[i].Seq)
	}
	return seqs
}

func testEvent(runID string, seq uint64, kind events.EventKind) events.Event {
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         runID,
		Seq:           seq,
		Timestamp:     time.Unix(int64(seq), 0).UTC(),
		Kind:          kind,
	}
}

func waitForEventChannelClose[T any](t *testing.T, ch <-chan T, name string, timeout time.Duration) {
	t.Helper()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("%s channel still open", name)
		}
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s channel to close", name)
	}
}

func remoteCursor(timestamp time.Time, seq uint64) *RemoteCursor {
	return &RemoteCursor{
		Timestamp: timestamp.UTC(),
		Sequence:  seq,
	}
}

func mustDeepEqual[T any](t *testing.T, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

type bufferedRemoteRunStream struct {
	items chan RemoteRunStreamItem
	errs  chan error
}

func newBufferedRemoteRunStream(items ...RemoteRunStreamItem) *bufferedRemoteRunStream {
	stream := &bufferedRemoteRunStream{
		items: make(chan RemoteRunStreamItem, len(items)),
		errs:  make(chan error),
	}
	for _, item := range items {
		stream.items <- item
	}
	close(stream.items)
	close(stream.errs)
	return stream
}

func (s *bufferedRemoteRunStream) Items() <-chan RemoteRunStreamItem {
	if s == nil {
		return nil
	}
	return s.items
}

func (s *bufferedRemoteRunStream) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errs
}

func (s *bufferedRemoteRunStream) Close() error {
	return nil
}
