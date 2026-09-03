package wire

import (
	"fmt"
	"sync"
	"time"
)

const slowConsumerTimeout = 10 * time.Second

type QueueOptions struct {
	Flow        Flow
	Now         func() time.Time
	SlowTimeout time.Duration
	Demoted     func(reason string)
	Evicted     func(reason string)
}

type queuedFrame struct {
	frame     Frame
	end       uint64
	size      int
	credit    int
	sequenced bool
}

type Queue struct {
	group     *Group
	now       func() time.Time
	slowAfter time.Duration
	onDemoted func(string)
	onEvicted func(string)

	mu           sync.Mutex
	flow         Flow
	items        []queuedFrame
	inflight     *queuedFrame
	queuedBytes  int
	pendingAck   int
	deliveredAck int
	high         bool
	closed       bool
	closeReason  string
	closing      bool
	fullSince    time.Time
	ackSlowTimer *time.Timer
	wake         chan struct{}
	out          chan Frame
	done         chan struct{}
}

func NewQueue(options QueueOptions) *Queue {
	flow := options.Flow
	if flow != FlowAck {
		flow = FlowDrop
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	slowAfter := options.SlowTimeout
	if slowAfter <= 0 {
		slowAfter = slowConsumerTimeout
	}
	queue := &Queue{
		flow: flow, now: now, slowAfter: slowAfter,
		onDemoted: options.Demoted, onEvicted: options.Evicted,
		wake: make(chan struct{}, 1), out: make(chan Frame), done: make(chan struct{}),
	}
	go queue.deliver()
	return queue
}

func (q *Queue) Frames() <-chan Frame { return q.out }

func (q *Queue) CloseReason() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closeReason
}

func (q *Queue) Flow() Flow {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.flow
}

func (q *Queue) PendingBytes() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pendingAck
}

func (q *Queue) Enqueue(frame Frame, end uint64) {
	demoted := false
	q.mu.Lock()
	if q.closed || q.closing {
		q.mu.Unlock()
		return
	}
	size := frameBytes(frame)
	credit := outputBytes(frame)
	q.items = append(q.items, queuedFrame{
		frame: cloneFrame(frame), end: end, size: size, credit: credit, sequenced: end > frame.Seq,
	})
	q.queuedBytes += size
	if q.flow == FlowAck {
		q.pendingAck += credit
		if !q.high && q.pendingAck >= AckHighWatermark {
			q.high = true
			q.startAckSlowTimerLocked()
		}
		if q.pendingAck > AckPendingLimit || q.queuedBytes > AckPendingLimit {
			demoted = q.demoteLocked()
		}
	}
	if q.flow == FlowDrop {
		q.trimDropQueueLocked()
	}
	q.notifyLocked()
	q.mu.Unlock()
	q.signalGroup()
	if demoted {
		q.notifyDemoted()
	}
}

func (q *Queue) Ack(bytes int) {
	if bytes <= 0 {
		return
	}
	q.mu.Lock()
	if !q.closed && q.flow == FlowAck {
		credited := min(bytes, q.deliveredAck)
		q.deliveredAck -= credited
		q.pendingAck = max(0, q.pendingAck-credited)
		if q.high && q.pendingAck <= AckLowWatermark {
			q.high = false
			q.stopAckSlowTimerLocked()
		}
	}
	q.mu.Unlock()
	q.signalGroup()
}

func (q *Queue) Close() {
	q.mu.Lock()
	q.closed = true
	q.stopAckSlowTimerLocked()
	q.notifyLocked()
	q.mu.Unlock()
	q.signalGroup()
	<-q.done
}

func (q *Queue) Finish() {
	q.mu.Lock()
	if !q.closed {
		q.closing = true
		q.notifyLocked()
	}
	q.mu.Unlock()
	q.signalGroup()
}

func (q *Queue) flowState() (Flow, bool, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.flow, q.high, q.closed
}

func (q *Queue) deliver() {
	defer close(q.done)
	defer close(q.out)
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return
		}
		if q.inflight == nil && len(q.items) == 0 {
			if q.closing {
				q.closed = true
				q.mu.Unlock()
				return
			}
			q.mu.Unlock()
			<-q.wake
			continue
		}
		if q.inflight == nil {
			item := q.items[0]
			q.items = q.items[1:]
			q.queuedBytes -= item.size
			q.inflight = &item
		}
		item := *q.inflight
		q.mu.Unlock()
		select {
		case q.out <- item.frame:
			q.mu.Lock()
			q.inflight = nil
			if q.flow == FlowAck {
				q.deliveredAck += item.credit
			}
			if q.queuedBytes < DropQueueLimit {
				q.fullSince = time.Time{}
			}
			q.mu.Unlock()
		case <-q.wake:
		}
	}
}

func (q *Queue) demoteLocked() bool {
	if q.flow != FlowAck {
		return false
	}
	q.flow = FlowDrop
	q.high = false
	q.stopAckSlowTimerLocked()
	q.pendingAck = 0
	q.deliveredAck = 0
	var from, to, dropped uint64
	sequenced := false
	for _, item := range q.items {
		if item.sequenced && !sequenced {
			from = item.frame.Seq
			sequenced = true
		}
		if item.sequenced {
			to = item.end
		}
		// #nosec G115 -- credit is the nonnegative encoded output length.
		dropped += uint64(item.credit)
	}
	q.items = nil
	q.queuedBytes = 0
	if sequenced {
		q.appendGapLocked(from, to, dropped)
	}
	return true
}

func (q *Queue) trimDropQueueLocked() {
	if q.queuedBytes <= DropQueueLimit {
		return
	}
	now := q.now()
	if q.fullSince.IsZero() {
		q.fullSince = now
	}
	var from, to uint64
	var dropped uint64
	sequenced := false
	for q.queuedBytes > DropQueueLimit && len(q.items) > 0 {
		item := q.items[0]
		q.items = q.items[1:]
		q.queuedBytes -= item.size
		if item.sequenced && !sequenced {
			from = item.frame.Seq
			sequenced = true
		}
		if item.sequenced {
			to = item.end
		}
		// #nosec G115 -- credit is the nonnegative encoded output length.
		dropped += uint64(item.credit)
	}
	if sequenced {
		q.appendGapLocked(from, to, dropped)
	}
	if now.Sub(q.fullSince) >= q.slowAfter {
		q.closed = true
		q.closeReason = "slow_consumer"
		if q.onEvicted != nil {
			go q.onEvicted("slow_consumer")
		}
	}
}

func (q *Queue) startAckSlowTimerLocked() {
	q.stopAckSlowTimerLocked()
	q.ackSlowTimer = time.AfterFunc(q.slowAfter, q.demoteSlowAck)
}

func (q *Queue) stopAckSlowTimerLocked() {
	if q.ackSlowTimer != nil {
		q.ackSlowTimer.Stop()
		q.ackSlowTimer = nil
	}
}

func (q *Queue) demoteSlowAck() {
	q.mu.Lock()
	demoted := !q.closed && q.high && q.demoteLocked()
	if demoted {
		q.notifyLocked()
	}
	q.mu.Unlock()
	if !demoted {
		return
	}
	q.signalGroup()
	q.notifyDemoted()
}

func (q *Queue) notifyDemoted() {
	if q.onDemoted != nil {
		q.onDemoted("demoted")
	}
}

func (q *Queue) appendGapLocked(from, to, dropped uint64) {
	payload := []byte(fmt.Sprintf(
		`{"dropped_bytes":%d,"from_seq":"%d","to_seq":"%d"}`,
		dropped, from, to,
	))
	frame := Frame{Op: ServerOpGap, Payload: payload}
	size := frameBytes(frame)
	q.items = append([]queuedFrame{{frame: frame, size: size}}, q.items...)
	q.queuedBytes += size
}

func (q *Queue) notifyLocked() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *Queue) signalGroup() {
	if q.group != nil {
		q.group.signal()
	}
}

func outputBytes(frame Frame) int {
	if frame.Op != ServerOpOutput {
		return 0
	}
	return len(frame.Payload)
}

func frameBytes(frame Frame) int {
	return 1 + 8 + len(frame.Payload)
}

func cloneFrame(frame Frame) Frame {
	frame.Payload = append([]byte(nil), frame.Payload...)
	return frame
}
