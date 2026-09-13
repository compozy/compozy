package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/registry"
)

func TestClientDownloadRevision(t *testing.T) {
	t.Parallel()
	t.Run("Should download the pinned repository archive without release lookup", func(t *testing.T) {
		t.Parallel()
		commit := strings.Repeat("b", 40)
		client := NewClient("", WithToken(""), WithHTTPClient(&http.Client{
			Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/repos/acme/demo/tarball/"+commit {
					t.Errorf("unexpected revision download path: %s", request.URL.Path)
				}
				response := newHTTPResponse(http.StatusOK, "archive bytes")
				response.Header.Set("Content-Type", "application/gzip")
				response.ContentLength = int64(len("archive bytes"))
				return response, nil
			}),
		}))
		result, err := client.DownloadRevision(t.Context(), "acme/demo", commit, 0)
		if err != nil {
			t.Fatal(err)
		}
		raw, readErr := io.ReadAll(result.Reader)
		closeErr := result.Reader.Close()
		if readErr != nil || closeErr != nil || string(raw) != "archive bytes" || result.Version != commit ||
			result.ContentSize != int64(len(raw)) {
			t.Fatalf("download = %+v, raw %q, read %v, close %v", result, raw, readErr, closeErr)
		}
		if _, err := client.DownloadRevision(t.Context(), "acme/demo", "main", 100); err == nil {
			t.Fatal("accepted an unpinned archive download")
		}
	})
}

func TestClientDownloadResponseCloseFailure(t *testing.T) {
	// not parallel: temporary-directory variables isolate the process-wide spool root.
	t.Run("Should remove the completed spool when response cleanup fails", func(t *testing.T) {
		temp := t.TempDir()
		t.Setenv("TMPDIR", temp)
		t.Setenv("TEMP", temp)
		t.Setenv("TMP", temp)
		closeFailure := errors.New("response close failed")
		client := NewClient("", WithToken(""), WithHTTPClient(&http.Client{
			Transport: stubRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/gzip"}},
					Body: &errorReadCloser{Reader: strings.NewReader("archive bytes"), closeErr: closeFailure},
				}, nil
			}),
		}))
		result, err := client.DownloadRevision(t.Context(), "acme/demo", strings.Repeat("a", 40), 100)
		if result != nil || !errors.Is(err, closeFailure) {
			t.Fatalf("failed cleanup = %+v, %v", result, err)
		}
		files, err := os.ReadDir(temp)
		if err != nil || len(files) != 0 {
			t.Fatalf("spool files after failure = %v, %v", files, err)
		}
	})
}

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
