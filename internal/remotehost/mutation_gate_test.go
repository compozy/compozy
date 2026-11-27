package remotehost

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestMutationGate(t *testing.T) {
	t.Parallel()

	t.Run("Should release idle state after repeated committed revocations", func(t *testing.T) {
		t.Parallel()
		gate := NewMutationGate(func(context.Context, string, uint64) error { return nil })
		connections := NewConnectionRegistry()
		commit := func(context.Context) error { return nil }

		for index := range 8 {
			deviceID := fmt.Sprintf("dev_%d", index)
			if _, err := gate.Revoke(t.Context(), deviceID, connections, commit); err != nil {
				t.Fatalf("Revoke(%q) error = %v", deviceID, err)
			}
		}
		if tracked := len(gate.devices); tracked != 0 {
			t.Fatalf("tracked mutation-gate devices = %d, want 0 after committed revocations", tracked)
		}
	})

	t.Run("Should release a lease when epoch validation is canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		gate := NewMutationGate(func(ctx context.Context, _ string, _ uint64) error {
			cancel()
			return ctx.Err()
		})

		if _, err := gate.AcquireMutation(ctx, "dev_canceled", 0); !errors.Is(err, context.Canceled) {
			t.Fatalf("AcquireMutation() error = %v, want context.Canceled", err)
		}
		if tracked := len(gate.devices); tracked != 0 {
			t.Fatalf("tracked mutation-gate devices = %d, want canceled lease released", tracked)
		}
	})
}
