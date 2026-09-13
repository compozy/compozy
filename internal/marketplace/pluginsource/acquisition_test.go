package pluginsource

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
)

func TestCapturePackage(t *testing.T) {
	t.Parallel()
	t.Run("Should retain the same confined package bytes after the checkout is removed", func(t *testing.T) {
		t.Parallel()
		checkout := t.TempDir()
		manifest := []byte(`{"name":"tool","version":"1.0.0"}`)
		writeMarketplaceDocument(t, checkout, "plugins/tool/plugin.json", manifest)
		writeMarketplaceDocument(t, checkout, "plugins/tool/.git/config", []byte("private Git configuration"))
		writeMarketplaceDocument(t, checkout, "unrelated.txt", []byte("outside the package"))
		cache := &PackageCache{Root: t.TempDir()}
		first, err := CapturePackage(t.Context(), checkout, "./plugins/tool", cache)
		if err != nil {
			t.Fatal(err)
		}
		stamp := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(filepath.Join(checkout, "plugins", "tool", "plugin.json"), stamp, stamp); err != nil {
			t.Fatal(err)
		}
		second, err := CapturePackage(t.Context(), checkout, "plugins/tool", cache)
		if err != nil || first != second {
			t.Fatalf("repeat capture digest = %q, want %q, %v", second, first, err)
		}
		if err := os.RemoveAll(checkout); err != nil {
			t.Fatal(err)
		}
		reopened := &PackageCache{Root: cache.Root}
		reader, err := reopened.Open(t.Context(), first)
		if err != nil {
			t.Fatal(err)
		}
		raw, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || cacheDigest(raw) != first {
			t.Fatalf("cached package integrity: read %v, close %v", readErr, closeErr)
		}
		archive := tar.NewReader(bytes.NewReader(raw))
		header, err := archive.Next()
		if err != nil || header.Name != "plugin.json" {
			t.Fatalf("package entry = %+v, %v", header, err)
		}
		payload, err := io.ReadAll(archive)
		if err != nil || !bytes.Equal(payload, manifest) {
			t.Fatalf("cached manifest = %q, %v", payload, err)
		}
		if _, err := archive.Next(); !errors.Is(err, io.EOF) {
			t.Fatalf("package included extra checkout/Git entries: %v", err)
		}
	})
	t.Run("Should refuse traversal absolute paths and symlink package roots without caching them", func(t *testing.T) {
		t.Parallel()
		checkout := t.TempDir()
		outside := t.TempDir()
		writeMarketplaceDocument(t, outside, "plugin.json", []byte(`{"name":"outside"}`))
		if err := os.Symlink(outside, filepath.Join(checkout, "escape")); err != nil {
			t.Fatal(err)
		}
		cache := &PackageCache{Root: t.TempDir()}
		for _, path := range []string{"../outside", outside, "escape", "..\\outside", "\x00"} {
			if _, err := CapturePackage(t.Context(), checkout, path, cache); !errors.Is(err, ErrSourceOutsideCheckout) {
				t.Fatalf("CapturePackage(%q) = %v", path, err)
			}
		}
		assertCacheEmpty(t, cache)
	})
	t.Run("Should leave the cache empty when a package exceeds its budget or cannot be read", func(t *testing.T) {
		t.Parallel()
		checkout := t.TempDir()
		writeMarketplaceDocument(t, checkout, "plugin.json", []byte(strings.Repeat("x", 4096)))
		cache := &PackageCache{Root: t.TempDir(), MaxBytes: 1024}
		if _, err := CapturePackage(t.Context(), checkout, "", cache); !errors.Is(err, fileutil.ErrTarSizeLimit) {
			t.Fatalf("oversized package = %v", err)
		}
		if _, err := CapturePackage(t.Context(), checkout, "missing", cache); !errors.Is(err, ErrSourceUnreachable) {
			t.Fatalf("missing package = %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		cancel()
		if _, err := CapturePackage(ctx, checkout, ".", cache); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled package = %v", err)
		}
		assertCacheEmpty(t, cache)
	})
}
