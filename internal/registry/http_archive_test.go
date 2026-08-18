package registry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

func TestHTTPArchiveDownloader(t *testing.T) {
	t.Parallel()

	t.Run("Should download the exact HTTPS artifact and preserve catalog version metadata", func(t *testing.T) {
		t.Parallel()

		const archive = "curated archive"
		client := &http.Client{Transport: httpArchiveRoundTripper(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/repository-orientation-v1.0.0.tar.gz" {
				t.Fatalf("request.URL.Path = %q, want curated artifact path", request.URL.Path)
			}
			return archiveResponse(request, http.StatusOK, "application/gzip", archive), nil
		})}

		downloader, err := NewHTTPArchiveDownloader(
			"https://artifacts.example.test/repository-orientation-v1.0.0.tar.gz",
			"1.0.0",
			client,
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

		client := &http.Client{Transport: httpArchiveRoundTripper(func(request *http.Request) (*http.Response, error) {
			return redirectArchiveResponse(request, "http://127.0.0.1/artifact.tar.gz"), nil
		})}

		downloader, err := NewHTTPArchiveDownloader(
			"https://artifacts.example.test/artifact.tar.gz",
			"1.0.0",
			client,
		)
		if err != nil {
			t.Fatalf("NewHTTPArchiveDownloader() error = %v", err)
		}
		_, err = downloader.Download(context.Background(), "compozy/artifact", DownloadOpts{})
		if !errors.Is(err, outboundpolicy.ErrInsecureTransport) {
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

	t.Run("Should download an HTTP artifact from an explicit loopback host", func(t *testing.T) {
		t.Parallel()

		const archive = "local curated archive"
		client := &http.Client{Transport: httpArchiveRoundTripper(func(request *http.Request) (*http.Response, error) {
			return archiveResponse(request, http.StatusOK, "application/gzip", archive), nil
		})}
		downloader, err := NewHTTPArchiveDownloader(
			"http://127.0.0.1:42123/artifact.tar.gz",
			"1.0.0",
			client,
		)
		if err != nil {
			t.Fatalf("NewHTTPArchiveDownloader() error = %v", err)
		}
		result, err := downloader.Download(t.Context(), "compozy/local-artifact", DownloadOpts{})
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}
		payload, readErr := io.ReadAll(result.Reader)
		closeErr := result.Reader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatalf("read downloaded archive error = %v", err)
		}
		if string(payload) != archive {
			t.Fatalf("Download() payload = %q, want %q", payload, archive)
		}
	})

	t.Run("Should reject private HTTPS artifact destinations before making a request", func(t *testing.T) {
		t.Parallel()

		for _, client := range []*http.Client{
			nil,
			{Transport: httpArchiveRoundTripper(func(*http.Request) (*http.Response, error) {
				t.Fatal("transport received a request for a blocked destination")
				return nil, errors.New("unreachable")
			})},
		} {
			_, err := NewHTTPArchiveDownloader("https://169.254.169.254/archive.tar.gz", "1.0.0", client)
			if !errors.Is(err, outboundpolicy.ErrBlockedDestination) {
				t.Fatalf("NewHTTPArchiveDownloader() error = %v, want ErrBlockedDestination", err)
			}
		}
	})

	t.Run("Should reject an HTTPS redirect to an untrusted loopback origin", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{Transport: httpArchiveRoundTripper(func(request *http.Request) (*http.Response, error) {
			if request.URL.Host != "artifacts.example.test" {
				t.Fatal("private redirect target received a request")
			}
			return redirectArchiveResponse(request, "https://127.0.0.1/archive.tar.gz"), nil
		})}

		downloader, err := NewHTTPArchiveDownloader(
			"https://artifacts.example.test/archive.tar.gz",
			"1.0.0",
			client,
		)
		if err != nil {
			t.Fatalf("NewHTTPArchiveDownloader() error = %v", err)
		}
		_, err = downloader.Download(t.Context(), "compozy/artifact", DownloadOpts{})
		if !errors.Is(err, outboundpolicy.ErrBlockedDestination) {
			t.Fatalf("Download() error = %v, want ErrBlockedDestination", err)
		}
	})

	t.Run("Should preserve drain and close failures for rejected responses", func(t *testing.T) {
		t.Parallel()

		drainErr := errors.New("archive drain failed")
		closeErr := errors.New("archive close failed")
		body := &scriptedHTTPArchiveBody{readErr: drainErr, closeErr: closeErr}
		client := &http.Client{Transport: httpArchiveRoundTripper(func(request *http.Request) (*http.Response, error) {
			response := archiveResponse(request, http.StatusBadGateway, "text/plain", "")
			response.Body = body
			return response, nil
		})}
		downloader, err := NewHTTPArchiveDownloader(
			"https://artifacts.example.test/archive.tar.gz",
			"1.0.0",
			client,
		)
		if err != nil {
			t.Fatalf("NewHTTPArchiveDownloader() error = %v", err)
		}
		_, err = downloader.Download(t.Context(), "compozy/artifact", DownloadOpts{})
		for _, want := range []error{drainErr, closeErr} {
			if !errors.Is(err, want) {
				t.Fatalf("Download() error = %v, want %v", err, want)
			}
		}
		if body.closeCalls != 1 {
			t.Fatalf("response body close calls = %d, want 1", body.closeCalls)
		}
	})
}

type httpArchiveRoundTripper func(*http.Request) (*http.Response, error)

func (f httpArchiveRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func archiveResponse(
	request *http.Request,
	status int,
	contentType string,
	body string,
) *http.Response {
	return &http.Response{
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Request:    request,
		Status:     http.StatusText(status),
		StatusCode: status,
	}
}

func redirectArchiveResponse(request *http.Request, destination string) *http.Response {
	response := archiveResponse(request, http.StatusFound, "text/plain", "")
	response.Header.Set("Location", destination)
	return response
}

type scriptedHTTPArchiveBody struct {
	readErr    error
	closeErr   error
	closeCalls int
}

func (b *scriptedHTTPArchiveBody) Read([]byte) (int, error) {
	return 0, b.readErr
}

func (b *scriptedHTTPArchiveBody) Close() error {
	b.closeCalls++
	return b.closeErr
}
