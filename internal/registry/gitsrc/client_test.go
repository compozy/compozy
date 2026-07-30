package gitsrc

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/registry"
)

func TestClientDownload(t *testing.T) {
	t.Parallel()

	t.Run("Should shallow clone the requested ref and return an archive", func(t *testing.T) {
		t.Parallel()

		var gotExecutable string
		var gotArgs []string
		client := NewClient(
			WithLookPath(func(file string) (string, error) {
				if file != "git" {
					t.Fatalf("lookPath(%q), want git", file)
				}
				return "/usr/bin/git", nil
			}),
			WithRunner(func(_ context.Context, executable string, args ...string) error {
				gotExecutable = executable
				gotArgs = append([]string(nil), args...)
				checkoutDir := args[len(args)-1]
				if err := os.MkdirAll(checkoutDir, 0o755); err != nil {
					return err
				}
				return os.WriteFile(
					filepath.Join(checkoutDir, "extension.toml"),
					[]byte("[extension]\nname = \"fixture\"\nversion = \"1.0.0\"\n"),
					0o600,
				)
			}),
		)

		result, err := client.Download(t.Context(), "https://example.com/acme/fixture.git", registry.DownloadOpts{
			Version: "v1.2.3",
		})
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}
		if result == nil || result.Reader == nil {
			t.Fatal("Download() returned no archive reader")
		}
		archive, err := io.ReadAll(result.Reader)
		if err != nil {
			t.Fatalf("io.ReadAll() error = %v", err)
		}
		if err := result.Reader.Close(); err != nil {
			t.Fatalf("Reader.Close() error = %v", err)
		}
		if len(archive) == 0 || result.ContentType != "application/gzip" {
			t.Fatalf("Download() = %#v, archive bytes = %d", result, len(archive))
		}
		if gotExecutable != "/usr/bin/git" {
			t.Fatalf("executable = %q, want /usr/bin/git", gotExecutable)
		}
		wantArgs := []string{
			"clone", "--depth", "1", "--single-branch", "--branch", "v1.2.3", "--",
			"https://example.com/acme/fixture.git", gotArgs[len(gotArgs)-1],
		}
		if !reflect.DeepEqual(gotArgs, wantArgs) {
			t.Fatalf("git args = %#v, want %#v", gotArgs, wantArgs)
		}
	})

	t.Run("Should return a deterministic diagnostic when git is unavailable", func(t *testing.T) {
		t.Parallel()

		client := NewClient(WithLookPath(func(string) (string, error) {
			return "", exec.ErrNotFound
		}))
		_, err := client.Download(t.Context(), "https://example.com/acme/fixture.git", registry.DownloadOpts{})
		if !errors.Is(err, ErrGitUnavailable) {
			t.Fatalf("Download() error = %v, want ErrGitUnavailable", err)
		}
		if !strings.Contains(err.Error(), "install Git") {
			t.Fatalf("Download() error = %q, want install diagnostic", err)
		}
	})
}

func TestClientSearch(t *testing.T) {
	t.Parallel()

	t.Run("Should report search as unsupported", func(t *testing.T) {
		t.Parallel()

		client := NewClient()
		_, err := client.Search(context.Background(), "fixture", registry.SearchOpts{})
		if !errors.Is(err, registry.ErrNotSupported) {
			t.Fatalf("Search() error = %v, want ErrNotSupported", err)
		}
	})
}
