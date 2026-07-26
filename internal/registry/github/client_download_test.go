package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/registry"
)

func TestClientDownloadArchiveLimitContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject oversized archive content length before returning a download", func(t *testing.T) {
		t.Parallel()

		const limit int64 = 64
		body := strings.Repeat("a", int(limit)+128)
		server := newArchiveLimitGitHubServer(t, func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/gzip")
			writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
			writeGitHubLimitResponse(t, writer, body)
		})

		result, err := NewClient(server.URL).Download(
			context.Background(),
			"acme/demo",
			registry.DownloadOpts{MaxArchiveSize: limit},
		)
		if result != nil && result.Reader != nil {
			t.Cleanup(func() {
				if closeErr := result.Reader.Close(); closeErr != nil {
					t.Errorf("result.Reader.Close() error = %v", closeErr)
				}
			})
		}
		if !errors.Is(err, registry.ErrArchiveTooLargeCompressed) {
			t.Fatalf("Download(oversized content length) error = %v, want ErrArchiveTooLargeCompressed", err)
		}
	})

	t.Run("Should reject chunked downloads that cross the compressed limit", func(t *testing.T) {
		t.Parallel()

		const limit int64 = 64
		body := strings.Repeat("a", int(limit)+128)
		server := newArchiveLimitGitHubServer(t, func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/gzip")
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			writeGitHubLimitResponse(t, writer, body)
		})

		result, err := NewClient(server.URL).Download(
			context.Background(),
			"acme/demo",
			registry.DownloadOpts{MaxArchiveSize: limit},
		)
		if result != nil && result.Reader != nil {
			t.Cleanup(func() {
				if closeErr := result.Reader.Close(); closeErr != nil {
					t.Errorf("result.Reader.Close() error = %v", closeErr)
				}
			})
		}
		if !errors.Is(err, registry.ErrArchiveTooLargeCompressed) {
			t.Fatalf("Download(chunked oversized) error = %v, want ErrArchiveTooLargeCompressed", err)
		}
	})
}

func TestClientInfoMissingRepositoryContract(t *testing.T) {
	t.Parallel()

	t.Run("Should classify missing repositories as registry package not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/repos/acme/missing/releases/latest", "/repos/acme/missing/releases":
				http.NotFound(writer, request)
			default:
				t.Errorf("unexpected request path = %q", request.URL.Path)
				http.NotFound(writer, request)
			}
		}))
		t.Cleanup(server.Close)

		_, err := NewClient(server.URL).Info(context.Background(), "acme/missing")
		if !errors.Is(err, registry.ErrPackageNotFound) {
			t.Fatalf("Info() error = %v, want registry.ErrPackageNotFound", err)
		}
		if err == nil || !strings.Contains(err.Error(), "github: repository \"acme/missing\" not found") {
			t.Fatalf("Info() error = %v, want GitHub repository context", err)
		}
	})
}

func newArchiveLimitGitHubServer(
	t *testing.T,
	downloadHandler func(http.ResponseWriter, *http.Request),
) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			writeGitHubLimitResponse(t, writer, fmt.Sprintf(`{
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"%s/downloads/source.tar.gz",
				"assets":[{"name":"demo-v1.2.3.tar.gz","url":"%s/downloads/asset.tar.gz","content_type":"application/gzip","size":123}]
			}`, server.URL, server.URL))
		case "/downloads/asset.tar.gz":
			downloadHandler(writer, request)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func writeGitHubLimitResponse(t *testing.T, writer http.ResponseWriter, body string) {
	t.Helper()

	if _, err := writer.Write([]byte(body)); err != nil {
		t.Errorf("writer.Write() error = %v", err)
	}
}
