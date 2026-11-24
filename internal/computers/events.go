package computers

import (
	"context"
	"strconv"

	"github.com/compozy/compozy/internal/remoteproto"
)

const replayLimit = 512

type Change struct {
	ComputerID remoteproto.ComputerID
	Computer   *View
	Removed    bool
}

type Event struct {
	ID    string
	Event string
	Data  Change
}

type Subscription struct {
	Replay []Event
	Events <-chan Event
	Done   <-chan struct{}
	Close  func()
}

type eventRing struct {
	first, sequence uint64
	events          [replayLimit]Event
	subscribers     map[chan Event]struct{}
}

func newEventRing(generation int64) eventRing {
	var base uint64
	if generation > 0 {
		base = uint64(generation) << 32
	}
	return eventRing{first: base, sequence: base, subscribers: make(map[chan Event]struct{})}
}

func (r *Registry) Subscribe(ctx context.Context, after string) (Subscription, error) {
	s, err := r.current()
	if err != nil {
		return Subscription{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return Subscription{}, err
	}
	if err := ctx.Err(); err != nil {
		return Subscription{}, err
	}
	updates := make(chan Event, 32)
	s.events.subscribers[updates] = struct{}{}
	return Subscription{
		Replay: s.events.replay(after), Events: updates, Done: s.ctx.Done(), Close: func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.events.remove(updates)
		},
	}, nil
}

func (r *eventRing) replay(after string) []Event {
	if after == "" {
		return nil
	}
	cursor, err := strconv.ParseUint(after, 10, 64)
	if err != nil || cursor < r.first || cursor > r.sequence || r.sequence-cursor > replayLimit {
		return []Event{{ID: strconv.FormatUint(r.sequence, 10), Event: "stream_reset"}}
	}
	result := make([]Event, 0, r.sequence-cursor)
	for seq := cursor + 1; seq <= r.sequence; seq++ {
		result = append(result, r.events[seq%replayLimit])
	}
	return result
}

func (r *eventRing) publish(change Change) {
	r.sequence++
	event := Event{ID: strconv.FormatUint(r.sequence, 10), Event: "computer.changed", Data: change}
	r.events[r.sequence%replayLimit] = event
	for sub := range r.subscribers {
		select {
		case sub <- event:
		default:
			r.remove(sub)
		}
	}
}

func (r *eventRing) remove(sub chan Event) {
	if _, exists := r.subscribers[sub]; exists {
		delete(r.subscribers, sub)
		close(sub)
	}
}

func (s *registryScope) changed(view View) {
	s.events.publish(Change{ComputerID: view.ID, Computer: &view})
}
