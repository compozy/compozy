package bridgesdk

import (
	"context"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func TestInboundBatcherRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should reject zero-delay enqueue after close without dispatch", func(t *testing.T) {
		t.Parallel()

		batches := make(chan InboundBatch, 1)
		batcher, err := NewInboundBatcher(InboundBatcherConfig{
			Delay: 0,
			Dispatch: func(_ context.Context, batch InboundBatch) error {
				batches <- batch
				return nil
			},
		})
		if err != nil {
			t.Fatalf("NewInboundBatcher() error = %v", err)
		}
		batcher.Close()

		err = batcher.Enqueue(testInboundEnvelope("idem-closed", "msg-closed", "closed"))
		if err == nil {
			t.Fatal("Enqueue() error = nil, want closed batcher error")
		}
		if got, want := err.Error(), "bridgesdk: inbound batcher is closed"; got != want {
			t.Fatalf("Enqueue() error = %q, want %q", got, want)
		}
		select {
		case batch := <-batches:
			t.Fatalf("dispatched batch after Close(): %#v", batch)
		default:
		}
	})

	t.Run("Should isolate explicit network conversation refs before zero-delay dispatch", func(t *testing.T) {
		t.Parallel()

		batches := make(chan InboundBatch, 1)
		batcher, err := NewInboundBatcher(InboundBatcherConfig{
			Delay: 0,
			Dispatch: func(_ context.Context, batch InboundBatch) error {
				batches <- batch
				return nil
			},
		})
		if err != nil {
			t.Fatalf("NewInboundBatcher() error = %v", err)
		}
		defer batcher.Close()

		envelope := testInboundEnvelope("idem-zero", "msg-zero", "hello")
		envelope.Conversation = &bridgepkg.NetworkConversationRef{
			Channel:  "network:primary",
			Surface:  bridgepkg.NetworkConversationSurfaceThread,
			ThreadID: "thread_original",
		}
		if err := batcher.Enqueue(envelope); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
		envelope.Conversation.ThreadID = "thread_mutated"

		select {
		case batch := <-batches:
			if got, want := batch.Items[0].Conversation.ThreadID, "thread_original"; got != want {
				t.Fatalf("batch.Items[0].Conversation.ThreadID = %q, want %q", got, want)
			}
		case <-time.After(250 * time.Millisecond):
			t.Fatal("timed out waiting for zero-delay batch")
		}
	})

	t.Run("Should isolate explicit network conversation refs before delayed dispatch", func(t *testing.T) {
		t.Parallel()

		batches := make(chan InboundBatch, 1)
		batcher, err := NewInboundBatcher(InboundBatcherConfig{
			Delay: time.Hour,
			Dispatch: func(_ context.Context, batch InboundBatch) error {
				batches <- batch
				return nil
			},
		})
		if err != nil {
			t.Fatalf("NewInboundBatcher() error = %v", err)
		}
		defer batcher.Close()

		envelope := testInboundEnvelope("idem-1", "msg-1", "hello")
		envelope.Conversation = &bridgepkg.NetworkConversationRef{
			Channel:  "network:primary",
			Surface:  bridgepkg.NetworkConversationSurfaceThread,
			ThreadID: "thread_original",
		}
		if err := batcher.Enqueue(envelope); err != nil {
			t.Fatalf("Enqueue() error = %v", err)
		}
		envelope.Conversation.ThreadID = "thread_mutated"

		if err := batcher.FlushAll(t.Context()); err != nil {
			t.Fatalf("FlushAll() error = %v", err)
		}

		select {
		case batch := <-batches:
			if got, want := batch.Items[0].Conversation.ThreadID, "thread_original"; got != want {
				t.Fatalf("batch.Items[0].Conversation.ThreadID = %q, want %q", got, want)
			}
		case <-time.After(250 * time.Millisecond):
			t.Fatal("timed out waiting for flushed batch")
		}
	})

	t.Run("Should avoid delimiter collisions in routing identity keys", func(t *testing.T) {
		t.Parallel()

		first := testInboundEnvelope("idem-1", "msg-1", "hello")
		first.WorkspaceID = "ws"
		first.PeerID = "peer|thread"
		first.ThreadID = "current"

		second := testInboundEnvelope("idem-2", "msg-2", "hello")
		second.WorkspaceID = "ws|peer"
		second.PeerID = "thread"
		second.ThreadID = "current"

		if gotFirst, gotSecond := InboundBatchKey(first), InboundBatchKey(second); gotFirst == gotSecond {
			t.Fatalf("InboundBatchKey collision = %q", gotFirst)
		}
	})

	t.Run("Should canonicalize empty message family like envelope validation", func(t *testing.T) {
		t.Parallel()

		withDefaultFamily := testInboundEnvelope("idem-1", "msg-1", "hello")
		withDefaultFamily.EventFamily = ""
		withMessageFamily := testInboundEnvelope("idem-1", "msg-1", "hello")

		if got, want := InboundBatchKey(withDefaultFamily), InboundBatchKey(withMessageFamily); got != want {
			t.Fatalf("InboundBatchKey(default family) = %q, want %q", got, want)
		}
	})
}
