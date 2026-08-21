package extensionpkg

import (
	"context"
	"errors"
	"testing"
)

func TestViewCallGateRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Should cap each extension instance and release canceled waiters", func(t *testing.T) {
		t.Parallel()
		registry := &viewCallGateRegistry{}
		key := InstanceKey{Name: "notes", WorkspaceID: "workspace-a"}
		releases := make([]func(), 0, maxConcurrentViewCallsPerInstance)
		for range maxConcurrentViewCallsPerInstance {
			release, err := registry.acquire(t.Context(), key)
			if err != nil {
				t.Fatalf("acquire(active slot) error = %v", err)
			}
			releases = append(releases, release)
		}

		otherRelease, err := registry.acquire(
			t.Context(),
			InstanceKey{Name: "notes", WorkspaceID: "workspace-b"},
		)
		if err != nil {
			t.Fatalf("acquire(other instance) error = %v", err)
		}
		otherRelease()

		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := registry.acquire(canceled, key); !errors.Is(err, context.Canceled) {
			t.Fatalf("acquire(canceled waiter) error = %v, want context.Canceled", err)
		}

		for _, release := range releases {
			release()
			release()
		}
		registry.mu.Lock()
		defer registry.mu.Unlock()
		if len(registry.gates) != 0 {
			t.Fatalf("gate count = %d, want zero after releases", len(registry.gates))
		}
	})
}
