package registry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPArchiveDownloader(t *testing.T) {
	t.Parallel()

	t.Run("Should download the exact HTTPS artifact and preserve catalog version metadata", func(t *testing.T) {
		t.Parallel()

		const archive = "curated archive"
		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/repository-orientation-v1.0.0.tar.gz" {
				t.Fatalf("request.URL.Path = %q, want curated artifact path", request.URL.Path)
			}
			writer.Header().Set("Content-Type", "application/gzip")
			if _, err := writer.Write([]byte(archive)); err != nil {
				t.Fatalf("writer.Write() error = %v", err)
			}
		}))
		t.Cleanup(server.Close)

		downloader, err := NewHTTPArchiveDownloader(
			server.URL+"/repository-orientation-v1.0.0.tar.gz",
			"1.0.0",
			server.Client(),
		)
		if err != nil {
			t.Fatalf("NewHTTPArchiveDownloader() error = %v", err)
		}
		result, err := downloader.Download(
			context.Background(),
			"compozy/repository-orientation",
			DownloadOpts{Version: "1.0.0"},
		)
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}
		payload, readErr := io.ReadAll(result.Reader)
		closeErr := result.Reader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatalf("read downloaded archive error = %v", err)
		}
		if got := string(payload); got != archive {
			t.Fatalf("Download() payload = %q, want %q", got, archive)
		}
		if result.Slug != "compozy/repository-orientation" || result.Version != "1.0.0" {
			t.Fatalf("Download() result = %#v, want curated slug and version", result)
		}
		if result.ContentType != "application/gzip" {
			t.Fatalf("Download() ContentType = %q, want application/gzip", result.ContentType)
		}
	})

	t.Run("Should reject a redirect that leaves HTTPS", func(t *testing.T) {
		t.Parallel()

		insecure := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		t.Cleanup(insecure.Close)
		secure := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, insecure.URL+"/artifact.tar.gz", http.StatusFound)
		}))
		t.Cleanup(secure.Close)

		downloader, err := NewHTTPArchiveDownloader(secure.URL+"/artifact.tar.gz", "1.0.0", secure.Client())
		if err != nil {
			t.Fatalf("NewHTTPArchiveDownloader() error = %v", err)
		}
		_, err = downloader.Download(context.Background(), "compozy/artifact", DownloadOpts{})
		if err == nil || !strings.Contains(err.Error(), "redirect must remain HTTPS") {
			t.Fatalf("Download() error = %v, want HTTPS redirect rejection", err)
		}
	})

	t.Run("Should reject non HTTPS artifact roots before making a request", func(t *testing.T) {
		t.Parallel()

		_, err := NewHTTPArchiveDownloader("http://example.test/artifact.tar.gz", "1.0.0", nil)
		if err == nil || !strings.Contains(err.Error(), "HTTPS") {
			t.Fatalf("NewHTTPArchiveDownloader() error = %v, want HTTPS requirement", err)
		}
	})
}
