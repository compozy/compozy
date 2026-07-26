package sse

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestDecodeContract(t *testing.T) {
	t.Parallel()

	t.Run("Should stop when context cancels a blocked reader", func(t *testing.T) {
		t.Parallel()

		unexpectedHandlerErr := errors.New("handler called")
		ctx, cancel := context.WithCancel(context.Background())
		reader, writer := io.Pipe()
		t.Cleanup(func() {
			if err := writer.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Errorf("PipeWriter.Close() error = %v", err)
			}
		})
		errCh := make(chan error, 1)
		go func() {
			errCh <- Decode(ctx, reader, func(Event) error {
				return unexpectedHandlerErr
			})
		}()

		cancel()
		select {
		case err := <-errCh:
			if errors.Is(err, unexpectedHandlerErr) {
				t.Fatal("Decode() handler called, want blocked reader cancellation")
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Decode() error = %v, want context.Canceled", err)
			}
		case <-time.After(200 * time.Millisecond):
			if err := reader.CloseWithError(context.Canceled); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("PipeReader.CloseWithError() error = %v", err)
			}
			select {
			case err := <-errCh:
				t.Fatalf("Decode() returned only after forced reader close with error %v", err)
			case <-time.After(time.Second):
				t.Fatal("Decode() did not return after forced reader close")
			}
		}
	})
}
