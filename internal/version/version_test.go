package version

import (
	"testing"
	"testing/synctest"
)

func TestCurrentReturnsDefaults(t *testing.T) {
	t.Run("Should return non-empty build defaults", func(t *testing.T) {
		t.Parallel()

		info := Current()
		if info.Version == "" || info.Commit == "" || info.BuildDate == "" {
			t.Fatalf("Current() = %#v, want non-empty fields", info)
		}
	})
}

func TestInfoStringIncludesBuildMetadata(t *testing.T) {
	t.Run("Should include every build metadata field", func(t *testing.T) {
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
	})
}

func TestOverrideVersionForTestingDoesNotBlockCurrent(t *testing.T) {
	t.Run("Should allow Current while a test override is active", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			restore := OverrideVersionForTesting("test-override")
			t.Cleanup(restore)

			done := make(chan Info, 1)
			go func() {
				done <- Current()
			}()

			synctest.Wait()
			select {
			case info := <-done:
				if info.Version != "test-override" {
					t.Fatalf("Current().Version = %q, want %q", info.Version, "test-override")
				}
			default:
				t.Fatal("Current() blocked while a test override was active")
			}
		})
	})
}
