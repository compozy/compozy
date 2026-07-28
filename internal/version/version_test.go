package version

import (
	"testing"
	"time"
)

func TestCurrentReturnsDefaults(t *testing.T) {
	t.Parallel()

	info := Current()
	if info.Version == "" || info.Commit == "" || info.BuildDate == "" {
		t.Fatalf("Current() = %#v, want non-empty fields", info)
	}
}

func TestInfoStringIncludesBuildMetadata(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:   "1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-04-03T00:00:00Z",
	}

	got := info.String()
	if got != "1.2.3 (abc123, 2026-04-03T00:00:00Z)" {
		t.Fatalf("Info.String() = %q", got)
	}
}

func TestOverrideVersionForTestingDoesNotBlockCurrent(t *testing.T) {
	restore := OverrideVersionForTesting("test-override")
	t.Cleanup(restore)

	done := make(chan Info, 1)
	go func() {
		done <- Current()
	}()

	select {
	case info := <-done:
		if info.Version != "test-override" {
			t.Fatalf("Current().Version = %q, want %q", info.Version, "test-override")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Current() blocked while a test override was active")
	}
}
