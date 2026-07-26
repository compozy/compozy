package bridgesdk

import (
	"context"
	"errors"
	"testing"
)

func TestRetryDoRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should reject nil context before invoking operation", func(t *testing.T) {
		t.Parallel()

		called := false
		_, err := RetryDo(nilRetryContext(), RetryConfig{}, func(context.Context) (string, error) {
			called = true
			return "unexpected", nil
		})
		if got, want := err.Error(), "bridgesdk: retry context is required"; got != want {
			t.Fatalf("RetryDo(nil context) error = %q, want %q", got, want)
		}
		if called {
			t.Fatal("operation called = true, want false")
		}
	})

	t.Run("Should reject canceled context before invoking operation", func(t *testing.T) {
		t.Parallel()

		called := false
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := RetryDo(ctx, RetryConfig{}, func(context.Context) (string, error) {
			called = true
			return "unexpected", nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RetryDo(canceled context) error = %v, want context.Canceled", err)
		}
		if called {
			t.Fatal("operation called = true, want false")
		}
	})

	t.Run("Should reject nil operation", func(t *testing.T) {
		t.Parallel()

		_, err := RetryDo[string](context.Background(), RetryConfig{}, nil)
		if got, want := err.Error(), "bridgesdk: retry function is required"; got != want {
			t.Fatalf("RetryDo(nil operation) error = %q, want %q", got, want)
		}
	})
}

func nilRetryContext() context.Context {
	return nil
}
