package registry

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestMultiRegistryContractNotFound(t *testing.T) {
	t.Parallel()

	t.Run("Should return canonical package not found when every source misses", func(t *testing.T) {
		t.Parallel()

		registry := NewMultiRegistry(testLogger(), &stubRegistrySource{
			name: "clawhub",
			infoFunc: func(_ context.Context, slug string) (*Detail, error) {
				return nil, NewPackageNotFoundError(slug)
			},
		})

		_, err := registry.Info(context.Background(), "missing")
		if !errors.Is(err, ErrPackageNotFound) {
			t.Fatalf("Info(missing) error = %v, want ErrPackageNotFound", err)
		}
		if got, want := err.Error(), "registry: package \"missing\" not found"; got != want {
			t.Fatalf("Info(missing) error = %q, want %q", got, want)
		}
	})

	t.Run("Should resolve lower priority source when higher priority source misses", func(t *testing.T) {
		t.Parallel()

		lower := &stubRegistrySource{
			name: "local",
			infoFunc: func(_ context.Context, slug string) (*Detail, error) {
				return &Detail{Listing: Listing{Slug: slug, Source: "local"}}, nil
			},
			downloadFunc: func(_ context.Context, slug string, _ DownloadOpts) (*DownloadResult, error) {
				return &DownloadResult{
					Reader: io.NopCloser(strings.NewReader("archive")),
					Slug:   slug,
				}, nil
			},
		}
		higher := &stubRegistrySource{
			name: "clawhub",
			infoFunc: func(_ context.Context, slug string) (*Detail, error) {
				return nil, NewPackageNotFoundError(slug)
			},
		}
		registry := NewMultiRegistry(testLogger(), lower, higher)

		detail, err := registry.Info(context.Background(), "shared")
		if err != nil {
			t.Fatalf("Info(shared) error = %v", err)
		}
		if detail.Source != "local" {
			t.Fatalf("Info(shared) source = %q, want local", detail.Source)
		}

		result, err := registry.Download(context.Background(), "shared", DownloadOpts{})
		if err != nil {
			t.Fatalf("Download(shared) error = %v", err)
		}
		t.Cleanup(func() {
			if err := result.Reader.Close(); err != nil {
				t.Errorf("result.Reader.Close() error = %v", err)
			}
		})
		if got := lower.downloadCalls.Load(); got != 1 {
			t.Fatalf("lower.downloadCalls = %d, want 1", got)
		}
		if got := higher.downloadCalls.Load(); got != 0 {
			t.Fatalf("higher.downloadCalls = %d, want 0", got)
		}
	})
}
