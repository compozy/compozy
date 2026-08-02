package sse

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

func TestDecodeContextContract(t *testing.T) {
	t.Parallel()

	t.Run("Should leave the reader caller-owned on every decoder exit", func(t *testing.T) {
		t.Parallel()

		handlerErr := errors.New("handler failed")
		for _, tt := range []struct {
			name       string
			stream     string
			handlerErr error
			wantErr    error
		}{
			{
				name: "Should preserve ownership after EOF",
			},
			{
				name:       "Should preserve ownership after ErrStop",
				stream:     "data: stop\n\n",
				handlerErr: ErrStop,
			},
			{
				name:       "Should preserve ownership after a handler error",
				stream:     "data: fail\n\n",
				handlerErr: handlerErr,
				wantErr:    handlerErr,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				reader := &trackingReadCloser{Reader: strings.NewReader(tt.stream)}
				err := Decode(t.Context(), reader, func(Event) error {
					return tt.handlerErr
				})

				if tt.wantErr == nil && err != nil {
					t.Fatalf("Decode() error = %v, want nil", err)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("Decode() error = %v, want %v", err, tt.wantErr)
				}
				if got := reader.closeCalls.Load(); got != 0 {
					t.Fatalf("Close() calls = %d, want 0", got)
				}
			})
		}
	})

	t.Run("Should return when a context-aware reader observes cancellation", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			reader := newContextAwareReadCloser(ctx)
			unexpectedHandlerErr := errors.New("handler called")
			errCh := make(chan error, 1)
			go func() {
				errCh <- Decode(ctx, reader, func(Event) error {
					return unexpectedHandlerErr
				})
			}()

			<-reader.readStarted
			cancel()
			err := <-errCh
			synctest.Wait()

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Decode() error = %v, want context.Canceled", err)
			}
			if errors.Is(err, unexpectedHandlerErr) {
				t.Fatalf("Decode() error = %v, want no handler error", err)
			}
			if got := reader.closeCalls.Load(); got != 0 {
				t.Fatalf("Close() calls = %d, want 0", got)
			}
		})
	})
}

type trackingReadCloser struct {
	io.Reader
	closeCalls atomic.Int32
}

func (r *trackingReadCloser) Close() error {
	r.closeCalls.Add(1)
	return errors.New("unexpected decoder close")
}

type contextAwareReadCloser struct {
	ctx         context.Context
	readStarted chan struct{}
	readOnce    sync.Once
	closeCalls  atomic.Int32
}

func newContextAwareReadCloser(ctx context.Context) *contextAwareReadCloser {
	return &contextAwareReadCloser{
		ctx:         ctx,
		readStarted: make(chan struct{}),
	}
}

func (r *contextAwareReadCloser) Read(_ []byte) (int, error) {
	r.readOnce.Do(func() {
		close(r.readStarted)
	})
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

func (r *contextAwareReadCloser) Close() error {
	r.closeCalls.Add(1)
	return errors.New("unexpected decoder close")
}
