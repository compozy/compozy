package pluginsource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// UT-072: the package cache owns atomic, verified bytes and reference-preserving eviction.
func TestPackageCache(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve verified bytes and eviction age across repeated writes and reopen", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: filepath.Join(t.TempDir(), "packages")}
		payload := []byte("canonical package bytes")
		digest := cacheDigest(payload)
		putCacheBlob(t, cache, payload)
		path := filepath.Join(cache.Root, digest+".tar")
		stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		putCacheBlob(t, cache, payload)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(stamp) {
			t.Fatal("idempotent Put changed eviction age")
		}
		reopened := &PackageCache{Root: cache.Root}
		assertCacheBytes(t, reopened, digest, payload)
	})
	t.Run("Should never publish a partial blob and remove failed staging files", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir()}
		payload := []byte("complete package")
		digest := cacheDigest(payload)
		injected := errors.New("source read failed")
		started := false
		source := cacheReadFunc(func(buffer []byte) (int, error) {
			if !started {
				started = true
				return copy(buffer, payload[:4]), nil
			}
			if _, err := os.Stat(filepath.Join(cache.Root, digest+".tar")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("uncommitted blob became visible: %v", err)
			}
			return 0, injected
		})
		if err := cache.Put(t.Context(), digest, source); !errors.Is(err, injected) {
			t.Fatalf("Put = %v", err)
		}
		assertCacheEmpty(t, cache)
	})
	t.Run("Should reject mismatched and oversized streams without leaving blobs", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir(), MaxBytes: 4}
		if err := cache.Put(
			t.Context(),
			cacheDigest([]byte("good")),
			strings.NewReader("bad"),
		); !errors.Is(
			err,
			ErrPackageUnavailable,
		) {
			t.Fatalf("mismatched Put = %v", err)
		}
		if err := cache.Put(
			t.Context(),
			cacheDigest([]byte("large")),
			strings.NewReader("large"),
		); !errors.Is(
			err,
			ErrCacheCapacity,
		) {
			t.Fatalf("oversized Put = %v", err)
		}
		assertCacheEmpty(t, cache)
		putCacheBlob(t, cache, []byte("four"))
		assertCacheBytes(t, cache, cacheDigest([]byte("four")), []byte("four"))
	})
	t.Run("Should reject corrupted blobs and repair them only with matching approved bytes", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir()}
		payload := []byte("approved")
		digest := cacheDigest(payload)
		putCacheBlob(t, cache, payload)
		if err := os.WriteFile(filepath.Join(cache.Root, digest+".tar"), []byte("corrupted"), 0o600); err != nil {
			t.Fatal(err)
		}
		reader, err := cache.Open(t.Context(), digest)
		if reader != nil || !errors.Is(err, ErrPackageUnavailable) {
			t.Fatalf("corrupt Open = %v, %v", reader, err)
		}
		putCacheBlob(t, cache, payload)
		assertCacheBytes(t, cache, digest, payload)
	})
	t.Run("Should preserve pinned blobs and evict unreferenced bytes oldest first", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir()}
		payloads := [][]byte{[]byte("pinned"), []byte("oldest"), []byte("recent")}
		stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		for index, payload := range payloads {
			putCacheBlob(t, cache, payload)
			date := stamp.Add(time.Duration(index) * time.Hour)
			if err := os.Chtimes(filepath.Join(cache.Root, cacheDigest(payload)+".tar"), date, date); err != nil {
				t.Fatal(err)
			}
		}
		cache.MaxBytes = 12
		if err := cache.Sweep(t.Context(), map[string]struct{}{cacheDigest(payloads[0]): {}}); err != nil {
			t.Fatal(err)
		}
		assertCacheBytes(t, cache, cacheDigest(payloads[0]), payloads[0])
		assertCacheBytes(t, cache, cacheDigest(payloads[2]), payloads[2])
		reader, err := cache.Open(t.Context(), cacheDigest(payloads[1]))
		if reader != nil || !errors.Is(err, ErrPackageUnavailable) {
			t.Fatalf("evicted Open = %v, %v", reader, err)
		}
	})
	t.Run("Should wait for publication before reading pins and sweeping", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir(), MaxBytes: 6}
		old, candidate := []byte("oldest"), []byte("newest")
		putCacheBlob(t, cache, old)
		release := sync.OnceFunc(cache.Hold())
		t.Cleanup(release)
		putCacheBlob(t, cache, candidate)
		started, loaded := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		go func() {
			close(started)
			report, err := cache.SweepCurrent(t.Context(), func(context.Context) (map[string]struct{}, error) {
				close(loaded)
				return map[string]struct{}{cacheDigest(candidate): {}}, nil
			})
			if err == nil && (report.EvictedCount != 1 || report.EvictedBytes != 6) {
				err = errors.New("sweep did not report the unreferenced package eviction")
			}
			done <- err
		}()
		<-started
		select {
		case <-loaded:
			t.Fatal("sweep read references before the held publication finished")
		case <-time.After(20 * time.Millisecond):
		}
		release()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		assertCacheBytes(t, cache, cacheDigest(candidate), candidate)
		if reader, err := cache.Open(
			t.Context(),
			cacheDigest(old),
		); reader != nil ||
			!errors.Is(err, ErrPackageUnavailable) {
			t.Fatalf("unreferenced package = %v, %v", reader, err)
		}
	})

	t.Run("Should preserve every pin even when the pinned set exceeds the budget", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir(), MaxBytes: 6}
		first, second := []byte("aaaa"), []byte("bbbb")
		putCacheBlob(t, cache, first)
		putCacheBlob(t, cache, second)
		pins := map[string]struct{}{cacheDigest(first): {}, cacheDigest(second): {}}
		if err := cache.Sweep(t.Context(), pins); !errors.Is(err, ErrCacheCapacity) {
			t.Fatalf("Sweep = %v", err)
		}
		assertCacheBytes(t, cache, cacheDigest(first), first)
		assertCacheBytes(t, cache, cacheDigest(second), second)
	})
	t.Run("Should keep an opened reader valid when its unreferenced blob is evicted", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir(), MaxBytes: 8}
		payload := []byte("original")
		digest := cacheDigest(payload)
		putCacheBlob(t, cache, payload)
		reader, err := cache.Open(t.Context(), digest)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reader.Close(); err != nil {
				t.Error(err)
			}
		})
		cache.MaxBytes = 4
		if err := cache.Sweep(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		actual, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actual, payload) {
			t.Fatalf("held bytes = %q", actual)
		}
	})
	t.Run("Should clean interrupted staging files without removing unrelated files", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir()}
		for _, name := range []string{".pending-interrupted", "operator-note"} {
			if err := os.WriteFile(filepath.Join(cache.Root, name), []byte("partial"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := cache.Sweep(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(cache.Root, ".pending-interrupted")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("staging remains: %v", err)
		}
		if _, err := os.Stat(filepath.Join(cache.Root, "operator-note")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should reject symlink blobs without changing the external file", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir()}
		payload := []byte("approved")
		digest := cacheDigest(payload)
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.WriteFile(outside, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(cache.Root, digest+".tar")); err != nil {
			t.Fatal(err)
		}
		reader, err := cache.Open(t.Context(), digest)
		if reader != nil || err == nil {
			t.Fatalf("symlink Open = %v, %v", reader, err)
		}
		if err := cache.Put(t.Context(), digest, bytes.NewReader(payload)); err == nil {
			t.Fatal("Put accepted symlink")
		}
		actual, err := os.ReadFile(outside)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actual, payload) {
			t.Fatal("external file changed")
		}
	})
	t.Run("Should serialize repeated concurrent writes without leaking staging files", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: t.TempDir()}
		payload := []byte("same digest")
		digest := cacheDigest(payload)
		var group sync.WaitGroup
		errorsCh := make(chan error, 8)
		for range 8 {
			group.Go(func() { errorsCh <- cache.Put(t.Context(), digest, bytes.NewReader(payload)) })
		}
		group.Wait()
		close(errorsCh)
		for err := range errorsCh {
			if err != nil {
				t.Fatal(err)
			}
		}
		assertCacheBytes(t, cache, digest, payload)
		entries, err := os.ReadDir(cache.Root)
		if err != nil || len(entries) != 1 {
			t.Fatalf("entries = %v, %v", entries, err)
		}
	})
	t.Run("Should stop canceled writes and reject invalid identities before filesystem writes", func(t *testing.T) {
		t.Parallel()
		cache := &PackageCache{Root: filepath.Join(t.TempDir(), "absent")}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := cache.Put(
			ctx,
			cacheDigest([]byte("data")),
			strings.NewReader("data"),
		); !errors.Is(
			err,
			context.Canceled,
		) {
			t.Fatalf("canceled Put = %v", err)
		}
		for _, digest := range []string{"", "../escape", strings.Repeat("A", 64), strings.Repeat("z", 64)} {
			if err := cache.Put(t.Context(), digest, strings.NewReader("data")); err == nil {
				t.Fatalf("accepted digest %q", digest)
			}
		}
		if _, err := os.Stat(cache.Root); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid request wrote root: %v", err)
		}
		if err := cache.Sweep(t.Context(), nil); err != nil {
			t.Fatal(err)
		}
		readCtx, cancelRead := context.WithCancel(t.Context())
		defer cancelRead()
		source := cacheReadFunc(func(buffer []byte) (int, error) {
			cancelRead()
			return copy(buffer, "data"), nil
		})
		if err := cache.Put(readCtx, cacheDigest([]byte("data")), source); !errors.Is(err, context.Canceled) {
			t.Fatalf("Put canceled while reading = %v", err)
		}
		assertCacheEmpty(t, cache)
	})
}

func cacheDigest(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func putCacheBlob(t *testing.T, cache *PackageCache, payload []byte) {
	t.Helper()
	if err := cache.Put(t.Context(), cacheDigest(payload), bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
}

func assertCacheBytes(t *testing.T, cache *PackageCache, digest string, expected []byte) {
	t.Helper()
	reader, err := cache.Open(t.Context(), digest)
	if err != nil {
		t.Fatal(err)
	}
	actual, readErr := io.ReadAll(reader)
	if err := errors.Join(readErr, reader.Close()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("Open bytes = %q, want %q", actual, expected)
	}
}

func assertCacheEmpty(t *testing.T, cache *PackageCache) {
	t.Helper()
	entries, err := os.ReadDir(cache.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed write left entries: %v", entries)
	}
}

type cacheReadFunc func([]byte) (int, error)

func (fn cacheReadFunc) Read(buffer []byte) (int, error) { return fn(buffer) }
