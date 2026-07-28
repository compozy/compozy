package bridgesdk

import (
	"context"
	"errors"
	"testing"
)

func TestHostAPIClientRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should reject nil context before invoking transport", func(t *testing.T) {
		t.Parallel()

		called := false
		client := NewHostAPIClientFromCall(func(context.Context, string, any, any) error {
			called = true
			return nil
		})

		if err := client.Call(nilHostAPIContext(), "bridges/instances/list", nil, nil); err == nil {
			t.Fatal("Call(nil context) error = nil, want non-nil")
		} else if got, want := err.Error(), "bridgesdk: host api context is required"; got != want {
			t.Fatalf("Call(nil context) error = %q, want %q", got, want)
		}
		if called {
			t.Fatal("transport called = true, want false")
		}
	})

	t.Run("Should reject canceled context before invoking transport", func(t *testing.T) {
		t.Parallel()

		called := false
		client := NewHostAPIClientFromCall(func(context.Context, string, any, any) error {
			called = true
			return nil
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := client.Call(ctx, "bridges/instances/list", nil, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Call(canceled context) error = %v, want context.Canceled", err)
		}
		if called {
			t.Fatal("transport called = true, want false")
		}
	})
}

func nilHostAPIContext() context.Context {
	return nil
}
