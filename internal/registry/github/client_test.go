package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/outboundpolicy"
	"github.com/compozy/compozy/internal/registry"
)

func TestClientSearchFindsExtensionTopicRepositories(t *testing.T) {
	t.Parallel()

	t.Run("Should query the exact page for an aligned offset", func(t *testing.T) {
		t.Parallel()

		server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/search/repositories" {
				t.Fatalf("Search() path = %q, want /search/repositories", request.URL.Path)
			}
			if got := request.URL.Query().Get("q"); got != "bridge topic:compozy-extension" {
				t.Fatalf("Search() q = %q, want topic-qualified query", got)
			}
			if got := request.URL.Query().Get("per_page"); got != "5" {
				t.Fatalf("Search() per_page = %q, want 5", got)
			}
			if got := request.URL.Query().Get("page"); got != "3" {
				t.Fatalf("Search() page = %q, want 3", got)
			}
			writeJSON(
				writer,
				`{"items":[{"full_name":"acme/bridge","name":"bridge","description":"Bridge extension","owner":{"login":"acme"},"stargazers_count":42,"html_url":"https://github.com/acme/bridge","topics":["compozy-extension"],"default_branch":"main"}]}`,
			)
		})
		defer server.Close()

		items, err := NewClient(server.URL).Search(t.Context(), " bridge ", registry.SearchOpts{
			Limit:  5,
			Offset: 10,
			Type:   registry.PackageTypeExtension,
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("Search() items = %#v, want one", items)
		}
		want := registry.Listing{
			Slug: "acme/bridge", Name: "bridge", Description: "Bridge extension", Author: "acme",
			Downloads: 42, Source: "github", Type: registry.PackageTypeExtension,
		}
		if items[0] != want {
			t.Fatalf("Search() item = %#v, want %#v", items[0], want)
		}
	})

	t.Run("Should start at a partial offset and still return the requested limit", func(t *testing.T) {
		t.Parallel()

		server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
			if got := request.URL.Query().Get("per_page"); got != "5" {
				t.Fatalf("Search() per_page = %q, want 5", got)
			}
			switch request.URL.Query().Get("page") {
			case "1":
				writeJSON(writer, `{"items":[
					{"full_name":"acme/repo-0","name":"repo-0","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"},
					{"full_name":"acme/repo-1","name":"repo-1","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"},
					{"full_name":"acme/repo-2","name":"repo-2","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"},
					{"full_name":"acme/repo-3","name":"repo-3","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"},
					{"full_name":"acme/repo-4","name":"repo-4","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"}
				]}`)
			case "2":
				writeJSON(writer, `{"items":[
					{"full_name":"acme/repo-5","name":"repo-5","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"},
					{"full_name":"acme/repo-6","name":"repo-6","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"},
					{"full_name":"acme/repo-7","name":"repo-7","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"},
					{"full_name":"acme/repo-8","name":"repo-8","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"},
					{"full_name":"acme/repo-9","name":"repo-9","owner":{"login":"acme"},"topics":["compozy-extension"],"default_branch":"main"}
				]}`)
			default:
				t.Fatalf("Search() page = %q, want 1 or 2", request.URL.Query().Get("page"))
			}
		})
		defer server.Close()

		items, err := NewClient(server.URL).Search(t.Context(), "repo", registry.SearchOpts{
			Limit: 5, Offset: 2, Type: registry.PackageTypeExtension,
		})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(items) != 5 {
			t.Fatalf("Search() count = %d, want 5: %#v", len(items), items)
		}
		for index, item := range items {
			want := "acme/repo-" + strconv.Itoa(index+2)
			if item.Slug != want {
				t.Fatalf("Search() item[%d].Slug = %q, want %q", index, item.Slug, want)
			}
		}
	})
}

func TestClientCapabilities(t *testing.T) {
	t.Run("Should report repository search support", testClientCapabilities)
}

func testClientCapabilities(t *testing.T) {
	t.Parallel()

	if caps := NewClient("").Capabilities(); caps != (registry.SourceCaps{Search: true}) {
		t.Fatalf("Capabilities() = %#v, want search enabled", caps)
	}
}

func TestClientUploadRelease(t *testing.T) {
	t.Run("Should publish an archive and digest sidecar", testClientUploadRelease)
}

func testClientUploadRelease(t *testing.T) {
	t.Parallel()

	token := "publish-token-sentinel"
	uploaded := make(map[string]string)
	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("Authorization header missing bound credential")
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/repos/acme/demo/releases/tags/v1.2.3":
			writer.WriteHeader(http.StatusNotFound)
		case request.Method == http.MethodPost && request.URL.Path == "/repos/acme/demo/releases":
			writeJSON(writer,
				`{"id":7,"tag_name":"v1.2.3","html_url":"https://github.test/acme/demo/releases/v1.2.3",`+
					`"upload_url":"SERVER_URL/uploads/7{?name,label}","assets":[]}`,
			)
		case request.Method == http.MethodPost && request.URL.Path == "/uploads/7":
			payload, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("io.ReadAll(upload) error = %v", err)
			}
			name := request.URL.Query().Get("name")
			uploaded[name] = string(payload)
			writeJSON(writer,
				`{"id":9,"name":"`+name+`",`+
					`"browser_download_url":"https://github.test/downloads/`+name+`"}`,
			)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
	})
	defer server.Close()

	result, err := NewClient(server.URL, WithToken(token)).UploadRelease(t.Context(), ReleaseUploadRequest{
		Repository: "acme/demo", TagName: "v1.2.3", AssetName: "demo-v1.2.3.tar.gz",
		Archive: []byte("archive"), DigestSidecar: []byte("digest  demo-v1.2.3.tar.gz\n"),
	})
	if err != nil {
		t.Fatalf("UploadRelease() error = %v", err)
	}
	if result.ReleaseURL != "https://github.test/acme/demo/releases/v1.2.3" ||
		result.AssetURL != "https://github.test/downloads/demo-v1.2.3.tar.gz" {
		t.Fatalf("UploadRelease() = %#v", result)
	}
	if uploaded["demo-v1.2.3.tar.gz"] != "archive" ||
		uploaded["demo-v1.2.3.tar.gz.sha256"] != "digest  demo-v1.2.3.tar.gz\n" {
		t.Fatalf("uploaded assets = %#v, want archive and sidecar", uploaded)
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
			writeTestHTTPResponse(t, writer, []byte(`{
				"name":"Demo",
				"body":"Release notes",
				"tag_name":"v1.2.3",
				"draft":false,
				"prerelease":false,
				"tarball_url":"`+serverURLPlaceholder+`/downloads/source.tar.gz",
				"assets":[{"name":"demo-v1.2.3.tar.gz","url":"`+serverURLPlaceholder+`/downloads/asset.tar.gz","browser_download_url":"`+serverURLPlaceholder+`/downloads/asset-browser.tar.gz","content_type":"application/gzip","size":128,"download_count":7}],
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
			writeTestHTTPResponse(t, writer, []byte(`[
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
			writeTestHTTPResponse(t, writer, archive)
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

func TestClientDownloadUsesReleaseDigestSidecar(t *testing.T) {
	t.Run("Should bind the release digest sidecar to the download", testClientDownloadUsesReleaseDigestSidecar)
}

func testClientDownloadUsesReleaseDigestSidecar(t *testing.T) {
	t.Parallel()

	archive := mustTarGz(t, map[string]string{"demo/extension.toml": "name = \"demo\"\nversion = \"1.2.3\"\n"})
	digestBytes := sha256.Sum256(archive)
	digest := hex.EncodeToString(digestBytes[:])
	server := newGitHubServer(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/demo/releases/latest":
			writeJSON(writer, `{
				"tag_name":"v1.2.3",
				"assets":[
					{"name":"demo-v1.2.3.tar.gz","url":"`+serverURLPlaceholder+`/downloads/asset.tar.gz","content_type":"application/gzip","size":123},
					{"name":"demo-v1.2.3.tar.gz.sha256","url":"`+serverURLPlaceholder+`/downloads/asset.tar.gz.sha256","content_type":"text/plain","size":80}
				]
			}`)
		case "/downloads/asset.tar.gz.sha256":
			writer.Header().Set("Content-Type", "text/plain")
			if _, err := writer.Write([]byte(digest + "  demo-v1.2.3.tar.gz\n")); err != nil {
				t.Fatalf("writer.Write(sidecar) error = %v", err)
			}
		case "/downloads/asset.tar.gz":
			writer.Header().Set("Content-Type", "application/gzip")
			if _, err := writer.Write(archive); err != nil {
				t.Fatalf("writer.Write(archive) error = %v", err)
			}
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
	if result.ExpectedSHA256 != digest {
		t.Fatalf("Download() expected SHA-256 = %q, want %q", result.ExpectedSHA256, digest)
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
			writeTestHTTPResponse(t, writer, archive)
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
			writeTestHTTPResponse(t, writer, archive)
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
			writeTestHTTPResponse(t, writer, []byte("<html>login</html>"))
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

func TestClientReleaseMetadataResponseOwnership(t *testing.T) {
	t.Parallel()

	t.Run("Should bound metadata and close the response exactly once", func(t *testing.T) {
		t.Parallel()

		body := &countingReadCloser{
			Reader: strings.NewReader(strings.Repeat("x", int(maxReleaseMetadataBytes)+1)),
		}
		client := newReleaseResponseClient(body)
		_, err := client.fetchLatestRelease(t.Context(), repoSlug{owner: "acme", name: "demo", full: "acme/demo"})
		if !errors.Is(err, errReleaseMetadataTooLarge) {
			t.Fatalf("fetchLatestRelease() error = %v, want errReleaseMetadataTooLarge", err)
		}
		if body.closeCalls != 1 {
			t.Fatalf("response body close calls = %d, want 1", body.closeCalls)
		}
	})

	t.Run("Should preserve read drain and close failures", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("metadata read failed")
		drainErr := errors.New("metadata drain failed")
		closeErr := errors.New("metadata close failed")
		body := &scriptedErrorReadCloser{
			readErrors: []error{readErr, drainErr},
			closeErr:   closeErr,
		}
		client := newReleaseResponseClient(body)
		_, err := client.fetchLatestRelease(t.Context(), repoSlug{owner: "acme", name: "demo", full: "acme/demo"})
		for _, want := range []error{readErr, drainErr, closeErr} {
			if !errors.Is(err, want) {
				t.Fatalf("fetchLatestRelease() error = %v, want %v", err, want)
			}
		}
		if body.closeCalls != 1 {
			t.Fatalf("response body close calls = %d, want 1", body.closeCalls)
		}
	})
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

	policyAttempts := 0
	policyWaits := 0
	policyClient := NewClient(
		"https://example.com",
		WithHTTPClient(&http.Client{
			Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				policyAttempts++
				switch policyAttempts {
				case 1:
					return nil, errors.New("temporary transport failure")
				case 2:
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Status:     "429 Too Many Requests",
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"message":"try later"}`)),
						Request:    request,
					}, nil
				default:
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
				}
			}),
		}),
		WithRetryPolicy(time.Millisecond, time.Millisecond, 2),
		WithSleep(func(context.Context, time.Duration) error {
			policyWaits++
			return nil
		}),
	)
	_, err = policyClient.fetchLatestRelease(
		context.Background(),
		repoSlug{owner: "acme", name: "demo", full: "acme/demo"},
	)
	if err != nil {
		t.Fatalf("fetchLatestRelease(transport and 429) error = %v", err)
	}
	if policyAttempts != 3 || policyWaits != 2 {
		t.Fatalf(
			"transport and 429 policy attempts/waits = %d/%d, want 3/2",
			policyAttempts,
			policyWaits,
		)
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
	for _, version := range []string{"v1.2.3", "1.2.3"} {
		release, err := client.fetchRequestedRelease(
			context.Background(),
			repoSlug{owner: "acme", name: "demo", full: "acme/demo"},
			version,
		)
		if err != nil {
			t.Fatalf("fetchRequestedRelease(%q) error = %v", version, err)
		}
		if release.TagName != "v1.2.3" {
			t.Fatalf("fetchRequestedRelease(%q) tag = %q, want v1.2.3", version, release.TagName)
		}
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

	//nolint:bodyclose // checkRateLimit owns the rate-limit response body.
	response := newHTTPResponse(http.StatusForbidden, `{"message":"rate limit"}`)
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
		if err := response.Body.Close(); err != nil {
			t.Errorf("response.Body.Close() error = %v", err)
		}
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
		if err := response.Body.Close(); err != nil {
			t.Errorf("response.Body.Close() error = %v", err)
		}
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
		if err := response.Body.Close(); err != nil {
			t.Errorf("response.Body.Close() error = %v", err)
		}
	}()
	response.Header.Set("X-RateLimit-Remaining", "5")

	client := NewClient("", WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err := client.checkRateLimit(response); err != nil {
		t.Fatalf("checkRateLimit() error = %v, want nil", err)
	}
}

func TestWithHTTPClientPreservesInjectedTransportBehavior(t *testing.T) {
	t.Parallel()

	called := false
	httpClient := &http.Client{
		Timeout: 123 * time.Millisecond,
		Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
			called = true
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Status:     "204 No Content",
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    request,
			}, nil
		}),
	}
	client := NewClient("https://example.com", WithHTTPClient(httpClient))
	response, err := client.doRequest(t.Context(), "https://example.com/ping", acceptJSON)
	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("response.Body.Close() error = %v", err)
	}
	if !called || client.httpClient.Timeout != httpClient.Timeout {
		t.Fatalf(
			"injected client behavior called=%v timeout=%s, want called and %s",
			called,
			client.httpClient.Timeout,
			httpClient.Timeout,
		)
	}
}

func TestClientOutboundPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the default credential origins on the public network", func(t *testing.T) {
		t.Parallel()

		policy, err := newGitHubNetworkPolicy(defaultBaseURL)
		if err != nil {
			t.Fatalf("newGitHubNetworkPolicy() error = %v", err)
		}
		apiURL, err := url.Parse(defaultBaseURL)
		if err != nil {
			t.Fatalf("url.Parse(defaultBaseURL) error = %v", err)
		}
		if !policy.IsTrustedOrigin(apiURL) {
			t.Fatal("default API origin is not authorized to receive credentials")
		}
		err = policy.ValidateResolvedAddresses(
			[]netip.Addr{netip.MustParseAddr("127.0.0.1")},
			apiURL.Hostname(),
			"443",
		)
		if !errors.Is(err, outboundpolicy.ErrBlockedDestination) {
			t.Fatalf("default API private resolution error = %v, want ErrBlockedDestination", err)
		}
	})

	t.Run("Should attach the token only to the configured API origin", func(t *testing.T) {
		t.Parallel()

		client := NewClient("https://api.example.com", WithToken("secret-token"))
		trusted, err := client.newRequest(t.Context(), "https://api.example.com/repos/acme/demo", acceptJSON)
		if err != nil {
			t.Fatalf("newRequest(trusted) error = %v", err)
		}
		if got := trusted.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("trusted Authorization = %q, want bearer token", got)
		}
		untrusted, err := client.newRequest(t.Context(), "https://objects.example.com/archive.tar.gz", acceptBinary)
		if err != nil {
			t.Fatalf("newRequest(untrusted) error = %v", err)
		}
		if got := untrusted.Header.Get("Authorization"); got != "" {
			t.Fatalf("untrusted Authorization = %q, want empty", got)
		}
	})

	t.Run("Should not attach the token to an untrusted mutation origin", func(t *testing.T) {
		t.Parallel()

		client := NewClient(
			"https://api.example.com",
			WithToken("secret-token"),
			WithHTTPClient(&http.Client{
				Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
					if got := request.Header.Get("Authorization"); got != "" {
						t.Fatalf("untrusted mutation Authorization = %q, want empty", got)
					}
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Status:     "204 No Content",
						Header:     make(http.Header),
						Body:       http.NoBody,
						Request:    request,
					}, nil
				}),
			}),
		)
		response, err := client.doMutationRequest(
			t.Context(),
			http.MethodPost,
			"https://objects.example.com/upload",
			"application/octet-stream",
			[]byte("payload"),
		)
		if err != nil {
			t.Fatalf("doMutationRequest() error = %v", err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatalf("response.Body.Close() error = %v", err)
		}
	})

	t.Run("Should strip credentials on a cross-origin redirect", func(t *testing.T) {
		t.Parallel()

		calls := 0
		client := NewClient(
			"https://api.example.com",
			WithToken("secret-token"),
			WithHTTPClient(&http.Client{
				Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					switch request.URL.Hostname() {
					case "api.example.com":
						if got := request.Header.Get("Authorization"); got != "Bearer secret-token" {
							t.Fatalf("API Authorization = %q, want bearer token", got)
						}
						return &http.Response{
							StatusCode: http.StatusFound,
							Status:     "302 Found",
							Header:     http.Header{"Location": []string{"https://objects.example.com/archive.tar.gz"}},
							Body:       http.NoBody,
							Request:    request,
						}, nil
					case "objects.example.com":
						if got := request.Header.Get("Authorization"); got != "" {
							t.Fatalf("redirect Authorization = %q, want empty", got)
						}
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Status:     "204 No Content",
							Header:     make(http.Header),
							Body:       http.NoBody,
							Request:    request,
						}, nil
					default:
						return nil, fmt.Errorf("unexpected request host %q", request.URL.Hostname())
					}
				}),
			}),
		)
		response, err := client.doRequest(t.Context(), "https://api.example.com/archive", acceptBinary)
		if err != nil {
			t.Fatalf("doRequest() error = %v", err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatalf("response.Body.Close() error = %v", err)
		}
		if calls != 2 {
			t.Fatalf("transport calls = %d, want 2", calls)
		}
	})

	t.Run("Should strip credentials added by an inherited redirect callback", func(t *testing.T) {
		t.Parallel()

		calls := 0
		client := NewClient(
			"https://api.example.com",
			WithToken("secret-token"),
			WithHTTPClient(&http.Client{
				CheckRedirect: func(request *http.Request, _ []*http.Request) error {
					request.Header.Set("Authorization", "Bearer callback-token")
					return nil
				},
				Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					if request.URL.Hostname() == "api.example.com" {
						return &http.Response{
							StatusCode: http.StatusFound,
							Status:     "302 Found",
							Header: http.Header{
								"Location": []string{"https://objects.example.com/archive.tar.gz"},
							},
							Body:    http.NoBody,
							Request: request,
						}, nil
					}
					if got := request.Header.Get("Authorization"); got != "" {
						t.Fatalf("redirect Authorization = %q, want empty after inherited callback", got)
					}
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Status:     "204 No Content",
						Header:     make(http.Header),
						Body:       http.NoBody,
						Request:    request,
					}, nil
				}),
			}),
		)
		response, err := client.doRequest(t.Context(), "https://api.example.com/archive", acceptBinary)
		if err != nil {
			t.Fatalf("doRequest() error = %v", err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatalf("response.Body.Close() error = %v", err)
		}
		if calls != 2 {
			t.Fatalf("transport calls = %d, want 2", calls)
		}
	})

	t.Run("Should reject private metadata destinations before transport", func(t *testing.T) {
		t.Parallel()

		called := false
		client := NewClient(
			"https://api.example.com",
			WithToken("secret-token"),
			WithHTTPClient(&http.Client{Transport: stubRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				called = true
				return nil, errors.New("unexpected transport call")
			})}),
		)
		//nolint:bodyclose // A blocked destination cannot return a response.
		_, err := client.doRequest(t.Context(), "https://169.254.169.254/latest/meta-data", acceptBinary)
		if !errors.Is(err, outboundpolicy.ErrBlockedDestination) {
			t.Fatalf("doRequest() error = %v, want ErrBlockedDestination", err)
		}
		if called {
			t.Fatal("private destination reached the transport")
		}
	})
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

	closer := &errorReadCloser{Reader: strings.NewReader("response"), closeErr: errors.New("close failed")}
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
		writeTestHTTPResponse(t, writer, payload)
	}))
	return server
}

func writeJSON(writer http.ResponseWriter, body string) {
	writer.Header().Set("Content-Type", "application/json")
	if _, err := writer.Write([]byte(body)); err != nil {
		panic(fmt.Sprintf("write JSON test response: %v", err))
	}
}

func writeTestHTTPResponse(t *testing.T, writer http.ResponseWriter, payload []byte) {
	t.Helper()
	if _, err := writer.Write(payload); err != nil {
		t.Errorf("test response Write() error = %v", err)
	}
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

func responseErrorForTest(t *testing.T, statusCode int, body string, operation string, slug string) (err error) {
	t.Helper()
	response := newHTTPResponse(statusCode, body)
	defer func() {
		err = errors.Join(err, response.Body.Close())
	}()
	return responseError(response, operation, slug)
}

func doRequestErrorForTest(ctx context.Context, client *Client, rawURL string, accept string) error {
	response, err := client.doRequest(ctx, rawURL, accept)
	if response != nil {
		err = errors.Join(err, response.Body.Close())
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

type errorReadCloser struct {
	io.Reader
	closeErr error
}

func (r *errorReadCloser) Close() error {
	return r.closeErr
}

type countingReadCloser struct {
	io.Reader
	closeCalls int
}

func (r *countingReadCloser) Close() error {
	r.closeCalls++
	return nil
}

type scriptedErrorReadCloser struct {
	readErrors []error
	readCalls  int
	closeErr   error
	closeCalls int
}

func (r *scriptedErrorReadCloser) Read([]byte) (int, error) {
	if r.readCalls >= len(r.readErrors) {
		return 0, io.EOF
	}
	err := r.readErrors[r.readCalls]
	r.readCalls++
	return 0, err
}

func (r *scriptedErrorReadCloser) Close() error {
	r.closeCalls++
	return r.closeErr
}

func newReleaseResponseClient(body io.ReadCloser) *Client {
	return &Client{
		baseURL: "https://api.example.com",
		httpClient: &http.Client{Transport: stubRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       body,
				Request:    request,
			}, nil
		})},
	}
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
