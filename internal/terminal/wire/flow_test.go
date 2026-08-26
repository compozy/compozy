package wire

// Suite: terminal wire flow control.
// Invariant: ack and drop subscribers remain bounded, independent, and observable under backpressure.
// Boundary IN: queued output and ACK credit. Boundary OUT: delivered frames, gaps, demotion, and eviction.

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestFlowGroupShouldPauseOnlyWhenEveryAckSubscriberIsHigh(t *testing.T) {
	t.Parallel()
	if AckGrainBytes > AckLowWatermark {
		t.Fatalf("AckGrainBytes = %d, must be <= low watermark %d", AckGrainBytes, AckLowWatermark)
	}
	group := NewGroup()
	first := NewQueue(QueueOptions{Flow: FlowAck})
	second := NewQueue(QueueOptions{Flow: FlowAck})
	group.Add(first)
	group.Add(second)
	t.Cleanup(func() { first.Close(); second.Close() })
	payload := make([]byte, AckHighWatermark+1)
	first.Enqueue(Frame{Op: ServerOpOutput, Seq: 0, Payload: payload}, uint64(len(payload)))
	receiveFrame(t, first)
	if err := group.WaitProducer(context.Background()); err != nil {
		t.Fatalf("WaitProducer() with one healthy subscriber error = %v", err)
	}
	second.Enqueue(Frame{Op: ServerOpOutput, Seq: 0, Payload: payload}, uint64(len(payload)))
	receiveFrame(t, second)
	unblocked := make(chan error, 1)
	go func() { unblocked <- group.WaitProducer(context.Background()) }()
	select {
	case err := <-unblocked:
		t.Fatalf("WaitProducer() returned while every ack subscriber was high: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	first.Ack(AckHighWatermark)
	select {
	case err := <-unblocked:
		if err != nil {
			t.Fatalf("WaitProducer() after low watermark error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitProducer() did not resume below low watermark")
	}
}

func TestFlowQueueShouldBoundDropAndDemoteAck(t *testing.T) {
	t.Parallel()

	t.Run("Should emit a gap when a drop queue overflows", func(t *testing.T) {
		t.Parallel()
		queue := NewQueue(QueueOptions{Flow: FlowDrop})
		t.Cleanup(queue.Close)
		for index := 0; index < 4; index++ {
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
			From    uint64 `json:"from_seq"`
			To      uint64 `json:"to_seq"`
			Dropped uint64 `json:"dropped_bytes"`
		}
		if err := json.Unmarshal(frame.Payload, &gap); err != nil {
			t.Fatalf("decode demotion GAP: %v", err)
		}
		if gap.From != 7 || gap.To != 7+uint64(len(payload)) || gap.Dropped != uint64(len(payload)) {
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
		queue.Enqueue(Frame{Op: ServerOpOutput, Seq: start, Payload: make([]byte, DropQueueLimit+1)}, start+DropQueueLimit+1)
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
	group := NewGroup()
	demoted := make(chan string, 1)
	queue := NewQueue(QueueOptions{
		Flow: FlowAck, SlowTimeout: 20 * time.Millisecond,
		Demoted: func(reason string) { demoted <- reason },
	})
	group.Add(queue)
	t.Cleanup(queue.Close)
	payload := make([]byte, AckHighWatermark)
	queue.Enqueue(Frame{Op: ServerOpOutput, Payload: payload}, uint64(len(payload)))
	receiveFrame(t, queue)
	unblocked := make(chan error, 1)
	go func() { unblocked <- group.WaitProducer(context.Background()) }()
	select {
	case err := <-unblocked:
		t.Fatalf("producer returned before slow timeout: %v", err)
	case <-time.After(5 * time.Millisecond):
	}
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
	select {
	case err := <-unblocked:
		if err != nil {
			t.Fatalf("producer after timeout demotion error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("producer did not resume after timeout demotion")
	}
}

func TestFlowQueueShouldEvictAContinuouslyFullObserver(t *testing.T) {
	t.Parallel()
	now := time.Unix(100, 0)
	evicted := make(chan string, 1)
	queue := NewQueue(QueueOptions{Flow: FlowDrop, Now: func() time.Time { return now }, Evicted: func(reason string) {
		evicted <- reason
	}})
	t.Cleanup(queue.Close)
	queue.Enqueue(Frame{Op: ServerOpOutput, Payload: make([]byte, DropQueueLimit+1)}, DropQueueLimit+1)
	now = now.Add(11 * time.Second)
	queue.Enqueue(Frame{Op: ServerOpOutput, Seq: DropQueueLimit + 1, Payload: make([]byte, DropQueueLimit+1)}, 2*(DropQueueLimit+1))
	select {
	case reason := <-evicted:
		if reason != "slow_consumer" {
			t.Fatalf("eviction reason = %q", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("full observer was not evicted")
	}
}

func TestFlowQueueShouldAllowImmediateCloseAfterGracefulFinish(t *testing.T) {
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
}

func TestFlowGroupShouldKeepHealthyAckAndDropSubscribersIndependentUnderFlood(t *testing.T) {
	t.Parallel()
	group := NewGroup()
	healthy := NewQueue(QueueOptions{Flow: FlowAck})
	stalled := NewQueue(QueueOptions{Flow: FlowAck})
	drops := []*Queue{
		NewQueue(QueueOptions{Flow: FlowDrop}),
		NewQueue(QueueOptions{Flow: FlowDrop}),
		NewQueue(QueueOptions{Flow: FlowDrop}),
	}
	group.Add(healthy)
	group.Add(stalled)
	for _, queue := range drops {
		group.Add(queue)
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
	for index := 0; index < chunks; index++ {
		start := uint64(index * chunkBytes)
		frame := Frame{Op: ServerOpOutput, Seq: start, Payload: make([]byte, chunkBytes)}
		healthy.Enqueue(frame, start+chunkBytes)
		received := receiveFrame(t, healthy)
		delivered += len(received.Payload)
		healthy.Ack(len(received.Payload))
		stalled.Enqueue(frame, start+chunkBytes)
		for _, queue := range drops {
			queue.Enqueue(frame, start+chunkBytes)
		}
		if err := group.WaitProducer(context.Background()); err != nil {
			t.Fatalf("WaitProducer(%d) error = %v", index, err)
		}
	}
	if delivered != chunks*chunkBytes {
		t.Fatalf("healthy ack bytes = %d, want %d", delivered, chunks*chunkBytes)
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
