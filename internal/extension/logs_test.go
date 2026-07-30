package extensionpkg

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/subprocess"
)

func TestExtensionLogRing(t *testing.T) {
	t.Parallel()

	t.Run("Should remain bounded and drop the oldest records under concurrent writes", func(t *testing.T) {
		t.Parallel()

		ring := newExtensionLogRing(func() time.Time {
			return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
		})
		const (
			writers          = 32
			recordsPerWriter = 16
		)
		message := strings.Repeat("x", 1024)
		completed := make(chan struct{})
		go func() {
			var writes sync.WaitGroup
			writes.Add(writers)
			for writer := range writers {
				go func(writer int) {
					defer writes.Done()
					for record := range recordsPerWriter {
						ring.append(fmt.Sprintf("%02d:%02d:%s", writer, record, message), "generation")
					}
				}(writer)
			}
			writes.Wait()
			close(completed)
		}()

		select {
		case <-completed:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent log writes blocked")
		}

		entries := ring.snapshot(0)
		if len(entries) == 0 {
			t.Fatal("snapshot() = empty, want retained records")
		}
		retainedBytes := 0
		for index, entry := range entries {
			retainedBytes += len(entry.Message)
			if index > 0 && entry.Sequence <= entries[index-1].Sequence {
				t.Fatalf("sequence[%d] = %d, want greater than %d", index, entry.Sequence, entries[index-1].Sequence)
			}
		}
		if retainedBytes > extensionLogRingCapacity {
			t.Fatalf("retained bytes = %d, want <= %d", retainedBytes, extensionLogRingCapacity)
		}
		if entries[0].Sequence <= 1 {
			t.Fatalf("oldest retained sequence = %d, want dropped records", entries[0].Sequence)
		}

		after := entries[len(entries)/2].Sequence
		newer := ring.snapshot(after)
		for _, entry := range newer {
			if entry.Sequence <= after {
				t.Fatalf("snapshot(%d) returned sequence %d", after, entry.Sequence)
			}
		}
	})
}

func TestExtensionStderrPipelineRedactsBeforeCapture(t *testing.T) {
	t.Parallel()

	const dynamicSecret = "runtime-extension-secret-123456789"
	cleanup := diagnostics.RegisterDynamicSecret(dynamicSecret)
	t.Cleanup(cleanup)

	ring := newExtensionLogRing(time.Now)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	process, err := subprocess.Launch(ctx, subprocess.LaunchConfig{
		Command:          os.Args[0],
		Args:             []string{"-test.run=^TestExtensionLogHelperProcess$"},
		Env:              append(os.Environ(), extensionHelperEnvKey+"=1", "COMPOZY_TEST_EXTENSION_LOG_HELPER=1"),
		DisableTransport: true,
		StderrTransform:  diagnostics.Redact,
		StderrSink:       extensionLogWriter{ring: ring, generationHash: "generation"},
	})
	if err != nil {
		t.Fatalf("subprocess.Launch() error = %v", err)
	}

	select {
	case <-process.Done():
	case <-ctx.Done():
		t.Fatalf("helper process did not exit: %v", ctx.Err())
	}
	waitErr := process.Wait()
	if waitErr == nil {
		t.Fatal("Process.Wait() error = nil, want helper exit error")
	}

	entries := ring.snapshot(0)
	var retained strings.Builder
	for _, entry := range entries {
		retained.WriteString(entry.Message)
	}
	assertNoExtensionLogSecret(t, "ring", retained.String(), dynamicSecret)

	env := newRegistryTestEnv(t)
	key := InstanceKey{Name: "redacted-logs", WorkspaceID: "workspace-redaction"}
	if _, err := env.registry.LinkDev(DevLinkRequest{
		Name:           key.Name,
		WorkspaceID:    key.WorkspaceID,
		OriginPath:     t.TempDir(),
		GenerationHash: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("LinkDev() error = %v", err)
	}
	manager := NewManager(env.registry)
	manager.devLogs[key] = ring
	logs, err := manager.Logs(key, 0)
	if err != nil {
		t.Fatalf("Manager.Logs() error = %v", err)
	}
	var projected strings.Builder
	for _, entry := range logs {
		projected.WriteString(entry.Message)
	}
	assertNoExtensionLogSecret(t, "Manager.Logs", projected.String(), dynamicSecret)
	assertNoExtensionLogSecret(t, "diagnostic capture", process.Stderr(), dynamicSecret)
	assertNoExtensionLogSecret(t, "wait error", waitErr.Error(), dynamicSecret)
}

func TestExtensionLogHelperProcess(t *testing.T) {
	if os.Getenv("COMPOZY_TEST_EXTENSION_LOG_HELPER") != "1" {
		return
	}
	if _, err := fmt.Fprintln(
		os.Stderr,
		"dynamic=runtime-extension-secret-123456789 token=provider-token safe=visible",
	); err != nil {
		t.Fatalf("fmt.Fprintln(os.Stderr) error = %v", err)
	}
	os.Exit(17)
}

func assertNoExtensionLogSecret(t *testing.T, surface, value, dynamicSecret string) {
	t.Helper()
	for _, secret := range []string{dynamicSecret, "provider-token"} {
		if strings.Contains(value, secret) {
			t.Fatalf("%s leaked %q: %q", surface, secret, value)
		}
	}
	if !strings.Contains(value, "[REDACTED]") {
		t.Fatalf("%s = %q, want redaction marker", surface, value)
	}
}
