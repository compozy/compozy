package session

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
)

// TestManagerNotifyAgentExtension pins the session keepalive leg: live processes receive the ping.
func TestManagerNotifyAgentExtension(t *testing.T) {
	t.Parallel()

	t.Run("Should forward one ping to the live agent process", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		created := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, created.ID) })
		proc := created.processHandle()
		if proc == nil {
			t.Fatal("session process = nil, want a live fake process")
		}
		params := map[string]any{
			"session_id": created.ID,
			"request_id": "req-clarify-01",
			"seq":        uint64(1),
		}
		if err := h.manager.NotifyAgentExtension(
			testutil.Context(t),
			created.ID,
			"_compozy/clarify_ping",
			params,
		); err != nil {
			t.Fatalf("NotifyAgentExtension() error = %v", err)
		}
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		if len(h.driver.notifyCalls) != 1 {
			t.Fatalf("notify calls = %d, want 1", len(h.driver.notifyCalls))
		}
		call := h.driver.notifyCalls[0]
		if call.proc != proc {
			t.Fatalf("notify proc = %p, want the live session process %p", call.proc, proc)
		}
		if call.method != "_compozy/clarify_ping" {
			t.Fatalf("notify method = %q, want _compozy/clarify_ping", call.method)
		}
		got, ok := call.params.(map[string]any)
		if !ok || got["request_id"] != "req-clarify-01" || got["seq"] != uint64(1) {
			t.Fatalf("notify params = %#v, want the ping identity payload", call.params)
		}
	})

	t.Run("Should silently no-op without a live process", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		created := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, created.ID) })
		stored, ok := h.manager.Get(created.ID)
		if !ok || stored == nil {
			t.Fatal("Get(created session) = missing, want the live session")
		}
		stored.clearProcess(time.Time{})
		if err := h.manager.NotifyAgentExtension(
			testutil.Context(t),
			created.ID,
			"_compozy/clarify_ping",
			map[string]any{"seq": uint64(1)},
		); err != nil {
			t.Fatalf("NotifyAgentExtension() error = %v, want silent nil", err)
		}
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		if len(h.driver.notifyCalls) != 0 {
			t.Fatalf("notify calls = %d, want zero without a live process", len(h.driver.notifyCalls))
		}
	})

	t.Run("Should silently no-op for unknown sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		if err := h.manager.NotifyAgentExtension(
			testutil.Context(t),
			"sess-unknown",
			"_compozy/clarify_ping",
			map[string]any{"seq": uint64(1)},
		); err != nil {
			t.Fatalf("NotifyAgentExtension() error = %v, want silent nil", err)
		}
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		if len(h.driver.notifyCalls) != 0 {
			t.Fatalf("notify calls = %d, want zero for unknown sessions", len(h.driver.notifyCalls))
		}
	})

	t.Run("Should surface delivery failures to the fail-open caller", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		created := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, created.ID) })
		h.driver.mu.Lock()
		h.driver.notifyHook = func(*AgentProcess, string, any) error {
			return errors.New("agent gone")
		}
		h.driver.mu.Unlock()
		err := h.manager.NotifyAgentExtension(
			testutil.Context(t),
			created.ID,
			"_compozy/clarify_ping",
			map[string]any{"seq": uint64(1)},
		)
		if err == nil || err.Error() != "agent gone" {
			t.Fatalf("NotifyAgentExtension() error = %v, want the delivery failure", err)
		}
	})

	t.Run("Should reject pings without identity", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		ctx := testutil.Context(t)
		if err := h.manager.NotifyAgentExtension(
			ctx, "  ", "_compozy/clarify_ping", nil,
		); err == nil {
			t.Fatal("NotifyAgentExtension(blank session) error = nil, want identity validation")
		}
		if err := h.manager.NotifyAgentExtension(
			ctx, "sess-one", "  ", nil,
		); err == nil {
			t.Fatal("NotifyAgentExtension(blank method) error = nil, want method validation")
		}
		if err := h.manager.NotifyAgentExtension(
			nil, "sess-one", "_compozy/clarify_ping", nil, //nolint:staticcheck // Verifies the nil-context guard.
		); err == nil {
			t.Fatal("NotifyAgentExtension(nil context) error = nil, want context validation")
		}
	})
}
