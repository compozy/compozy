package clawhub

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/registry"
)

func TestClientSearchParsesListingsAndLimit(t *testing.T) {
	t.Parallel()

	t.Run("Should search skills through the ClawHub search endpoint", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				t.Fatalf("request.Method = %q, want %q", request.Method, http.MethodGet)
			}
			if request.URL.Path != "/api/v1/search" {
				t.Fatalf("request.URL.Path = %q, want %q", request.URL.Path, "/api/v1/search")
			}
			if got := request.URL.Query().Get("q"); got != "agent" {
				t.Fatalf("query q = %q, want %q", got, "agent")
			}
			if got := request.URL.Query().Get("type"); got != "skill" {
				t.Fatalf("query type = %q, want %q", got, "skill")
			}
			if got := request.URL.Query().Get("limit"); got != "7" {
				t.Fatalf("query limit = %q, want %q", got, "7")
			}

			writer.Header().Set("Content-Type", "application/json")
			if _, err := writer.Write(
				[]byte(
					`{"results":[{"slug":"review","displayName":"Review","summary":"Review code","ownerHandle":"agh","tags":{"latest":"1.2.0"},"stats":{"downloads":42}}]}`,
				),
			); err != nil {
				t.Fatalf("write response: %v", err)
			}
		}))
		t.Cleanup(server.Close)

		client := NewClient(server.URL)

		listings, err := client.Search(context.Background(), "agent", registry.SearchOpts{Limit: 7})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(listings) != 1 {
			t.Fatalf("len(Search()) = %d, want 1", len(listings))
		}

		got := listings[0]
		if got.Slug != "review" || got.Name != "review" || got.Author != "agh" ||
			got.Description != "Review code" || got.Version != "1.2.0" || got.Downloads != 42 {
			t.Fatalf("Search() listing = %#v", got)
		}
		if got.Source != "clawhub" {
			t.Fatalf("Search() source = %q, want clawhub", got.Source)
		}
		if got.Type != registry.PackageTypeSkill {
			t.Fatalf("Search() type = %q, want %q", got.Type, registry.PackageTypeSkill)
		}
	})
}

func TestClientSearchEmptyResultsReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/search" {
			t.Fatalf("request.URL.Path = %q, want %q", request.URL.Path, "/api/v1/search")
		}
		if got := request.URL.Query().Get("type"); got != "skill" {
			t.Fatalf("query type = %q, want %q", got, "skill")
		}

		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write([]byte("{\"results\":[]}")); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)

	listings, err := client.Search(context.Background(), "missing", registry.SearchOpts{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if listings == nil {
		t.Fatal("Search() = nil, want empty slice")
	}
	if len(listings) != 0 {
		t.Fatalf("len(Search()) = %d, want 0", len(listings))
	}
}

func TestDecodeListingsSupportsCurrentItemsEnvelope(t *testing.T) {
	t.Parallel()

	t.Run("Should map current ClawHub items envelope into registry listings", func(t *testing.T) {
		t.Parallel()

		payload := strings.NewReader(`{
			"items": [
				{
					"slug": "clone-website",
					"displayName": "Clone, copy and duplicate any website into a clean project",
					"summary": "Clone any website into a project scaffold.",
					"tags": {"latest": "1.0.0"},
					"stats": {"downloads": 7, "installsAllTime": 11, "installsCurrent": 3}
				}
			]
		}`)
		listings, err := decodeListings(payload)
		if err != nil {
			t.Fatalf("decodeListings() error = %v", err)
		}
		if len(listings) != 1 {
			t.Fatalf("len(decodeListings()) = %d, want 1", len(listings))
		}

		got := listings[0]
		if got.Slug != "clone-website" {
			t.Fatalf("decodeListings() slug = %q, want clone-website", got.Slug)
		}
		if got.Name != "clone-website" {
			t.Fatalf("decodeListings() name = %q, want slug-derived name", got.Name)
		}
		if got.Description != "Clone any website into a project scaffold." {
			t.Fatalf("decodeListings() description = %q, want summary", got.Description)
		}
		if got.Version != "1.0.0" {
			t.Fatalf("decodeListings() version = %q, want tags.latest", got.Version)
		}
		if got.Downloads != 7 {
			t.Fatalf("decodeListings() downloads = %d, want stats.downloads", got.Downloads)
		}
	})
}

func TestClientInfoParsesSkillDetail(t *testing.T) {
	t.Parallel()

	t.Run("Should parse direct registry detail responses", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/api/v1/skills/@agh/review" {
				t.Fatalf("request.URL.Path = %q, want decoded scoped slug path", request.URL.Path)
			}
			if request.URL.RawPath != "/api/v1/skills/@agh%2Freview" {
				t.Fatalf("request.URL.RawPath = %q, want one escaped scoped slug segment", request.URL.RawPath)
			}

			writer.Header().Set("Content-Type", "application/json")
			if _, err := writer.Write([]byte(`{
				"slug":"@agh/review",
				"name":"Review",
				"description":"Review code",
				"author":"agh",
				"version":"1.2.0",
				"downloads":42,
				"readme":"# Review",
				"mcp_servers":["github"],
				"tags":["quality","code"]
			}`)); err != nil {
				t.Fatalf("writer.Write() error = %v", err)
			}
		}))
		t.Cleanup(server.Close)

		client := NewClient(server.URL)

		detail, err := client.Info(context.Background(), "@agh/review")
		if err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		if detail == nil {
			t.Fatal("Info() = nil, want detail")
		}
		if detail.Slug != "@agh/review" || detail.Name != "Review" || detail.Readme != "# Review" ||
			!slices.Equal(detail.MCPServers, []string{"github"}) ||
			!slices.Equal(detail.Tags, []string{"quality", "code"}) {
			t.Fatalf("Info() detail = %#v", detail)
		}
		if detail.Source != "clawhub" {
			t.Fatalf("Info() source = %q, want clawhub", detail.Source)
		}
		if detail.Type != registry.PackageTypeSkill {
			t.Fatalf("Info() type = %q, want %q", detail.Type, registry.PackageTypeSkill)
		}
	})

	t.Run("Should parse current ClawHub skill envelopes", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/api/v1/skills/review" {
				t.Fatalf("request.URL.Path = %q, want %q", request.URL.Path, "/api/v1/skills/review")
			}

			writer.Header().Set("Content-Type", "application/json")
			if _, err := writer.Write([]byte(`{
				"skill":{
					"slug":"review",
					"displayName":"Review",
					"summary":"Review code",
					"description":"# Review body",
					"stats":{"downloads":42}
				},
				"latestVersion":{"version":"1.2.0","license":"MIT"},
				"owner":{"handle":"ethagent"}
			}`)); err != nil {
				t.Fatalf("writer.Write() error = %v", err)
			}
		}))
		t.Cleanup(server.Close)

		client := NewClient(server.URL)

		detail, err := client.Info(context.Background(), "review")
		if err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		if detail == nil {
			t.Fatal("Info() = nil, want detail")
		}
		if detail.Slug != "review" || detail.Name != "review" || detail.Author != "ethagent" ||
			detail.Description != "Review code" || detail.Version != "1.2.0" ||
			detail.Downloads != 42 || detail.Readme != "# Review body" || detail.License != "MIT" {
			t.Fatalf("Info() detail = %#v", detail)
		}
		if detail.Source != "clawhub" {
			t.Fatalf("Info() source = %q, want clawhub", detail.Source)
		}
		if detail.Type != registry.PackageTypeSkill {
			t.Fatalf("Info() type = %q, want %q", detail.Type, registry.PackageTypeSkill)
		}
	})
}

func TestClientDownloadSynthesizesSkillArchiveFromInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should synthesize skill archives from info when the download endpoint is unavailable", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/api/v1/skills/review/download":
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusNotFound)
				if _, err := writer.Write([]byte(`{"error":"Not found"}`)); err != nil {
					t.Fatalf("writer.Write(download not found) error = %v", err)
				}
			case "/api/v1/skills/review":
				writer.Header().Set("Content-Type", "application/json")
				if _, err := writer.Write([]byte(`{
					"skill":{
						"slug":"review",
						"displayName":"Review",
						"summary":"Review code",
						"description":"---\nname: review\ndescription: Review code\n---\n# Review body\n",
						"stats":{"downloads":42}
					},
					"latestVersion":{"version":"1.0.0"},
					"owner":{"handle":"ethagent"}
				}`)); err != nil {
					t.Fatalf("writer.Write(info envelope) error = %v", err)
				}
			default:
				t.Fatalf("request.URL.Path = %q, want download or info path", request.URL.Path)
			}
		}))
		t.Cleanup(server.Close)

		result, err := NewClient(server.URL).Download(context.Background(), "review", registry.DownloadOpts{})
		if err != nil {
			t.Fatalf("Download() error = %v", err)
		}
		if result == nil || result.Reader == nil {
			t.Fatal("Download() result = nil, want synthesized archive")
		}
		t.Cleanup(func() {
			if err := result.Reader.Close(); err != nil {
				t.Errorf("result.Reader.Close() error = %v", err)
			}
		})

		if result.Slug != "review" || result.Version != "1.0.0" {
			t.Fatalf("Download() result = %#v, want raw slug and latest version", result)
		}
		if result.ContentType != "application/gzip" {
			t.Fatalf("Download() content type = %q, want application/gzip", result.ContentType)
		}
		files := readTarGz(t, result.Reader)
		if got := files["review/SKILL.md"]; got != "---\nname: review\ndescription: Review code\n---\n# Review body\n" {
			t.Fatalf("synthesized SKILL.md = %q, want ClawHub info description", got)
		}
	})
}

func TestClientDownloadPropagatesFallbackSynthesisErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should return synthesized archive validation failures from the info fallback", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/api/v1/skills/review/download":
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusNotFound)
				if _, err := writer.Write([]byte(`{"error":"Not found"}`)); err != nil {
					t.Fatalf("writer.Write(download not found) error = %v", err)
				}
			case "/api/v1/skills/review":
				writer.Header().Set("Content-Type", "application/json")
				if _, err := writer.Write([]byte(`{
					"skill":{
						"slug":"review",
						"displayName":"Review",
						"summary":"Review code",
						"description":"   ",
						"stats":{"downloads":42}
					},
					"latestVersion":{"version":"1.0.0"},
					"owner":{"handle":"ethagent"}
				}`)); err != nil {
					t.Fatalf("writer.Write(info envelope) error = %v", err)
				}
			default:
				t.Fatalf("request.URL.Path = %q, want download or info path", request.URL.Path)
			}
		}))
		t.Cleanup(server.Close)

		_, err := NewClient(server.URL).Download(context.Background(), "review", registry.DownloadOpts{})
		if err == nil {
			t.Fatal("Download() error = nil, want synthesized fallback failure")
		}
		if !errors.Is(err, registry.ErrPackageNotFound) {
			t.Fatalf("Download() error = %v, want ErrPackageNotFound ancestry", err)
		}
		if !strings.Contains(err.Error(), `skill "review" has no downloadable content`) {
			t.Fatalf("Download() error = %v, want synthesized archive detail", err)
		}
	})
}

func TestClientDownloadUsesLatestEndpointWhenVersionEmpty(t *testing.T) {
	t.Parallel()

	archive := mustTarGz(t, map[string]string{
		"review/SKILL.md": "---\nname: review\n---\n",
	})

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/skills/@agh/review/download" {
			t.Fatalf("request.URL.Path = %q, want decoded scoped slug download path", request.URL.Path)
		}
		if request.URL.RawPath != "/api/v1/skills/@agh%2Freview/download" {
			t.Fatalf("request.URL.RawPath = %q, want one escaped scoped slug download path", request.URL.RawPath)
		}

		writer.Header().Set("Content-Type", "application/gzip")
		writer.Header().Set("X-Skill-Version", "1.2.0")
		_, _ = writer.Write(archive)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	result, err := client.Download(context.Background(), "@agh/review", registry.DownloadOpts{})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	t.Cleanup(func() {
		if err := result.Reader.Close(); err != nil {
			t.Errorf("result.Reader.Close() error = %v", err)
		}
	})

	if result.Slug != "@agh/review" || result.Version != "1.2.0" {
		t.Fatalf("Download() result = %#v, want slug/version", result)
	}
	if result.ContentType != "application/gzip" {
		t.Fatalf("Download() content type = %q, want application/gzip", result.ContentType)
	}

	files := readTarGz(t, result.Reader)
	if got := files["review/SKILL.md"]; got != "---\nname: review\n---\n" {
		t.Fatalf("downloaded file = %q, want SKILL.md contents", got)
	}
}

func TestClientDownloadUsesVersionedEndpointWhenVersionSpecified(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/skills/@agh/review/versions/1.2.3/archive" {
			t.Fatalf(
				"request.URL.Path = %q, want decoded scoped slug version path %q",
				request.URL.Path,
				"/api/v1/skills/@agh/review/versions/1.2.3/archive",
			)
		}
		if request.URL.RawPath != "/api/v1/skills/@agh%2Freview/versions/1.2.3/archive" {
			t.Fatalf("request.URL.RawPath = %q, want one escaped scoped slug version path", request.URL.RawPath)
		}

		writer.Header().Set("Content-Type", "application/gzip")
		writer.Header().Set("X-Skill-Version", "1.2.3")
		_, _ = writer.Write(mustTarGz(t, map[string]string{"review/SKILL.md": "ok"}))
	}))
	defer server.Close()

	client := NewClient(server.URL)

	result, err := client.Download(context.Background(), "@agh/review", registry.DownloadOpts{
		Version: "1.2.3",
		Asset:   "ignored.tar.gz",
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	t.Cleanup(func() {
		if err := result.Reader.Close(); err != nil {
			t.Errorf("result.Reader.Close() error = %v", err)
		}
	})

	if result.Version != "1.2.3" {
		t.Fatalf("Download() version = %q, want 1.2.3", result.Version)
	}
}

func TestClientCapabilities(t *testing.T) {
	t.Parallel()

	if caps := NewClient("").Capabilities(); caps != (registry.SourceCaps{Search: true}) {
		t.Fatalf("Capabilities() = %#v, want search enabled", caps)
	}
}

func TestClientSearchReturnsEmptyForExtensionFilter(t *testing.T) {
	t.Parallel()

	client := NewClient("")

	listings, err := client.Search(
		context.Background(),
		"review",
		registry.SearchOpts{Type: registry.PackageTypeExtension},
	)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(listings) != 0 {
		t.Fatalf("len(Search()) = %d, want 0", len(listings))
	}
}

func TestClientRetriesHTTP500WithBackoff(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	var waits []time.Duration

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		current := attempts.Add(1)
		if current <= 3 {
			http.Error(writer, `{"error":"temporary failure"}`, http.StatusInternalServerError)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(
			[]byte(
				`{"skills":[{"slug":"@agh/review","name":"Review","description":"Review code","author":"agh","version":"1.2.0","downloads":42}]}`,
			),
		)
	}))
	defer server.Close()

	client := NewClient(
		server.URL,
		WithRetryPolicy(time.Millisecond, 4*time.Millisecond, 3),
		WithSleep(func(_ context.Context, wait time.Duration) error {
			waits = append(waits, wait)
			return nil
		}),
	)

	listings, err := client.Search(context.Background(), "review", registry.SearchOpts{})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("len(Search()) = %d, want 1", len(listings))
	}
	if got := attempts.Load(); got != 4 {
		t.Fatalf("attempts = %d, want 4 (initial + 3 retries)", got)
	}
	if !slices.Equal(waits, []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}) {
		t.Fatalf("waits = %#v, want [1ms 2ms 4ms]", waits)
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

func TestNewClientDefaults(t *testing.T) {
	t.Parallel()

	client := NewClient("", WithHTTPClient(nil), WithSleep(nil), WithRetryPolicy(0, 0, -1))
	if client.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL, defaultBaseURL)
	}
	if client.httpClient == nil {
		t.Fatal("httpClient = nil, want default client")
	}
	if client.sleep == nil {
		t.Fatal("sleep = nil, want default sleeper")
	}
	if client.initialBackoff != defaultInitialBackoff || client.maxBackoff != defaultMaxBackoff ||
		client.maxRetries != 0 {
		t.Fatalf(
			"retry config = (%s, %s, %d), want defaults with zero retries",
			client.initialBackoff,
			client.maxBackoff,
			client.maxRetries,
		)
	}
}

func TestDecodeListingsSupportsDirectArray(t *testing.T) {
	t.Parallel()

	listings, err := decodeListings(
		strings.NewReader(
			`[{"slug":"@agh/review","name":"Review","description":"Review code","author":"agh","version":"1.0.0","downloads":1}]`,
		),
	)
	if err != nil {
		t.Fatalf("decodeListings() error = %v", err)
	}
	if len(listings) != 1 || listings[0].Slug != "@agh/review" {
		t.Fatalf("decodeListings() = %#v, want one direct-array listing", listings)
	}
}

func TestDecodeListingsSupportsResultsEnvelope(t *testing.T) {
	t.Parallel()

	listings, err := decodeListings(strings.NewReader(`{"results":[{"slug":"@agh/review"}]}`))
	if err != nil {
		t.Fatalf("decodeListings() error = %v", err)
	}
	if len(listings) != 1 || listings[0].Slug != "@agh/review" {
		t.Fatalf("decodeListings() = %#v, want one results-envelope listing", listings)
	}
}

func TestClientTimeoutReturnsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"skills":[]}`))
	}))
	defer server.Close()

	client := NewClient(
		server.URL,
		WithHTTPClient(&http.Client{Timeout: 20 * time.Millisecond}),
		WithRetryPolicy(time.Millisecond, time.Millisecond, 0),
	)

	_, err := client.Search(context.Background(), "slow", registry.SearchOpts{})
	if err == nil {
		t.Fatal("Search() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Search() error = %v, want context deadline exceeded", err)
	}
}

func TestClientContextCancellationAbortsPromptly(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
	}))
	defer server.Close()

	client := NewClient(server.URL, WithRetryPolicy(time.Millisecond, time.Millisecond, 0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		<-started
		cancel()
		close(done)
	}()

	start := time.Now()
	_, err := client.Search(ctx, "cancel", registry.SearchOpts{})
	<-done
	if err == nil {
		t.Fatal("Search() error = nil, want cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Search() error = %v, want context canceled", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Search() took %s after cancellation, want prompt abort", elapsed)
	}
}

func TestClientSearchRejectsEmptyQuery(t *testing.T) {
	t.Parallel()

	client := NewClient("")

	_, err := client.Search(context.Background(), "   ", registry.SearchOpts{})
	if err == nil {
		t.Fatal("Search() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("Search() error = %v, want query validation", err)
	}
}

func TestClientInfoRejectsEmptySlug(t *testing.T) {
	t.Parallel()

	client := NewClient("")

	_, err := client.Info(context.Background(), "   ")
	if err == nil {
		t.Fatal("Info() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "skill slug is required") {
		t.Fatalf("Info() error = %v, want slug validation", err)
	}
}

func TestClientDownloadRejectsEmptySlug(t *testing.T) {
	t.Parallel()

	client := NewClient("")

	_, err := client.Download(context.Background(), "   ", registry.DownloadOpts{})
	if err == nil {
		t.Fatal("Download() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "skill slug is required") {
		t.Fatalf("Download() error = %v, want slug validation", err)
	}
}

func TestDecodeListingsRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := decodeListings(strings.NewReader(`{"skills":`))
	if err == nil {
		t.Fatal("decodeListings() error = nil, want invalid json")
	}
}

func TestResponseErrorUsesJSONAndPlainTextMessages(t *testing.T) {
	t.Parallel()

	notFound := responseErrorForTest(http.StatusNotFound, `{"error":"missing skill"}`, "info", "@agh/missing")
	if !strings.Contains(notFound.Error(), "skill not found") || !strings.Contains(notFound.Error(), "missing skill") {
		t.Fatalf("responseError(notFound) = %v, want not-found message", notFound)
	}

	internal := responseErrorForTest(http.StatusInternalServerError, "boom", "search", "")
	if !strings.Contains(internal.Error(), "boom") {
		t.Fatalf("responseError(internal) = %v, want body text", internal)
	}

	empty := responseErrorForTest(http.StatusBadGateway, "", "search", "")
	if !strings.Contains(empty.Error(), "Bad Gateway") {
		t.Fatalf("responseError(empty) = %v, want status text", empty)
	}
}

func TestReadErrorMessageHandlesEmptyAndStructuredBodies(t *testing.T) {
	t.Parallel()

	if got := readErrorMessage(io.NopCloser(strings.NewReader(""))); got != "" {
		t.Fatalf("readErrorMessage(empty) = %q, want empty", got)
	}
	if got := readErrorMessage(io.NopCloser(strings.NewReader(`{"message":"bad request"}`))); got != "bad request" {
		t.Fatalf("readErrorMessage(json) = %q, want %q", got, "bad request")
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

func TestDoRequestRejectsCanceledContextBeforeHTTP(t *testing.T) {
	t.Parallel()

	client := NewClient("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := doRequestErrorForTest(ctx, client, "/skills", nil, "search", "")
	if err == nil {
		t.Fatal("doRequest() error = nil, want canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("doRequest() error = %v, want context canceled", err)
	}
}

func TestDoRequestRejectsInvalidBaseURL(t *testing.T) {
	t.Parallel()

	client := NewClient("://bad url")

	err := doRequestErrorForTest(context.Background(), client, "/skills", nil, "search", "")
	if err == nil {
		t.Fatal("doRequest() error = nil, want invalid base url")
	}
	if !strings.Contains(err.Error(), "build search request URL") {
		t.Fatalf("doRequest() error = %v, want URL build context", err)
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

func TestHelperFunctions(t *testing.T) {
	t.Parallel()

	if got := contentSize(12); got != 12 {
		t.Fatalf("contentSize(12) = %d, want 12", got)
	}
	if got := contentSize(0); got != -1 {
		t.Fatalf("contentSize(0) = %d, want -1", got)
	}
	if got := firstNonEmpty("", " value "); got != "value" {
		t.Fatalf("firstNonEmpty() = %q, want value", got)
	}
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

func newHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func responseErrorForTest(statusCode int, body string, operation string, slug string) error {
	response := newHTTPResponse(statusCode, body)
	defer func() {
		_ = response.Body.Close()
	}()
	return responseError(response, operation, slug)
}

func doRequestErrorForTest(
	ctx context.Context,
	client *Client,
	requestPath string,
	query url.Values,
	operation string,
	slug string,
) error {
	response, err := client.doRequest(ctx, requestPath, query, operation, slug)
	if response != nil {
		_ = response.Body.Close()
	}
	return err
}

func readTarGz(t *testing.T, reader io.Reader) map[string]string {
	t.Helper()

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	t.Cleanup(func() {
		if err := gzipReader.Close(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
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
