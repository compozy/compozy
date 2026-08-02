package bridgesdk

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestInboundBatcher(t *testing.T) {
	t.Parallel()

	t.Run("Should coalesce a short burst after the delay and preserve ordering", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			const delay = time.Hour

			batches := make(chan InboundBatch, 1)
			batcher, err := NewInboundBatcher(InboundBatcherConfig{
				Context: t.Context(),
				Delay:   delay,
				Now:     func() time.Time { return time.Now().UTC() },
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

			synctest.Wait()
			select {
			case batch := <-batches:
				t.Fatalf("batch dispatched before delay: %#v", batch)
			default:
			}

			time.Sleep(delay)
			synctest.Wait()
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
			default:
				t.Fatal("batched dispatch was not observed after delay")
			}
		})
	})

	t.Run("Should synchronously flush every pending batch", func(t *testing.T) {
		t.Parallel()

		batches := make(chan InboundBatch, 1)
		batcher, err := NewInboundBatcher(InboundBatcherConfig{
			Context: t.Context(),
			Delay:   time.Minute,
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
		if err := batcher.FlushAll(t.Context()); err != nil {
			t.Fatalf("FlushAll() error = %v", err)
		}

		select {
		case batch := <-batches:
			if got, want := len(batch.Items), 1; got != want {
				t.Fatalf("len(batch.Items) = %d, want %d", got, want)
			}
		default:
			t.Fatal("flushed batch was not dispatched synchronously")
		}
	})

	t.Run("Should require owned contexts without consuming pending batches", func(t *testing.T) {
		t.Parallel()

		if _, err := NewInboundBatcher(InboundBatcherConfig{
			Dispatch: func(context.Context, InboundBatch) error { return nil },
		}); err == nil {
			t.Fatal("NewInboundBatcher(nil context) error = nil, want context validation failure")
		}

		batches := make(chan InboundBatch, 1)
		batcher, err := NewInboundBatcher(InboundBatcherConfig{
			Context: t.Context(),
			Delay:   time.Minute,
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
		var missingContext context.Context
		if err := batcher.FlushAll(missingContext); err == nil {
			t.Fatal("FlushAll(nil) error = nil, want context validation failure")
		}
		select {
		case batch := <-batches:
			t.Fatalf("batch dispatched after rejected flush: %#v", batch)
		default:
		}

		if err := batcher.FlushAll(t.Context()); err != nil {
			t.Fatalf("FlushAll(valid context) error = %v", err)
		}
		select {
		case batch := <-batches:
			if got, want := len(batch.Items), 1; got != want {
				t.Fatalf("len(batch.Items) = %d, want %d", got, want)
			}
		default:
			t.Fatal("pending batch was consumed by rejected flush")
		}
	})
}
