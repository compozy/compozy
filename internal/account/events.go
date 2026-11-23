package account

import (
	"context"
	"strconv"
	"time"
)

const accountReplayLimit = 512

type Event struct {
	ID    string `json:"id"`
	Event string `json:"event"`
	Data  Change `json:"data"`
}

type Subscription struct {
	Replay []Event
	Events <-chan Event
	Done   <-chan struct{}
	Close  func()
}

type generation struct {
	ctx            context.Context
	cancel         context.CancelFunc
	base, sequence uint64
	rendered       [accountReplayLimit]Event
	subscribers    map[chan Event]struct{}
	feed           *cloudFeed
	pending        map[string]time.Time
	results        map[string]seenResult
}

type seenResult struct {
	value string
	at    time.Time
}

func newGeneration(parent context.Context, id int64) *generation {
	ctx, cancel := context.WithCancel(parent)
	// The high bits bind the numeric SSE cursor to the durable auth generation.
	base := uint64(id) << 32
	return &generation{
		ctx:      ctx,
		cancel:   cancel,
		base:     base,
		sequence: base,
		subscribers: make(
			map[chan Event]struct{},
		),
		pending: make(map[string]time.Time),
		results: make(map[string]seenResult),
	}
}

func (s *Service) Subscribe(after string) (Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.installation.State != SignedIn {
		return Subscription{}, ErrSignInRequired
	}
	g := s.generation
	events := make(chan Event, 32)
	g.subscribers[events] = struct{}{}
	s.updateFeedLocked(g)
	return Subscription{Replay: g.replay(after), Events: events, Done: g.ctx.Done(), Close: func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.removeSubscriberLocked(g, events)
	}}, nil
}

func (g *generation) replay(after string) []Event {
	if after == "" {
		return nil
	}
	cursor, err := strconv.ParseUint(after, 10, 64)
	if err != nil || cursor < g.base || cursor > g.sequence || g.sequence-cursor > accountReplayLimit {
		return []Event{{ID: strconv.FormatUint(g.sequence, 10), Event: "stream_reset"}}
	}
	result := make([]Event, 0, g.sequence-cursor)
	for seq := cursor + 1; seq <= g.sequence; seq++ {
		result = append(result, g.rendered[seq%accountReplayLimit])
	}
	return result
}

func (s *Service) removeSubscriberLocked(g *generation, events chan Event) {
	if _, open := g.subscribers[events]; !open {
		return
	}
	delete(g.subscribers, events)
	close(events)
	s.updateFeedLocked(g)
}

func (s *Service) emitLocked(g *generation, change Change) {
	if s.closed || g != s.generation {
		return
	}
	g.sequence++
	event := Event{ID: strconv.FormatUint(g.sequence, 10), Event: "account.changed", Data: change}
	g.rendered[g.sequence%accountReplayLimit] = event
	for sub := range g.subscribers {
		select {
		case sub <- event:
		default:
			s.removeSubscriberLocked(g, sub)
		}
	}
}

func (s *Service) SubscribeSignIn() (<-chan SignInEvent, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stream := make(chan SignInEvent, 32)
	if s.closed {
		close(stream)
	} else {
		s.progress[stream] = struct{}{}
	}
	return stream, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closeProgressLocked(stream)
	}
}

func (s *Service) sendProgressLocked(event SignInEvent) {
	for sub := range s.progress {
		select {
		case sub <- event:
		default:
			s.closeProgressLocked(sub)
		}
	}
}

func (s *Service) closeProgressLocked(sub chan SignInEvent) {
	if _, open := s.progress[sub]; open {
		delete(s.progress, sub)
		close(sub)
	}
}
