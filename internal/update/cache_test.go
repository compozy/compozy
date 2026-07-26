package update

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestReadWriteCacheRoundTrip(t *testing.T) {
	t.Run("Should persist and reload one cache entry", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "cache", "update-state.json")
		now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
		entry := testCacheEntry(
			t,
			nil,
			"v1.2.3",
			"https://github.com/compozy/agh/releases/tag/v1.2.3",
			now,
		)

		if err := writeCache(path, entry); err != nil {
			t.Fatalf("writeCache() error = %v", err)
		}

		got, err := readCache(path)
		if err != nil {
			t.Fatalf("readCache() error = %v", err)
		}
		if got.LatestVersion != entry.LatestVersion {
			t.Fatalf("LatestVersion = %q, want %q", got.LatestVersion, entry.LatestVersion)
		}
		if got.ReleaseURL != entry.ReleaseURL {
			t.Fatalf("ReleaseURL = %q, want %q", got.ReleaseURL, entry.ReleaseURL)
		}
		if !got.CheckedAt.Equal(entry.CheckedAt) {
			t.Fatalf("CheckedAt = %s, want %s", got.CheckedAt, entry.CheckedAt)
		}
		if len(got.Assets) != len(entry.Assets) {
			t.Fatalf("len(Assets) = %d, want %d", len(got.Assets), len(entry.Assets))
		}
	})
}

func TestReadCacheMissingFileReturnsSentinel(t *testing.T) {
	t.Run("Should return the no-cache sentinel for a missing file", func(t *testing.T) {
		t.Parallel()

		_, err := readCache(filepath.Join(t.TempDir(), "missing.json"))
		if !errors.Is(err, ErrNoCachedRelease) {
			t.Fatalf("readCache() error = %v, want %v", err, ErrNoCachedRelease)
		}
	})
}
