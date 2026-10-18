package wire

// Suite: terminal wire flow control.
// Invariant: ack and drop subscribers remain bounded, independent, and observable under backpressure.
// Boundary IN: queued output and ACK credit. Boundary OUT: delivered frames, gaps, demotion, and eviction.

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func TestFlowQueueShouldBoundDropAndDemoteAck(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver ATTACHED before a cumulative gap from an oversized initial replay", func(t *testing.T) {
		t.Parallel()
		for _, flow := range []Flow{FlowAck, FlowDrop} {
			t.Run(string(flow), func(t *testing.T) {
				t.Parallel()
				// Delay the delivery worker until replay has filled the queue.
				queue := &Queue{
					flow: flow, now: time.Now, slowAfter: time.Hour,
					wake: make(chan struct{}, 1), out: make(chan Frame), done: make(chan struct{}),
				}
				queue.Enqueue(Frame{Op: ServerOpAttached, Payload: []byte(`{"seq":"0"}`)}, 0)
				queue.Enqueue(Frame{Op: ServerOpOutput, Payload: make([]byte, AckPendingLimit+1)}, AckPendingLimit+1)
				queue.Enqueue(
					Frame{Op: ServerOpOutput, Seq: AckPendingLimit + 1, Payload: make([]byte, AckPendingLimit+1)},
					2*(AckPendingLimit+1),
				)
				go queue.deliver()
				t.Cleanup(queue.Close)
				if frame := receiveFrame(t, queue); frame.Op != ServerOpAttached {
					t.Fatalf("first opcode = 0x%02x, want ATTACHED", frame.Op)
				}
				frame := receiveFrame(t, queue)
				if frame.Op != ServerOpGap {
					t.Fatalf("second opcode = 0x%02x, want GAP", frame.Op)
				}
				var gap struct {
					From    string `json:"from_seq"`
					To      string `json:"to_seq"`
					Dropped uint64 `json:"dropped_bytes"`
				}
				if err := json.Unmarshal(frame.Payload, &gap); err != nil {
					t.Fatalf("decode cumulative GAP: %v", err)
				}
				if gap.From != "0" || gap.To != strconv.Itoa(2*(AckPendingLimit+1)) ||
					gap.Dropped != 2*(AckPendingLimit+1) {
					t.Fatalf("cumulative GAP = %#v, want full replay range and byte count", gap)
				}
			})
		}
	})

	t.Run("Should emit a gap when a drop queue overflows", func(t *testing.T) {
		t.Parallel()
		queue := NewQueue(QueueOptions{Flow: FlowDrop})
		t.Cleanup(queue.Close)
		for index := range 4 {
			start := uint64(index * (32 << 10))
			queue.Enqueue(Frame{Op: ServerOpOutput, Seq: start, Payload: make([]byte, 32<<10)}, start+(32<<10))
		}
		deadline := time.After(time.Second)
		for {
			select {
			case frame := <-queue.Frames():
				if frame.Op != ServerOpGap {
					continue
				}
				var gap struct {
					Dropped uint64 `json:"dropped_bytes"`
				}
				if err := json.Unmarshal(frame.Payload, &gap); err != nil {
					t.Fatalf("decode GAP: %v", err)
				}
				if gap.Dropped == 0 {
					t.Fatal("GAP dropped_bytes = 0")
				}
				return
			case <-deadline:
				t.Fatal("drop queue emitted no GAP")
			}
		}
	})

	t.Run("Should atomically demote an ack subscriber above one MiB", func(t *testing.T) {
		t.Parallel()
		demoted := make(chan string, 1)
		queue := NewQueue(QueueOptions{Flow: FlowAck, Demoted: func(reason string) { demoted <- reason }})
		t.Cleanup(queue.Close)
		payload := make([]byte, AckPendingLimit+1)
		queue.Enqueue(Frame{Op: ServerOpOutput, Seq: 7, Payload: payload}, 7+uint64(len(payload)))
		if queue.Flow() != FlowDrop {
			t.Fatalf("Flow() = %q, want drop", queue.Flow())
		}
		if queue.PendingBytes() != 0 {
			t.Fatalf("PendingBytes() = %d, want 0 after demotion", queue.PendingBytes())
		}
		select {
		case reason := <-demoted:
			if reason != "demoted" {
				t.Fatalf("demotion reason = %q", reason)
			}
		case <-time.After(time.Second):
			t.Fatal("demotion callback did not run")
		}
		frame := receiveFrame(t, queue)
		if frame.Op != ServerOpGap {
			t.Fatalf("demotion opcode = 0x%02x, want GAP", frame.Op)
		}
		var gap struct {
			From    string `json:"from_seq"`
			To      string `json:"to_seq"`
			Dropped uint64 `json:"dropped_bytes"`
		}
		if err := json.Unmarshal(frame.Payload, &gap); err != nil {
			t.Fatalf("decode demotion GAP: %v", err)
		}
		if gap.From != "7" || gap.To != strconv.FormatUint(7+uint64(len(payload)), 10) ||
			gap.Dropped != uint64(len(payload)) {
			t.Fatalf("demotion GAP = %#v", gap)
		}
	})

	t.Run("Should preserve an in-flight frame while trimming the drop queue", func(t *testing.T) {
		t.Parallel()
		queue := NewQueue(QueueOptions{Flow: FlowDrop})
		t.Cleanup(queue.Close)
		first := Frame{Op: ServerOpOutput, Seq: 0, Payload: []byte("first")}
		queue.Enqueue(first, uint64(len(first.Payload)))
		waitForQueueState(t, func() bool {
			queue.mu.Lock()
			selected := queue.inflight != nil
			queue.mu.Unlock()
			return selected
		}, "delivery worker to select the first frame")
		start := uint64(len(first.Payload))
		queue.Enqueue(
			Frame{Op: ServerOpOutput, Seq: start, Payload: make([]byte, DropQueueLimit+1)},
			start+DropQueueLimit+1,
		)
		select {
		case frame := <-queue.Frames():
			if string(frame.Payload) != "first" {
				t.Fatalf("first delivered payload = %q", frame.Payload)
			}
		case <-time.After(time.Second):
			t.Fatal("in-flight frame was not delivered")
		}
	})

	t.Run("Should ignore ACK credit for bytes not yet delivered", func(t *testing.T) {
		t.Parallel()
		queue := NewQueue(QueueOptions{Flow: FlowAck})
		t.Cleanup(queue.Close)
		payload := make([]byte, AckHighWatermark)
		queue.Enqueue(Frame{Op: ServerOpOutput, Payload: payload}, uint64(len(payload)))
		queue.Ack(AckHighWatermark)
		if pending := queue.PendingBytes(); pending != AckHighWatermark {
			t.Fatalf("PendingBytes() after early ACK = %d, want %d", pending, AckHighWatermark)
		}
		receiveFrame(t, queue)
		waitForQueueState(t, func() bool {
			queue.mu.Lock()
			delivered := queue.deliveredAck
			queue.mu.Unlock()
			return delivered == AckHighWatermark
		}, "delivered ACK credit to be recorded")
		queue.Ack(AckHighWatermark)
		if pending := queue.PendingBytes(); pending != 0 {
			t.Fatalf("PendingBytes() after delivered ACK = %d, want 0", pending)
		}
	})
}

func TestFlowQueueShouldDemoteAHighAckSubscriberAfterTimeout(t *testing.T) {
	t.Parallel()
	t.Run("Should demote a high ACK subscriber after timeout", func(t *testing.T) {
		t.Parallel()
		demoted := make(chan string, 1)
		queue := NewQueue(QueueOptions{
			Flow: FlowAck, SlowTimeout: 20 * time.Millisecond,
			Demoted: func(reason string) { demoted <- reason },
		})
		t.Cleanup(queue.Close)
		payload := make([]byte, AckHighWatermark)
		queue.Enqueue(Frame{Op: ServerOpOutput, Payload: payload}, uint64(len(payload)))
		receiveFrame(t, queue)
		select {
		case reason := <-demoted:
			if reason != "demoted" {
				t.Fatalf("demotion reason = %q", reason)
			}
		case <-time.After(time.Second):
			t.Fatal("high ack subscriber was not demoted after the slow timeout")
		}
		if queue.Flow() != FlowDrop || queue.PendingBytes() != 0 {
			t.Fatalf("flow/pending after timeout = %q/%d, want drop/0", queue.Flow(), queue.PendingBytes())
		}
	})
}

func TestFlowQueueShouldEvictAContinuouslyFullObserver(t *testing.T) {
	t.Parallel()
	t.Run("Should evict a continuously full observer", func(t *testing.T) {
		t.Parallel()
		now := time.Unix(100, 0)
		evicted := make(chan string, 1)
		queue := NewQueue(
			QueueOptions{Flow: FlowDrop, Now: func() time.Time { return now }, Evicted: func(reason string) {
				evicted <- reason
			}},
		)
		t.Cleanup(queue.Close)
		queue.Enqueue(Frame{Op: ServerOpOutput, Payload: make([]byte, DropQueueLimit+1)}, DropQueueLimit+1)
		now = now.Add(11 * time.Second)
		queue.Enqueue(
			Frame{Op: ServerOpOutput, Seq: DropQueueLimit + 1, Payload: make([]byte, DropQueueLimit+1)},
			2*(DropQueueLimit+1),
		)
		select {
		case reason := <-evicted:
			if reason != "slow_consumer" {
				t.Fatalf("eviction reason = %q", reason)
			}
		case <-time.After(time.Second):
			t.Fatal("full observer was not evicted")
		}
	})
}

func TestFlowQueueShouldAllowImmediateCloseAfterGracefulFinish(t *testing.T) {
	t.Parallel()
	t.Run("Should allow immediate close after graceful finish", func(t *testing.T) {
		t.Parallel()
		queue := NewQueue(QueueOptions{Flow: FlowDrop})
		queue.Enqueue(Frame{Op: ServerOpOutput, Payload: []byte("blocked")}, uint64(len("blocked")))
		queue.Finish()
		done := make(chan struct{})
		go func() {
			queue.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Close() could not interrupt a blocked graceful drain")
		}
	})
}

func TestFlowQueueShouldKeepHealthyAckAndDropSubscribersIndependentUnderFlood(t *testing.T) {
	t.Parallel()
	t.Run("Should keep healthy ACK and drop subscribers independent under flood", func(t *testing.T) {
		t.Parallel()
		healthy := NewQueue(QueueOptions{Flow: FlowAck})
		stalled := NewQueue(QueueOptions{Flow: FlowAck})
		drops := []*Queue{
			NewQueue(QueueOptions{Flow: FlowDrop}),
			NewQueue(QueueOptions{Flow: FlowDrop}),
			NewQueue(QueueOptions{Flow: FlowDrop}),
		}
		t.Cleanup(func() {
			healthy.Close()
			stalled.Close()
			for _, queue := range drops {
				queue.Close()
			}
		})
		const chunks = 40
		const chunkBytes = 32 << 10
		delivered := 0
		for index := range chunks {
			start := uint64(index * chunkBytes)
			frame := Frame{Op: ServerOpOutput, Seq: start, Payload: make([]byte, chunkBytes)}
			healthy.Enqueue(frame, start+chunkBytes)
			received := receiveFrame(t, healthy)
			delivered += len(received.Payload)
			waitForQueueState(t, func() bool {
				healthy.mu.Lock()
				defer healthy.mu.Unlock()
				return healthy.deliveredAck >= len(received.Payload)
			}, "delivered ACK credit")
			healthy.Ack(len(received.Payload))
			stalled.Enqueue(frame, start+chunkBytes)
			for _, queue := range drops {
				queue.Enqueue(frame, start+chunkBytes)
			}
		}
		if delivered != chunks*chunkBytes {
			t.Fatalf("healthy ack bytes = %d, want %d", delivered, chunks*chunkBytes)
		}
		if healthy.Flow() != FlowAck {
			t.Fatalf("healthy flow = %q, want ack", healthy.Flow())
		}
		if stalled.Flow() != FlowDrop || stalled.PendingBytes() != 0 {
			t.Fatalf("stalled flow/pending = %q/%d, want drop/0", stalled.Flow(), stalled.PendingBytes())
		}
		for index, queue := range drops {
			queue.mu.Lock()
			queuedBytes := queue.queuedBytes
			queue.mu.Unlock()
			if queuedBytes > DropQueueLimit {
				t.Fatalf("drop[%d] queued bytes = %d, want <= %d", index, queuedBytes, DropQueueLimit)
			}
			foundGap := false
			deadline := time.After(time.Second)
			for !foundGap {
				select {
				case frame := <-queue.Frames():
					foundGap = frame.Op == ServerOpGap
				case <-deadline:
					t.Fatalf("drop[%d] emitted no independent GAP", index)
				}
			}
		}
	})
}

func receiveFrame(t *testing.T, queue *Queue) Frame {
	t.Helper()
	select {
	case frame := <-queue.Frames():
		return frame
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queued frame")
		return Frame{}
	}
}

func waitForQueueState(t *testing.T, predicate func() bool, description string) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if predicate() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for %s", description)
		case <-ticker.C:
		}
	}
}
