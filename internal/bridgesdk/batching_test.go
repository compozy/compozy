package bridgesdk

import (
	"context"
	"testing"
	"time"
)

func TestInboundBatcherCoalescesShortBurstAndPreservesOrdering(t *testing.T) {
	t.Parallel()

	batches := make(chan InboundBatch, 1)
	batcher, err := NewInboundBatcher(InboundBatcherConfig{
		Delay: 20 * time.Millisecond,
		Now:   func() time.Time { return time.Now().UTC() },
		Dispatch: func(_ context.Context, batch InboundBatch) error {
			batches <- batch
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewInboundBatcher() error = %v", err)
	}
	defer batcher.Close()

	if err := batcher.Enqueue(testInboundEnvelope("idem-1", "msg-1", "first")); err != nil {
		t.Fatalf("Enqueue(first) error = %v", err)
	}
	if err := batcher.Enqueue(testInboundEnvelope("idem-2", "msg-2", "second")); err != nil {
		t.Fatalf("Enqueue(second) error = %v", err)
	}
	if err := batcher.Enqueue(testInboundEnvelope("idem-3", "msg-3", "third")); err != nil {
		t.Fatalf("Enqueue(third) error = %v", err)
	}

	select {
	case batch := <-batches:
		if got, want := len(batch.Items), 3; got != want {
			t.Fatalf("len(batch.Items) = %d, want %d", got, want)
		}
		if got, want := batch.Items[0].PlatformMessageID, "msg-1"; got != want {
			t.Fatalf("batch.Items[0].PlatformMessageID = %q, want %q", got, want)
		}
		if got, want := batch.Items[1].PlatformMessageID, "msg-2"; got != want {
			t.Fatalf("batch.Items[1].PlatformMessageID = %q, want %q", got, want)
		}
		if got, want := batch.Items[2].PlatformMessageID, "msg-3"; got != want {
			t.Fatalf("batch.Items[2].PlatformMessageID = %q, want %q", got, want)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for batched dispatch")
	}
}

func TestInboundBatcherFlushAllDispatchesPendingBatches(t *testing.T) {
	t.Parallel()

	batches := make(chan InboundBatch, 1)
	batcher, err := NewInboundBatcher(InboundBatcherConfig{
		Delay: time.Minute,
		Dispatch: func(_ context.Context, batch InboundBatch) error {
			batches <- batch
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewInboundBatcher() error = %v", err)
	}
	defer batcher.Close()

	if err := batcher.Enqueue(testInboundEnvelope("idem-1", "msg-1", "first")); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if err := batcher.FlushAll(context.Background()); err != nil {
		t.Fatalf("FlushAll() error = %v", err)
	}

	select {
	case batch := <-batches:
		if got, want := len(batch.Items), 1; got != want {
			t.Fatalf("len(batch.Items) = %d, want %d", got, want)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for flushed batch")
	}
}
