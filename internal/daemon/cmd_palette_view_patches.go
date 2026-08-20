package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/compozy/compozy/internal/cmdpalette"
)

const (
	viewPatchRetainLimit          = 32
	viewPatchSubscriberBufferSize = 32
)

type viewPatchHub struct {
	mu      sync.Mutex
	closed  bool
	nextID  uint64
	streams map[string]*viewPatchStream
}

type viewPatchStream struct {
	epoch       string
	nextSeq     int64
	log         []cmdpalette.ViewPatchEvent
	subscribers map[uint64]chan cmdpalette.ViewPatchEvent
}

func newViewPatchHub() *viewPatchHub {
	return &viewPatchHub{streams: make(map[string]*viewPatchStream)}
}

func viewPatchStreamKey(workspaceID cmdpalette.WorkspaceID, viewID string) string {
	return string(workspaceID) + "\x00" + viewID
}

func (h *viewPatchHub) publish(
	workspaceID cmdpalette.WorkspaceID,
	patch cmdpalette.ViewPatch,
	epoch string,
) (cmdpalette.ViewPatchEvent, error) {
	if h == nil {
		return cmdpalette.ViewPatchEvent{}, errors.New("daemon: view patch hub is unavailable")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return cmdpalette.ViewPatchEvent{}, errors.New("daemon: view patch hub is closed")
	}
	stream := h.ensureStreamLocked(workspaceID, strings.TrimSpace(patch.ViewID), epoch)
	stream.nextSeq++
	event := cmdpalette.ViewPatchEvent{
		Sequence: stream.nextSeq, StreamEpoch: stream.epoch, Patch: cloneViewPatch(patch),
	}
	stream.log = append(stream.log, event)
	if overflow := len(stream.log) - viewPatchRetainLimit; overflow > 0 {
		stream.log = append([]cmdpalette.ViewPatchEvent(nil), stream.log[overflow:]...)
	}
	for _, updates := range stream.subscribers {
		select {
		case updates <- event:
		default:
		}
	}
	return event, nil
}

func (h *viewPatchHub) subscribe(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
	viewID string,
	after int64,
	epoch string,
) (<-chan cmdpalette.ViewPatchEvent, func(), error) {
	if ctx == nil {
		return nil, nil, errors.New("daemon: view patch subscription context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if h == nil {
		return nil, nil, errors.New("daemon: view patch hub is unavailable")
	}
	if after < 0 {
		return nil, nil, errors.New("daemon: view patch cursor after cannot be negative")
	}
	viewID = strings.TrimSpace(viewID)
	if workspaceID == "" || viewID == "" {
		return nil, nil, errors.New("daemon: view patch workspace and view are required")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, nil, errors.New("daemon: view patch hub is closed")
	}
	stream := h.ensureStreamLocked(workspaceID, viewID, epoch)
	replay := stream.replayLocked(after, epoch)
	updates := make(chan cmdpalette.ViewPatchEvent, viewPatchSubscriberBufferSize)
	for _, event := range replay {
		updates <- event
	}
	h.nextID++
	id := h.nextID
	stream.subscribers[id] = updates
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.removeSubscriber(workspaceID, viewID, id, updates)
		})
	}
	return updates, cancel, nil
}

func (h *viewPatchHub) close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, stream := range h.streams {
		for id, updates := range stream.subscribers {
			close(updates)
			delete(stream.subscribers, id)
		}
	}
}

func (h *viewPatchHub) ensureStreamLocked(
	workspaceID cmdpalette.WorkspaceID,
	viewID string,
	epoch string,
) *viewPatchStream {
	key := viewPatchStreamKey(workspaceID, viewID)
	if stream := h.streams[key]; stream != nil {
		return stream
	}
	epoch = strings.TrimSpace(epoch)
	if epoch == "" {
		epoch = "vse_" + uuid.NewString()
	}
	stream := &viewPatchStream{
		epoch: epoch, subscribers: make(map[uint64]chan cmdpalette.ViewPatchEvent),
	}
	h.streams[key] = stream
	return stream
}

func (h *viewPatchHub) removeSubscriber(
	workspaceID cmdpalette.WorkspaceID,
	viewID string,
	id uint64,
	updates chan cmdpalette.ViewPatchEvent,
) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stream := h.streams[viewPatchStreamKey(workspaceID, viewID)]
	if stream == nil {
		return
	}
	current, ok := stream.subscribers[id]
	if !ok || current != updates {
		return
	}
	delete(stream.subscribers, id)
	close(updates)
}

func (s *viewPatchStream) replayLocked(after int64, epoch string) []cmdpalette.ViewPatchEvent {
	if after > 0 && strings.TrimSpace(epoch) != "" && epoch != s.epoch {
		return nil
	}
	replay := make([]cmdpalette.ViewPatchEvent, 0, len(s.log))
	for _, event := range s.log {
		if event.Sequence > after {
			replay = append(replay, event)
		}
	}
	return replay
}

func cloneViewPatch(patch cmdpalette.ViewPatch) cmdpalette.ViewPatch {
	cloned := patch
	if patch.Ops == nil {
		return cloned
	}
	cloned.Ops = make([]cmdpalette.PatchOp, len(patch.Ops))
	for index, operation := range patch.Ops {
		cloned.Ops[index] = cmdpalette.PatchOp{
			Op: operation.Op, Path: operation.Path, Value: append(json.RawMessage(nil), operation.Value...),
		}
	}
	return cloned
}
