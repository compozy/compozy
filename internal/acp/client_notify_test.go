package acp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
)

// TestDriverNotifyExtension pins the ACP keepalive leg: one fire-and-forget ping over the live connection.
func TestDriverNotifyExtension(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver ping notifications to the live helper agent", func(t *testing.T) {
		t.Parallel()

		driver := New()
		captureFile := filepath.Join(t.TempDir(), "clarify-ping.jsonl")
		proc := startHelperProcess(t, driver, "echo_prompt", "", StartOpts{
			Env: helperEnvWithCapture("echo_prompt", "", captureFile),
		})
		t.Cleanup(func() {
			stopProcess(t, driver, proc)
		})
		ctx := testutil.Context(t)
		for _, seq := range []uint64{1, 2} {
			if err := driver.NotifyExtension(ctx, proc, "_compozy/clarify_ping", map[string]any{
				"session_id": "sess-9f2c",
				"request_id": "req-clarify-01",
				"seq":        seq,
				"asked_at":   "2026-09-17T10:00:00Z",
				"deadline":   nil,
			}); err != nil {
				t.Fatalf("NotifyExtension(seq %d) error = %v", seq, err)
			}
		}
		pings := waitForCapturedNotifications(t, captureFile, "_compozy/clarify_ping", 2)
		for index, params := range pings {
			if len(params) != 5 {
				payload, err := json.Marshal(params)
				if err != nil {
					t.Fatalf("json.Marshal(ping %d) error = %v", index+1, err)
				}
				t.Fatalf("ping %d params = %s, want exactly 5 identity keys", index+1, payload)
			}
			var decoded struct {
				SessionID string  `json:"session_id"`
				RequestID string  `json:"request_id"`
				Seq       float64 `json:"seq"`
				AskedAt   string  `json:"asked_at"`
				Deadline  *string `json:"deadline"`
			}
			payload, err := json.Marshal(params)
			if err != nil {
				t.Fatalf("json.Marshal(ping %d) error = %v", index+1, err)
			}
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("json.Unmarshal(ping %d) error = %v", index+1, err)
			}
			if decoded.SessionID != "sess-9f2c" || decoded.RequestID != "req-clarify-01" ||
				decoded.Seq != float64(index+1) || decoded.AskedAt != "2026-09-17T10:00:00Z" ||
				decoded.Deadline != nil {
				t.Fatalf("ping %d = %+v, want identity-only payload with increasing seq", index+1, decoded)
			}
		}
	})

	t.Run("Should reject pings without an extension method or process", func(t *testing.T) {
		t.Parallel()

		driver := New()
		ctx := testutil.Context(t)
		proc := &AgentProcess{}
		for name, call := range map[string]func() error{
			"nil context": func() error {
				return driver.NotifyExtension(nil, proc, "_compozy/clarify_ping", nil) //nolint:staticcheck // Verifies the nil-context guard.
			},
			"nil process": func() error {
				return driver.NotifyExtension(ctx, nil, "_compozy/clarify_ping", nil)
			},
			"uninitialized connection": func() error {
				return driver.NotifyExtension(ctx, proc, "_compozy/clarify_ping", nil)
			},
		} {
			if err := call(); err == nil {
				t.Fatalf("NotifyExtension(%s) error = nil, want validation", name)
			}
		}
		if err := driver.NotifyExtension(
			ctx, proc, "_compozy/clarify_ping", nil,
		); !errors.Is(err, errProcessConnectionUninitialized) {
			t.Fatalf("NotifyExtension(uninitialized) error = %v, want %v", err, errProcessConnectionUninitialized)
		}

		live := startHelperProcess(t, driver, "echo_prompt", "", StartOpts{})
		t.Cleanup(func() {
			stopProcess(t, driver, live)
		})
		for _, method := range []string{"  ", "session/prompt"} {
			err := driver.NotifyExtension(ctx, live, method, nil)
			if err == nil || !strings.Contains(err.Error(), "must start with '_'") {
				t.Fatalf("NotifyExtension(method %q) error = %v, want the extension-method guard", method, err)
			}
		}
	})

	t.Run("Should surface a dead connection to the fail-open caller", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "echo_prompt", "", StartOpts{})
		stopProcess(t, driver, proc)
		if err := driver.NotifyExtension(
			testutil.Context(t),
			proc,
			"_compozy/clarify_ping",
			map[string]any{"seq": uint64(1)},
		); err == nil {
			t.Fatal("NotifyExtension(dead connection) error = nil, want the delivery failure")
		}
	})
}

// waitForCapturedNotifications polls the helper stdin capture until the
// expected number of extension notifications arrives. The helper tees its
// stdin on read, so it can lag the synchronous client-side write.
func waitForCapturedNotifications(
	t *testing.T,
	captureFile string,
	method string,
	want int,
) []map[string]json.RawMessage {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if matches := capturedNotificationParams(captureFile, method); len(matches) >= want {
			return matches
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("capture file %q holds fewer than %d %q notifications", captureFile, want, method)
	return nil
}

func capturedNotificationParams(captureFile, method string) []map[string]json.RawMessage {
	data, err := os.ReadFile(captureFile)
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return nil
	}
	matches := make([]map[string]json.RawMessage, 0)
	for line := range strings.Lines(strings.TrimSpace(string(data))) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var envelope capturedRequestEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			continue
		}
		if envelope.Method != method {
			continue
		}
		var params map[string]json.RawMessage
		if err := json.Unmarshal(envelope.Params, &params); err != nil {
			continue
		}
		matches = append(matches, params)
	}
	return matches
}
