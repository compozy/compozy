package monitor

import (
	"context"
	"sync"
	"time"
)

// observerTimeout bounds one observer call so a slow reader never stalls the queue.
const observerTimeout = 5 * time.Second

// noticeQueue coalesces pending observer notices by session. A session appears
// at most once while pending, so bursts never drop its latest refresh and the
// queue stays bounded by the number of distinct sessions with changes.
type noticeQueue struct {
	mu      sync.Mutex
	order   []string
	pending map[string]struct{}
	ready   chan struct{}
}

func newNoticeQueue() *noticeQueue {
	return &noticeQueue{pending: make(map[string]struct{}), ready: make(chan struct{}, 1)}
}

func (q *noticeQueue) push(sessionID string) {
	q.mu.Lock()
	if _, ok := q.pending[sessionID]; !ok {
		q.pending[sessionID] = struct{}{}
		q.order = append(q.order, sessionID)
	}
	q.mu.Unlock()
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

func (q *noticeQueue) pop() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.order) == 0 {
		return "", false
	}
	sessionID := q.order[0]
	q.order[0] = ""
	q.order = q.order[1:]
	delete(q.pending, sessionID)
	return sessionID, true
}

// notify queues an observer notice for sessionID without blocking the caller,
// which may hold session-manager locks (a session stop cancels its monitors).
func (s *Service) notify(sessionID string) {
	if s.notices == nil {
		return
	}
	s.notices.push(sessionID)
}

// dispatchNotices delivers queued notices one at a time on a service-owned
// goroutine, each bounded by observerTimeout, until the service shuts down.
func (s *Service) dispatchNotices() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.notices.ready:
		}
		for s.ctx.Err() == nil {
			sessionID, ok := s.notices.pop()
			if !ok {
				break
			}
			callCtx, cancel := context.WithTimeout(s.ctx, observerTimeout)
			s.observer(callCtx, sessionID)
			cancel()
		}
	}
}
