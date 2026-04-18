package runs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

const (
	remoteWatchReconnectDelay = 100 * time.Millisecond
	publicRunStatusCrashed    = "crashed"
	daemonRunStatusCanceled   = "canceled"
)

// RemoteCursor is the package-local cursor shape used by daemon-backed readers.
type RemoteCursor struct {
	Timestamp time.Time
	Sequence  uint64
}

// RemoteRunSnapshot is the minimal snapshot state needed to decide whether a
// watch should reconnect after EOF.
type RemoteRunSnapshot struct {
	Status     string
	NextCursor *RemoteCursor
}

// RemoteRunStreamItem is one parsed stream delivery from a daemon-backed watch.
type RemoteRunStreamItem struct {
	Event           *events.Event
	HeartbeatCursor *RemoteCursor
	OverflowCursor  *RemoteCursor
}

// RemoteRunStream is the minimal replay-plus-live stream contract required by
// WatchRemote.
type RemoteRunStream interface {
	Items() <-chan RemoteRunStreamItem
	Errors() <-chan error
	Close() error
}

type remoteWatchState struct {
	currentStream RemoteRunStream
	itemCh        <-chan RemoteRunStreamItem
	errCh         <-chan error
	lastCursor    RemoteCursor
}

// RemoteStreamClient is the package-local daemon observation surface.
type RemoteStreamClient interface {
	GetRunSnapshot(context.Context, string) (RemoteRunSnapshot, error)
	OpenRunStream(context.Context, string, RemoteCursor) (RemoteRunStream, error)
}

// WatchRemote follows one daemon-backed run stream with cursor resume semantics.
func WatchRemote(ctx context.Context, client RemoteStreamClient, runID string) (<-chan events.Event, <-chan error) {
	out := make(chan events.Event)
	errs := make(chan error, 4)

	go func() {
		defer close(out)
		defer close(errs)

		if client == nil {
			sendRunError(ctx, errs, errors.New("watch remote run: daemon client is required"))
			return
		}

		trimmedRunID := strings.TrimSpace(runID)
		if trimmedRunID == "" {
			sendRunError(ctx, errs, errors.New("watch remote run: run id is required"))
			return
		}

		stream, err := client.OpenRunStream(ctx, trimmedRunID, RemoteCursor{})
		if err != nil {
			sendRunError(ctx, errs, fmt.Errorf("open remote run stream: %w", err))
			return
		}
		if stream == nil {
			sendRunError(ctx, errs, errors.New("open remote run stream: nil stream"))
			return
		}
		watchRemoteStreamLoop(ctx, client, trimmedRunID, stream, out)
	}()

	return out, errs
}

func watchRemoteStreamLoop(
	ctx context.Context,
	client RemoteStreamClient,
	runID string,
	stream RemoteRunStream,
	out chan<- events.Event,
) {
	state := newRemoteWatchState(stream)
	defer func() {
		closeRemoteWatchStream(state.currentStream)
	}()

	for {
		var stop bool
		state, stop = ensureRemoteWatchState(ctx, client, runID, state)
		if stop {
			return
		}
		if state.currentStream == nil {
			continue
		}

		state, stop = waitForRemoteWatchUpdate(ctx, client, runID, out, state)
		if stop {
			return
		}
	}
}

func newRemoteWatchState(stream RemoteRunStream) remoteWatchState {
	itemCh, errCh := remoteRunStreamChannels(stream)
	return remoteWatchState{
		currentStream: stream,
		itemCh:        itemCh,
		errCh:         errCh,
	}
}

func ensureRemoteWatchState(
	ctx context.Context,
	client RemoteStreamClient,
	runID string,
	state remoteWatchState,
) (remoteWatchState, bool) {
	if state.currentStream != nil {
		return state, false
	}
	if !sleepRemoteWatch(ctx, remoteWatchReconnectDelay) {
		return state, true
	}

	reconnected, err := client.OpenRunStream(ctx, runID, state.lastCursor)
	if err != nil || reconnected == nil {
		if shouldStopRemoteWatch(ctx, client, runID, state.lastCursor) {
			return state, true
		}
		return state, false
	}
	state.currentStream = reconnected
	state.itemCh, state.errCh = remoteRunStreamChannels(reconnected)
	return state, false
}

func waitForRemoteWatchUpdate(
	ctx context.Context,
	client RemoteStreamClient,
	runID string,
	out chan<- events.Event,
	state remoteWatchState,
) (remoteWatchState, bool) {
	select {
	case <-ctx.Done():
		return state, true
	case err, ok := <-state.errCh:
		return handleRemoteWatchError(ctx, client, runID, state, err, ok)
	case item, ok := <-state.itemCh:
		return handleRemoteWatchItem(ctx, client, runID, out, state, item, ok)
	}
}

func handleRemoteWatchError(
	ctx context.Context,
	client RemoteStreamClient,
	runID string,
	state remoteWatchState,
	err error,
	ok bool,
) (remoteWatchState, bool) {
	if !ok {
		state.errCh = nil
		if state.itemCh != nil {
			return state, false
		}
		return handleRemoteWatchEOF(ctx, client, runID, state)
	}
	if err == nil {
		return state, false
	}
	return resetRemoteWatchState(state), false
}

func handleRemoteWatchItem(
	ctx context.Context,
	client RemoteStreamClient,
	runID string,
	out chan<- events.Event,
	state remoteWatchState,
	item RemoteRunStreamItem,
	ok bool,
) (remoteWatchState, bool) {
	if !ok {
		state.itemCh = nil
		if state.errCh != nil {
			return state, false
		}
		return handleRemoteWatchEOF(ctx, client, runID, state)
	}

	if item.HeartbeatCursor != nil {
		state.lastCursor = maxRemoteWatchCursor(state.lastCursor, *item.HeartbeatCursor)
		return state, false
	}
	if item.OverflowCursor != nil {
		state.lastCursor = maxRemoteWatchCursor(state.lastCursor, *item.OverflowCursor)
		return resetRemoteWatchState(state), false
	}
	if item.Event == nil {
		return state, false
	}

	state.lastCursor = remoteCursorFromEvent(*item.Event)
	if !sendRunEvent(ctx, out, *item.Event) {
		return state, true
	}
	if isTerminalRemoteRunEvent(item.Event.Kind) {
		return state, true
	}
	return state, false
}

func handleRemoteWatchEOF(
	ctx context.Context,
	client RemoteStreamClient,
	runID string,
	state remoteWatchState,
) (remoteWatchState, bool) {
	state = resetRemoteWatchState(state)
	if shouldStopRemoteWatch(ctx, client, runID, state.lastCursor) {
		return state, true
	}
	return state, false
}

func resetRemoteWatchState(state remoteWatchState) remoteWatchState {
	closeRemoteWatchStream(state.currentStream)
	state.currentStream = nil
	state.itemCh = nil
	state.errCh = nil
	return state
}

func closeRemoteWatchStream(stream RemoteRunStream) {
	if stream == nil {
		return
	}
	_ = stream.Close()
}

func maxRemoteWatchCursor(current RemoteCursor, next RemoteCursor) RemoteCursor {
	if next.Sequence > current.Sequence {
		return next
	}
	return current
}

func sleepRemoteWatch(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func shouldStopRemoteWatch(
	ctx context.Context,
	client RemoteStreamClient,
	runID string,
	lastCursor RemoteCursor,
) bool {
	if client == nil {
		return false
	}
	snapshot, err := client.GetRunSnapshot(ctx, runID)
	if err != nil {
		return false
	}
	if !isTerminalRemoteRunStatus(snapshot.Status) {
		return false
	}
	if snapshot.NextCursor == nil {
		return true
	}
	return snapshot.NextCursor.Sequence <= lastCursor.Sequence
}

func isTerminalRemoteRunStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case publicRunStatusCompleted,
		publicRunStatusFailed,
		publicRunStatusCancelled,
		daemonRunStatusCanceled,
		publicRunStatusCrashed:
		return true
	default:
		return false
	}
}

func isTerminalRemoteRunEvent(kind events.EventKind) bool {
	switch kind {
	case events.EventKindRunCompleted, events.EventKindRunFailed, events.EventKindRunCancelled:
		return true
	default:
		return false
	}
}

func remoteRunStreamChannels(stream RemoteRunStream) (<-chan RemoteRunStreamItem, <-chan error) {
	if stream == nil {
		return nil, nil
	}
	return stream.Items(), stream.Errors()
}

func remoteCursorFromEvent(event events.Event) RemoteCursor {
	return RemoteCursor{
		Timestamp: event.Timestamp.UTC(),
		Sequence:  event.Seq,
	}
}
