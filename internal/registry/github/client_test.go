package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/registry"
)

func TestClientSearchNotSupported(t *testing.T) {
	t.Parallel()

	_, err := NewClient("").Search(context.Background(), "anything", registry.SearchOpts{})
	if !errors.Is(err, registry.ErrNotSupported) {
		t.Fatalf("Search() error = %v, want ErrNotSupported", err)
	}
}

func TestClientCapabilities(t *testing.T) {
	t.Parallel()

	if caps := NewClient("").Capabilities(); caps != (registry.SourceCaps{Search: false}) {
		t.Fatalf("Capabilities() = %#v, want search disabled", caps)
	}
}

func TestClientInfoFetchesLatestAndVersions(t *testing.T) {
	t.Parallel()

	var latestCalls, releasesCalls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			latestCalls++
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{
				"name":"Demo",
				"body":"Release notes",
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"` + serverURLPlaceholder + `/downloads/source.tar.gz",
				"assets":[{"name":"demo-v1.2.3.tar.gz","url":"` + serverURLPlaceholder + `/downloads/asset.tar.gz","browser_download_url":"` + serverURLPlaceholder + `/downloads/asset-browser.tar.gz","content_type":"application/gzip","size":128,"download_count":7}],
				"author":{"login":"octocat"}
			}`))
		case "/repos/acme/demo/releases":
			releasesCalls++
			if got := request.URL.Query().Get("per_page"); got != "30" {
				t.Fatalf("per_page = %q, want 30", got)
			}
			if got := request.URL.Query().Get("page"); got != "1" {
				t.Fatalf("page = %q, want 1", got)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`[
				{"tag_name":"v1.2.3","draft":false,"prerelease":false},
				{"tag_name":"v1.2.2","draft":false,"prerelease":false},
				{"tag_name":"v1.2.1-rc1","draft":false,"prerelease":true},
				{"tag_name":"v1.2.0","draft":true,"prerelease":false}
			]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	detail, err := client.Info(context.Background(), "acme/demo")
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}

	if latestCalls != 1 || releasesCalls != 1 {
		t.Fatalf("Info() calls latest=%d releases=%d, want 1 each", latestCalls, releasesCalls)
	}
	if detail.Slug != "acme/demo" || detail.Name != "Demo" || detail.Author != "octocat" || detail.Version != "v1.2.3" {
		t.Fatalf("Info() detail = %#v", detail)
	}
	if got, want := detail.Repository, "https://github.com/acme/demo"; got != want {
		t.Fatalf("Info() repository = %q, want %q", got, want)
	}
	if got, want := strings.Join(detail.Versions, ","), "v1.2.3,v1.2.2"; got != want {
		t.Fatalf("Info() versions = %q, want %q", got, want)
	}
}

func TestClientDownloadSingleTarballAsset(t *testing.T) {
	t.Parallel()

	archive := mustTarGz(t, map[string]string{"demo/extension.toml": "name = \"demo\"\nversion = \"1.2.3\"\n"})
	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			writeJSON(writer, `{
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz",
				"assets":[{"name":"demo-v1.2.3.tar.gz","url":"`+serverURLPlaceholder+`/downloads/asset.tar.gz","browser_download_url":"`+serverURLPlaceholder+`/downloads/asset-browser.tar.gz","content_type":"application/gzip","size":123}]
			}`)
		case "/downloads/asset.tar.gz":
			writer.Header().Set("Content-Type", "application/gzip")
			_, _ = writer.Write(archive)
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Download(context.Background(), "acme/demo", registry.DownloadOpts{})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	t.Cleanup(func() {
		if err := result.Reader.Close(); err != nil {
			t.Errorf("result.Reader.Close() error = %v", err)
		}
	})

	if result.Version != "v1.2.3" || result.ContentType != "application/gzip" {
		t.Fatalf("Download() result = %#v", result)
	}
	files := readTarGz(t, result.Reader)
	if files["demo/extension.toml"] == "" {
		t.Fatalf("downloaded files = %#v, want extension.toml", files)
	}
}

func TestClientDownloadMultipleAssetsRequiresSelection(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			writeJSON(writer, `{
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz",
				"assets":[
					{"name":"demo-linux.tar.gz","url":"`+serverURLPlaceholder+`/downloads/linux.tar.gz","content_type":"application/gzip","size":123},
					{"name":"demo-darwin.tar.gz","url":"`+serverURLPlaceholder+`/downloads/darwin.tar.gz","content_type":"application/gzip","size":123}
				]
			}`)
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Download(context.Background(), "acme/demo", registry.DownloadOpts{})
	if err == nil {
		t.Fatal("Download() error = nil, want asset disambiguation failure")
	}
	if !strings.Contains(err.Error(), "demo-darwin.tar.gz") || !strings.Contains(err.Error(), "--asset") {
		t.Fatalf("Download() error = %v, want asset listing", err)
	}
}

func TestClientDownloadSelectsRequestedAsset(t *testing.T) {
	t.Parallel()

	archive := mustTarGz(t, map[string]string{"demo/extension.toml": "name = \"demo\"\nversion = \"1.2.3\"\n"})
	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			writeJSON(writer, `{
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz",
				"assets":[
					{"name":"demo-linux.tar.gz","url":"`+serverURLPlaceholder+`/downloads/linux.tar.gz","content_type":"application/gzip","size":123},
					{"name":"demo-darwin.tar.gz","url":"`+serverURLPlaceholder+`/downloads/darwin.tar.gz","content_type":"application/gzip","size":456}
				]
			}`)
		case "/downloads/darwin.tar.gz":
			writer.Header().Set("Content-Type", "application/gzip")
			_, _ = writer.Write(archive)
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Download(
		context.Background(),
		"acme/demo",
		registry.DownloadOpts{Asset: "demo-darwin.tar.gz"},
	)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	t.Cleanup(func() {
		if err := result.Reader.Close(); err != nil {
			t.Errorf("result.Reader.Close() error = %v", err)
		}
	})
	if result.ContentSize != int64(len(archive)) {
		t.Fatalf("Download() content size = %d, want %d", result.ContentSize, len(archive))
	}
}

func TestClientDownloadFallsBackToSourceArchive(t *testing.T) {
	t.Parallel()

	archive := mustTarGz(t, map[string]string{"demo-v1.2.3/extension.toml": "name = \"demo\"\nversion = \"1.2.3\"\n"})
	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			writeJSON(writer, `{
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz",
				"assets":[]
			}`)
		case "/downloads/source.tar.gz":
			writer.Header().Set("Content-Type", "application/x-gzip")
			_, _ = writer.Write(archive)
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Download(context.Background(), "acme/demo", registry.DownloadOpts{})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	t.Cleanup(func() {
		if err := result.Reader.Close(); err != nil {
			t.Errorf("result.Reader.Close() error = %v", err)
		}
	})
	if result.ContentType != "application/x-gzip" {
		t.Fatalf("Download() content type = %q, want application/x-gzip", result.ContentType)
	}
}

func TestClientDownloadRejectsUnexpectedContentType(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			writeJSON(writer, `{
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz",
				"assets":[{"name":"demo-v1.2.3.tar.gz","url":"`+serverURLPlaceholder+`/downloads/asset.tar.gz","content_type":"application/gzip","size":123}]
			}`)
		case "/downloads/asset.tar.gz":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = writer.Write([]byte("<html>login</html>"))
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Download(context.Background(), "acme/demo", registry.DownloadOpts{})
	if err == nil {
		t.Fatal("Download() error = nil, want content-type failure")
	}
	if !strings.Contains(err.Error(), "unexpected download content type") {
		t.Fatalf("Download() error = %v, want content-type failure", err)
	}
}

func TestClientDownloadJoinsCloseErrorOnContentTypeValidationFailure(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("asset close failed")
	client := NewClient(
		"https://example.com",
		WithHTTPClient(&http.Client{
			Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/repos/acme/demo/releases/latest":
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     "200 OK",
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body: io.NopCloser(strings.NewReader(`{
							"tag_name":"v1.2.3",
							"draft":false,
							"prerelease":false,
							"tarball_url":"https://example.com/downloads/source.tar.gz",
							"assets":[{"name":"demo-v1.2.3.tar.gz","url":"https://example.com/downloads/asset.tar.gz","content_type":"application/gzip","size":123}]
						}`)),
						Request: request,
					}, nil
				case "/downloads/asset.tar.gz":
					return &http.Response{
						StatusCode: http.StatusOK,
						Status:     "200 OK",
						Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
						Body: &errorReadCloser{
							Reader:   strings.NewReader("<html>login</html>"),
							closeErr: closeErr,
						},
						Request: request,
					}, nil
				default:
					return newHTTPResponse(http.StatusNotFound, `{"message":"not found"}`), nil
				}
			}),
		}),
	)

	_, err := client.Download(context.Background(), "acme/demo", registry.DownloadOpts{})
	if err == nil {
		t.Fatal("Download() error = nil, want content-type + close failure")
	}
	if !strings.Contains(err.Error(), "unexpected download content type") {
		t.Fatalf("Download() error = %v, want content-type failure", err)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("Download() error = %v, want joined close failure", err)
	}
}

func TestClientDownloadSurfacesHTTPFailuresBeforeContentTypeValidation(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			writeJSON(writer, `{
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz",
				"assets":[{"name":"demo-v1.2.3.tar.gz","url":"`+serverURLPlaceholder+`/downloads/asset.tar.gz","content_type":"application/gzip","size":123}]
			}`)
		case "/downloads/asset.tar.gz":
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.Error(writer, `{"message":"missing asset"}`, http.StatusNotFound)
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Download(context.Background(), "acme/demo", registry.DownloadOpts{})
	if err == nil {
		t.Fatal("Download() error = nil, want HTTP failure")
	}
	if !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "missing asset") {
		t.Fatalf("Download() error = %v, want download HTTP context", err)
	}
	if strings.Contains(err.Error(), "unexpected download content type") {
		t.Fatalf("Download() error = %v, want HTTP failure before content-type validation", err)
	}
}

func TestClientRateLimitExceeded(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-RateLimit-Remaining", "0")
		http.Error(writer, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Info(context.Background(), "acme/demo")
	if err == nil {
		t.Fatal("Info() error = nil, want rate-limit failure")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("Info() error = %v, want GITHUB_TOKEN hint", err)
	}
}

func TestClientPrivateRepositoryRequiresToken(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"Requires authentication"}`, http.StatusUnauthorized)
	})
	defer server.Close()

	client := NewClient(server.URL, WithToken(""))
	_, err := client.Info(context.Background(), "acme/private")
	if err == nil {
		t.Fatal("Info() error = nil, want authentication failure")
	}
	if !strings.Contains(err.Error(), "private repositories") {
		t.Fatalf("Info() error = %v, want private repo hint", err)
	}
}

func TestClientRepositoryWithoutReleases(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			http.NotFound(writer, request)
		case "/repos/acme/demo/releases":
			writeJSON(writer, `[]`)
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Info(context.Background(), "acme/demo")
	if err == nil {
		t.Fatal("Info() error = nil, want no releases failure")
	}
	if !strings.Contains(err.Error(), "no published releases") {
		t.Fatalf("Info() error = %v, want no releases message", err)
	}
}

func TestClientFallsBackToFirstPublishedReleaseWhenLatestEndpointIsMissing(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			http.NotFound(writer, request)
		case "/repos/acme/demo/releases":
			writeJSON(writer, `[
				{"tag_name":"v2.0.0","draft":false,"prerelease":false,"tarball_url":"`+serverURLPlaceholder+`/downloads/v2.0.0.tar.gz","assets":[]},
				{"tag_name":"v1.5.0","draft":false,"prerelease":false,"tarball_url":"`+serverURLPlaceholder+`/downloads/v1.5.0.tar.gz","assets":[]}
			]`)
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	release, err := client.fetchLatestRelease(
		context.Background(),
		repoSlug{owner: "acme", name: "demo", full: "acme/demo"},
	)
	if err != nil {
		t.Fatalf("fetchLatestRelease() error = %v", err)
	}
	if release.TagName != "v2.0.0" {
		t.Fatalf("fetchLatestRelease() tag = %q, want v2.0.0", release.TagName)
	}
}

func TestClientUsesGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-token")
	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		writeJSON(
			writer,
			`{"tag_name":"v1.2.3","draft":false,"prerelease":false,"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz","assets":[]}`,
		)
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.fetchLatestRelease(context.Background(), repoSlug{owner: "acme", name: "demo", full: "acme/demo"})
	if err != nil {
		t.Fatalf("fetchLatestRelease() error = %v", err)
	}
}

func TestClientRetriesHTTP500(t *testing.T) {
	t.Parallel()

	var attempts int
	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/acme/demo/releases/latest" {
			http.NotFound(writer, request)
			return
		}
		attempts++
		if attempts < 3 {
			http.Error(writer, `{"message":"temporary failure"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(
			writer,
			`{"tag_name":"v1.2.3","draft":false,"prerelease":false,"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz","assets":[]}`,
		)
	})
	defer server.Close()

	client := NewClient(
		server.URL,
		WithRetryPolicy(time.Millisecond, time.Millisecond, 2),
		WithSleep(func(context.Context, time.Duration) error { return nil }),
	)
	_, err := client.fetchLatestRelease(context.Background(), repoSlug{owner: "acme", name: "demo", full: "acme/demo"})
	if err != nil {
		t.Fatalf("fetchLatestRelease() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestClientRetriesHTTP500LogsCloseErrorBeforeRetry(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("retry close failed")
	var logBuffer bytes.Buffer
	attempts := 0

	client := NewClient(
		"https://example.com",
		WithHTTPClient(&http.Client{
			Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				attempts++
				if attempts == 1 {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Status:     "500 Internal Server Error",
						Header:     make(http.Header),
						Body: &errorReadCloser{
							Reader:   strings.NewReader(`{"message":"temporary failure"}`),
							closeErr: closeErr,
						},
						Request: request,
					}, nil
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"tag_name":"v1.2.3",
						"draft":false,
						"prerelease":false,
						"tarball_url":"https://example.com/downloads/source.tar.gz",
						"assets":[]
					}`)),
					Request: request,
				}, nil
			}),
		}),
		WithRetryPolicy(time.Millisecond, time.Millisecond, 1),
		WithSleep(func(context.Context, time.Duration) error { return nil }),
		WithLogger(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))),
	)

	_, err := client.fetchLatestRelease(context.Background(), repoSlug{owner: "acme", name: "demo", full: "acme/demo"})
	if err != nil {
		t.Fatalf("fetchLatestRelease() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}

	logs := logBuffer.String()
	if !strings.Contains(logs, "github: close response body before retry") {
		t.Fatalf("retry logs = %q, want close-before-retry message", logs)
	}
	if !strings.Contains(logs, "retry close failed") {
		t.Fatalf("retry logs = %q, want close failure details", logs)
	}
}

func TestNewClientDefaults(t *testing.T) {
	t.Parallel()

	client := NewClient(
		"",
		WithHTTPClient(nil),
		WithSleep(nil),
		WithRetryPolicy(0, 0, -1),
		WithLogger(nil),
		WithToken(" "),
	)
	if client.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL, defaultBaseURL)
	}
	if client.httpClient == nil {
		t.Fatal("httpClient = nil, want default client")
	}
	if client.sleep == nil {
		t.Fatal("sleep = nil, want default sleep")
	}
	if client.logger == nil {
		t.Fatal("logger = nil, want default logger")
	}
	if client.maxRetries != 0 {
		t.Fatalf("maxRetries = %d, want 0 after negative override normalization", client.maxRetries)
	}
}

func TestClientCloseSafeMultipleTimes(t *testing.T) {
	t.Parallel()

	client := NewClient("")
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestParseRepoSlugValidation(t *testing.T) {
	t.Parallel()

	if _, err := parseRepoSlug("acme/demo"); err != nil {
		t.Fatalf("parseRepoSlug() error = %v", err)
	}
	if _, err := parseRepoSlug("acme"); err == nil {
		t.Fatal("parseRepoSlug() error = nil, want invalid slug failure")
	}
}

func TestClientFetchRequestedReleaseByTag(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/tags/v1.2.3":
			writeJSON(
				writer,
				`{"tag_name":"v1.2.3","draft":false,"prerelease":false,"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz","assets":[]}`,
			)
		default:
			http.NotFound(writer, request)
		}
	})
	defer server.Close()

	client := NewClient(server.URL)
	release, err := client.fetchRequestedRelease(
		context.Background(),
		repoSlug{owner: "acme", name: "demo", full: "acme/demo"},
		"v1.2.3",
	)
	if err != nil {
		t.Fatalf("fetchRequestedRelease() error = %v", err)
	}
	if release.TagName != "v1.2.3" {
		t.Fatalf("fetchRequestedRelease() tag = %q, want v1.2.3", release.TagName)
	}
}

func TestClientFetchRequestedReleaseNotFound(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.fetchRequestedRelease(
		context.Background(),
		repoSlug{owner: "acme", name: "demo", full: "acme/demo"},
		"v9.9.9",
	)
	if err == nil {
		t.Fatal("fetchRequestedRelease() error = nil, want not found")
	}
	if !strings.Contains(err.Error(), "v9.9.9") {
		t.Fatalf("fetchRequestedRelease() error = %v, want version in error", err)
	}
}

func TestClientFetchRequestedReleaseRejectsPrerelease(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(
			writer,
			`{"tag_name":"v1.2.3-rc1","draft":false,"prerelease":true,"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz","assets":[]}`,
		)
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.fetchRequestedRelease(
		context.Background(),
		repoSlug{owner: "acme", name: "demo", full: "acme/demo"},
		"v1.2.3-rc1",
	)
	if err == nil {
		t.Fatal("fetchRequestedRelease() error = nil, want prerelease failure")
	}
	if !strings.Contains(err.Error(), "published full release") {
		t.Fatalf("fetchRequestedRelease() error = %v, want prerelease failure", err)
	}
}

func TestClientFetchReleasePageErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should not found", func(t *testing.T) {
		t.Parallel()

		server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
			http.NotFound(writer, request)
		})
		defer server.Close()

		client := NewClient(server.URL)
		_, err := client.fetchReleasePage(
			context.Background(),
			repoSlug{owner: "acme", name: "missing", full: "acme/missing"},
		)
		if err == nil {
			t.Fatal("fetchReleasePage() error = nil, want not found")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Fatalf("fetchReleasePage() error = %v, want not found", err)
		}
	})

	t.Run("Should unauthorized", func(t *testing.T) {
		t.Parallel()

		server := newGitHubServer(t, func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, `{"message":"Requires authentication"}`, http.StatusUnauthorized)
		})
		defer server.Close()

		client := NewClient(server.URL)
		_, err := client.fetchReleasePage(
			context.Background(),
			repoSlug{owner: "acme", name: "private", full: "acme/private"},
		)
		if err == nil {
			t.Fatal("fetchReleasePage() error = nil, want unauthorized")
		}
		if !strings.Contains(err.Error(), "private repositories") {
			t.Fatalf("fetchReleasePage() error = %v, want private repo hint", err)
		}
	})
}

func TestClientFetchLatestReleaseUnauthorized(t *testing.T) {
	t.Parallel()

	server := newGitHubServer(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"Requires authentication"}`, http.StatusUnauthorized)
	})
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.fetchLatestRelease(
		context.Background(),
		repoSlug{owner: "acme", name: "private", full: "acme/private"},
	)
	if err == nil {
		t.Fatal("fetchLatestRelease() error = nil, want unauthorized")
	}
	if !strings.Contains(err.Error(), "private repositories") {
		t.Fatalf("fetchLatestRelease() error = %v, want private repo hint", err)
	}
}

func TestResponseErrorUsesJSONAndPlainTextMessages(t *testing.T) {
	t.Parallel()

	jsonError := responseErrorForTest(
		t,
		http.StatusInternalServerError,
		`{"message":"boom"}`,
		"latest release",
		"acme/demo",
	)
	if !strings.Contains(jsonError.Error(), "boom") {
		t.Fatalf("responseError(json) = %v, want message", jsonError)
	}

	textError := responseErrorForTest(t, http.StatusBadGateway, "gateway", "latest release", "acme/demo")
	if !strings.Contains(textError.Error(), "gateway") {
		t.Fatalf("responseError(text) = %v, want body", textError)
	}
}

func TestReadErrorMessageHandlesEmptyAndStructuredBodies(t *testing.T) {
	t.Parallel()

	if got := readErrorMessage(io.NopCloser(strings.NewReader(""))); got != "" {
		t.Fatalf("readErrorMessage(empty) = %q, want empty", got)
	}
	if got := readErrorMessage(io.NopCloser(strings.NewReader(`{"message":"bad request"}`))); got != "bad request" {
		t.Fatalf("readErrorMessage(json) = %q, want bad request", got)
	}
}

func TestSleepContextReturnsOnCancelAndZeroWait(t *testing.T) {
	t.Parallel()

	if err := sleepContext(context.Background(), 0); err != nil {
		t.Fatalf("sleepContext(zero) error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepContext(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("sleepContext(canceled) error = %v, want context canceled", err)
	}
}

func TestNextBackoffRespectsDefaultsAndMax(t *testing.T) {
	t.Parallel()

	if got := nextBackoff(0, 0); got != defaultInitialBackoff {
		t.Fatalf("nextBackoff(0,0) = %s, want %s", got, defaultInitialBackoff)
	}
	if got := nextBackoff(2*time.Second, 3*time.Second); got != 3*time.Second {
		t.Fatalf("nextBackoff(2s,3s) = %s, want 3s", got)
	}
}

func TestValidateDownloadContentType(t *testing.T) {
	t.Parallel()

	for _, contentType := range []string{"application/gzip", "application/x-gzip", "application/octet-stream"} {
		if err := validateDownloadContentType(contentType); err != nil {
			t.Fatalf("validateDownloadContentType(%q) error = %v", contentType, err)
		}
	}
	if err := validateDownloadContentType("text/html"); err == nil {
		t.Fatal("validateDownloadContentType(text/html) error = nil, want failure")
	}
	if err := validateDownloadContentType(""); err == nil {
		t.Fatal("validateDownloadContentType(\"\") error = nil, want failure")
	}
}

func TestReleaseDescriptionFallbacks(t *testing.T) {
	t.Parallel()

	if got := releaseDescription(&release{Name: "Demo"}); got != "Demo" {
		t.Fatalf("releaseDescription(name) = %q, want Demo", got)
	}
	if got := releaseDescription(&release{Body: "Line one\nLine two"}); got != "Line one" {
		t.Fatalf("releaseDescription(body) = %q, want Line one", got)
	}
}

func TestDoRequestRejectsEmptyURL(t *testing.T) {
	t.Parallel()

	client := NewClient("")
	err := doRequestErrorForTest(context.Background(), client, "", acceptJSON)
	if err == nil {
		t.Fatal("doRequest() error = nil, want empty URL failure")
	}
}

func TestDoRequestRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	client := NewClient("")
	err := doRequestErrorForTest(context.Background(), client, "://bad", acceptJSON)
	if err == nil {
		t.Fatal("doRequest() error = nil, want invalid URL failure")
	}
}

func TestCheckRateLimitErrorsWhenRemainingZero(t *testing.T) {
	t.Parallel()

	response := newHTTPResponse(http.StatusForbidden, `{"message":"rate limit"}`)
	defer func() {
		_ = response.Body.Close()
	}()
	response.Header.Set("X-RateLimit-Remaining", "0")

	client := NewClient("")
	if err := client.checkRateLimit(response); err == nil {
		t.Fatal("checkRateLimit() error = nil, want rate-limit failure")
	}
}

func TestCheckRateLimitAllowsSuccessfulResponsesThatConsumeFinalQuota(t *testing.T) {
	t.Parallel()

	var logBuffer bytes.Buffer
	response := newHTTPResponse(http.StatusOK, `{"ok":true}`)
	defer func() {
		_ = response.Body.Close()
	}()
	response.Header.Set("X-RateLimit-Remaining", "0")

	client := NewClient(
		"",
		WithLogger(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelWarn}))),
	)
	if err := client.checkRateLimit(response); err != nil {
		t.Fatalf("checkRateLimit() error = %v, want nil", err)
	}
	if !strings.Contains(logBuffer.String(), "rate limit exhausted after successful response") {
		t.Fatalf("rate limit logs = %q, want exhausted-after-success warning", logBuffer.String())
	}
}

func TestCheckRateLimitLogsInvalidRemainingHeader(t *testing.T) {
	t.Parallel()

	var logBuffer bytes.Buffer
	response := newHTTPResponse(http.StatusOK, "")
	defer func() {
		_ = response.Body.Close()
	}()
	response.Header.Set("X-RateLimit-Remaining", "bogus")

	client := NewClient(
		"",
		WithLogger(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))),
	)
	if err := client.checkRateLimit(response); err != nil {
		t.Fatalf("checkRateLimit() error = %v, want nil", err)
	}

	logs := logBuffer.String()
	if !strings.Contains(logs, "invalid X-RateLimit-Remaining") {
		t.Fatalf("rate limit logs = %q, want invalid-header message", logs)
	}
}

func TestCheckRateLimitJoinsCloseErrorWhenRemainingZero(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("rate limit close failed")
	response := &http.Response{
		StatusCode: http.StatusForbidden,
		Status:     http.StatusText(http.StatusForbidden),
		Body: &errorReadCloser{
			Reader:   strings.NewReader(`{"message":"rate limit"}`),
			closeErr: closeErr,
		},
		Header:  make(http.Header),
		Request: &http.Request{URL: mustParseURL("https://api.github.com/repos/acme/demo/releases/latest")},
	}
	response.Header.Set("X-RateLimit-Remaining", "0")

	client := NewClient("")
	err := client.checkRateLimit(response)
	if err == nil {
		t.Fatal("checkRateLimit() error = nil, want rate-limit failure")
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("checkRateLimit() error = %v, want GITHUB_TOKEN hint", err)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("checkRateLimit() error = %v, want joined close failure", err)
	}
}

func TestCheckRateLimitWarnsWithoutFailing(t *testing.T) {
	t.Parallel()

	response := newHTTPResponse(http.StatusOK, "")
	defer func() {
		_ = response.Body.Close()
	}()
	response.Header.Set("X-RateLimit-Remaining", "5")

	client := NewClient("", WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err := client.checkRateLimit(response); err != nil {
		t.Fatalf("checkRateLimit() error = %v, want nil", err)
	}
}

func TestWithHTTPClientOverridesClient(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Timeout: 123 * time.Millisecond}
	client := NewClient("", WithHTTPClient(httpClient))
	if client.httpClient != httpClient {
		t.Fatal("WithHTTPClient() did not override http client")
	}
}

func TestSelectReleaseDownloadErrors(t *testing.T) {
	t.Parallel()

	_, err := selectReleaseDownload(&release{
		TarballURL: "",
		Assets: []releaseAsset{
			{Name: "demo.zip"},
		},
	}, "")
	if err == nil {
		t.Fatal("selectReleaseDownload() error = nil, want missing archive failure")
	}

	_, err = selectReleaseDownload(&release{
		TarballURL: serverURLPlaceholder,
		Assets: []releaseAsset{
			{Name: "demo.zip"},
		},
	}, "demo.zip")
	if err == nil {
		t.Fatal("selectReleaseDownload(requested zip) error = nil, want non-tar.gz failure")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	t.Parallel()

	if got := firstNonEmpty("", " value "); got != "value" {
		t.Fatalf("firstNonEmpty() = %q, want value", got)
	}
}

func TestCloseResponseBodyAndJoinErrors(t *testing.T) {
	t.Parallel()

	if err := closeResponseBody(nil, "nil body"); err != nil {
		t.Fatalf("closeResponseBody(nil) error = %v", err)
	}

	closer := &stubCloser{err: errors.New("close failed")}
	err := closeResponseBody(closer, "test response")
	if err == nil || !strings.Contains(err.Error(), "close failed") {
		t.Fatalf("closeResponseBody() error = %v, want close failure", err)
	}

	joined := joinErrors(errors.New("one"), nil, errors.New("two"))
	if joined == nil || !strings.Contains(joined.Error(), "one") || !strings.Contains(joined.Error(), "two") {
		t.Fatalf("joinErrors() = %v, want joined error", joined)
	}
}

func newGitHubServer(t *testing.T, handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		recorder := httptest.NewRecorder()
		handler(recorder, request)
		response := recorder.Result()
		defer func() {
			if err := response.Body.Close(); err != nil {
				t.Fatalf("response.Body.Close() error = %v", err)
			}
		}()

		for key, values := range response.Header {
			for _, value := range values {
				writer.Header().Add(key, value)
			}
		}
		writer.WriteHeader(response.StatusCode)

		payload, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("io.ReadAll(response.Body) error = %v", err)
		}
		payload = bytes.ReplaceAll(payload, []byte(serverURLPlaceholder), []byte(server.URL))
		_, _ = writer.Write(payload)
	}))
	return server
}

func writeJSON(writer http.ResponseWriter, body string) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(body))
}

const serverURLPlaceholder = "SERVER_URL"

func newHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    &http.Request{URL: mustParseURL("https://api.github.com/repos/acme/demo/releases/latest")},
	}
}

func responseErrorForTest(t *testing.T, statusCode int, body string, operation string, slug string) error {
	t.Helper()
	response := newHTTPResponse(statusCode, body)
	defer func() {
		_ = response.Body.Close()
	}()
	return responseError(response, operation, slug)
}

func doRequestErrorForTest(ctx context.Context, client *Client, rawURL string, accept string) error {
	response, err := client.doRequest(ctx, rawURL, accept)
	if response != nil {
		_ = response.Body.Close()
	}
	return err
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}

type stubCloser struct {
	err error
}

func (s *stubCloser) Close() error {
	return s.err
}

type errorReadCloser struct {
	io.Reader
	closeErr error
}

func (r *errorReadCloser) Close() error {
	return r.closeErr
}

type stubRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f stubRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func mustTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", name, err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}

	return buffer.Bytes()
}

func readTarGz(t *testing.T, reader io.Reader) map[string]string {
	t.Helper()

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	t.Cleanup(func() {
		if err := gzipReader.Close(); err != nil {
			t.Errorf("gzipReader.Close() error = %v", err)
		}
	})

	tarReader := tar.NewReader(gzipReader)
	files := make(map[string]string)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return files
		}
		if err != nil {
			t.Fatalf("tarReader.Next() error = %v", err)
		}

		payload, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("io.ReadAll(%q) error = %v", header.Name, err)
		}
		files[header.Name] = string(payload)
	}
}
