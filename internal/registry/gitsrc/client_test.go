package gitsrc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/outboundpolicy"
	"github.com/compozy/compozy/internal/registry"
)

func TestClientDownload(t *testing.T) {
	t.Parallel()

	t.Run("Should shallow clone the requested ref and return an archive", func(t *testing.T) {
		t.Parallel()

		var gotExecutable string
		var gotArgs []string
		archiveTempDir := t.TempDir()
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
			withGitVersionProbe(supportedGitVersion),
			withRepositoryResolver(&staticRepositoryResolver{addresses: publicRepositoryAddresses()}),
		)
		client.archiveTempDir = archiveTempDir

		result, err := client.Download(t.Context(), "https://EXAMPLE.COM/acme/fixture.git", registry.DownloadOpts{
			Version: "v1.2.3",
		})
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}
		if result == nil || result.Reader == nil {
			t.Fatal("Download() returned no archive reader")
		}
		owned, ok := result.Reader.(*ownedArchiveFile)
		if !ok {
			t.Fatalf("Download() reader type = %T, want *ownedArchiveFile", result.Reader)
		}
		archivePath := owned.path
		archive, err := io.ReadAll(result.Reader)
		if err != nil {
			t.Fatalf("io.ReadAll() error = %v", err)
		}
		if err := result.Reader.Close(); err != nil {
			t.Fatalf("Reader.Close() error = %v", err)
		}
		if _, err := os.Stat(archivePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(%q) error = %v, want os.ErrNotExist after Close", archivePath, err)
		}
		if len(archive) == 0 || result.ContentType != "application/gzip" {
			t.Fatalf("Download() = %#v, archive bytes = %d", result, len(archive))
		}
		if result.ContentSize != int64(len(archive)) {
			t.Fatalf("ContentSize = %d, archive bytes = %d", result.ContentSize, len(archive))
		}
		assertArchiveFile(t, archive, "extension.toml")
		if gotExecutable != "/usr/bin/git" {
			t.Fatalf("executable = %q, want /usr/bin/git", gotExecutable)
		}
		wantArgs := []string{
			"-c", "protocol.allow=never",
			"-c", "protocol.https.allow=always",
			"-c", "http.followRedirects=false",
			"-c", "http.proxy=",
			"-c", "http.curloptResolve=",
			"-c", "http.curloptResolve=+example.com:443:8.8.8.8",
			"-c", "credential.helper=",
			"-c", "credential.interactive=false",
			"-c", "core.hooksPath=" + os.DevNull,
			"clone", "--depth", "1", "--single-branch", "--branch", "v1.2.3", "--",
			"https://example.com/acme/fixture.git", gotArgs[len(gotArgs)-1],
		}
		if !reflect.DeepEqual(gotArgs, wantArgs) {
			t.Fatalf("git args = %#v, want %#v", gotArgs, wantArgs)
		}
	})

	t.Run("Should reject non HTTPS and private literal repository destinations", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			ref  string
			want error
		}{
			{name: "Should reject HTTP", ref: "http://example.com/acme/fixture.git"},
			{name: "Should reject SSH URLs", ref: "ssh://git@example.com/acme/fixture.git"},
			{name: "Should reject SCP syntax", ref: "git@example.com:acme/fixture.git"},
			{name: "Should reject local paths", ref: "../fixture"},
			{name: "Should reject empty credentials", ref: "https://@example.com/acme/fixture.git"},
			{name: "Should reject an empty query", ref: "https://example.com/acme/fixture.git?"},
			{name: "Should reject an empty fragment", ref: "https://example.com/acme/fixture.git#"},
			{name: "Should reject a trailing-dot host", ref: "https://example.com./acme/fixture.git"},
			{name: "Should reject a Unicode host", ref: "https://éxample.com/acme/fixture.git"},
			{
				name: "Should reject metadata IPs",
				ref:  "https://169.254.169.254/acme/fixture.git",
				want: outboundpolicy.ErrBlockedDestination,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				_, err := NewClient().Download(t.Context(), test.ref, registry.DownloadOpts{})
				if err == nil {
					t.Fatal("Download() error = nil, want destination rejection")
				}
				if !errors.Is(err, ErrInvalidRepositoryRef) {
					t.Fatalf("Download() error = %v, want ErrInvalidRepositoryRef", err)
				}
				if test.want != nil && !errors.Is(err, test.want) {
					t.Fatalf("Download() error = %v, want %v", err, test.want)
				}
			})
		}
	})

	t.Run("Should reject mixed public and private DNS before running git", func(t *testing.T) {
		t.Parallel()

		run := false
		client := NewClient(
			WithLookPath(func(string) (string, error) { return "/usr/bin/git", nil }),
			WithRunner(func(context.Context, string, ...string) error {
				run = true
				return nil
			}),
			withGitVersionProbe(supportedGitVersion),
			withRepositoryResolver(&staticRepositoryResolver{addresses: []netip.Addr{
				netip.MustParseAddr("8.8.8.8"),
				netip.MustParseAddr("10.0.0.8"),
			}}),
		)
		_, err := client.Download(t.Context(), "https://example.com/acme/fixture.git", registry.DownloadOpts{})
		if !errors.Is(err, outboundpolicy.ErrBlockedDestination) {
			t.Fatalf("Download() error = %v, want ErrBlockedDestination", err)
		}
		if !errors.Is(err, ErrRepositoryDestinationBlocked) {
			t.Fatalf("Download() error = %v, want ErrRepositoryDestinationBlocked", err)
		}
		if run {
			t.Fatal("git runner executed after a blocked DNS answer")
		}
	})

	t.Run("Should require Git 2.37 before resolving or cloning", func(t *testing.T) {
		t.Parallel()

		resolver := &staticRepositoryResolver{addresses: publicRepositoryAddresses()}
		run := false
		client := NewClient(
			WithLookPath(func(string) (string, error) { return "/usr/bin/git", nil }),
			WithRunner(func(context.Context, string, ...string) error {
				run = true
				return nil
			}),
			withGitVersionProbe(func(context.Context, string) (string, error) {
				return "git version 2.36.6", nil
			}),
			withRepositoryResolver(resolver),
		)
		_, err := client.Download(t.Context(), "https://example.com/acme/fixture.git", registry.DownloadOpts{})
		if !errors.Is(err, ErrGitVersionUnsupported) {
			t.Fatalf("Download() error = %v, want ErrGitVersionUnsupported", err)
		}
		if resolver.calls != 0 || run {
			t.Fatalf("resolver calls = %d, runner called = %v, want zero and false", resolver.calls, run)
		}
	})

	t.Run("Should reject compressed archives at the producer boundary and remove temp state", func(t *testing.T) {
		t.Parallel()

		archiveTempDir := t.TempDir()
		client := newFixtureGitClient(t, archiveTempDir, 1)
		_, err := client.Download(t.Context(), "https://example.com/acme/fixture.git", registry.DownloadOpts{
			MaxArchiveSize: 16,
		})
		if !errors.Is(err, registry.ErrArchiveTooLargeCompressed) {
			t.Fatalf("Download() error = %v, want ErrArchiveTooLargeCompressed", err)
		}
		assertEmptyDirectory(t, archiveTempDir)
	})

	t.Run("Should reject repository trees above the file count budget", func(t *testing.T) {
		t.Parallel()

		archiveTempDir := t.TempDir()
		client := newFixtureGitClient(t, archiveTempDir, 2)
		client.maxFileCount = 1
		_, err := client.Download(t.Context(), "https://example.com/acme/fixture.git", registry.DownloadOpts{})
		if !errors.Is(err, ErrRepositoryTooManyFiles) {
			t.Fatalf("Download() error = %v, want ErrRepositoryTooManyFiles", err)
		}
		assertEmptyDirectory(t, archiveTempDir)
	})

	t.Run("Should reject repository trees above the uncompressed byte budget", func(t *testing.T) {
		t.Parallel()

		archiveTempDir := t.TempDir()
		client := newFixtureGitClient(t, archiveTempDir, 1)
		client.maxUncompressedSize = 128
		_, err := client.Download(t.Context(), "https://example.com/acme/fixture.git", registry.DownloadOpts{})
		if !errors.Is(err, ErrRepositoryTooLarge) {
			t.Fatalf("Download() error = %v, want ErrRepositoryTooLarge", err)
		}
		assertEmptyDirectory(t, archiveTempDir)
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

func assertArchiveFile(t *testing.T, archive []byte, wantName string) {
	t.Helper()

	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	t.Cleanup(func() {
		if err := gzipReader.Close(); err != nil {
			t.Errorf("gzip.Reader.Close() error = %v", err)
		}
	})
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar.Reader.Next() error = %v", err)
		}
		if header.Name == wantName {
			return
		}
	}
	t.Fatalf("archive does not contain %q", wantName)
}

func newFixtureGitClient(t *testing.T, archiveTempDir string, fileCount int) *Client {
	t.Helper()

	client := NewClient(
		WithLookPath(func(string) (string, error) { return "/usr/bin/git", nil }),
		WithRunner(func(_ context.Context, _ string, args ...string) error {
			checkoutDir := args[len(args)-1]
			if err := os.MkdirAll(checkoutDir, 0o755); err != nil {
				return err
			}
			for index := range fileCount {
				name := filepath.Join(checkoutDir, fmt.Sprintf("fixture-%d.txt", index))
				if err := os.WriteFile(name, []byte(strings.Repeat("x", 256)), 0o600); err != nil {
					return err
				}
			}
			return nil
		}),
		withGitVersionProbe(supportedGitVersion),
		withRepositoryResolver(&staticRepositoryResolver{addresses: publicRepositoryAddresses()}),
	)
	client.archiveTempDir = archiveTempDir
	return client
}

type staticRepositoryResolver struct {
	addresses []netip.Addr
	err       error
	calls     int
}

func (r *staticRepositoryResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	r.calls++
	return append([]netip.Addr(nil), r.addresses...), r.err
}

func supportedGitVersion(context.Context, string) (string, error) {
	return "git version 2.37.0", nil
}

func publicRepositoryAddresses() []netip.Addr {
	return []netip.Addr{netip.MustParseAddr("8.8.8.8")}
}

func assertEmptyDirectory(t *testing.T, path string) {
	t.Helper()

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v", path, err)
	}
	if len(entries) != 0 {
		t.Fatalf("os.ReadDir(%q) returned %d entries, want empty", path, len(entries))
	}
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
