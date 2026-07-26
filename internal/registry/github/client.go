package github

import (
	"context"

	"fmt"

	"log/slog"

	"net/http"

	"os"

	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/registry"
)

const (
	clientApplicationXGzipPath = "application/x-gzip"
)

const (
	clientApplicationGzipPath = "application/gzip"
)

const (
	defaultBaseURL          = "https://api.github.com"
	defaultRequestTimeout   = 30 * time.Second
	defaultInitialBackoff   = time.Second
	defaultMaxBackoff       = 30 * time.Second
	defaultMaxRetries       = 3
	maxErrorBodyBytes       = 64 << 10
	rateLimitWarnThreshold  = 10
	acceptJSON              = "application/vnd.github+json"
	acceptBinary            = "application/octet-stream"
	githubRepositoryBaseURL = "https://github.com"
)

// Option customizes a GitHub client.
type Option func(*Client)

// Client implements the GitHub Releases registry source.
type Client struct {
	baseURL        string
	httpClient     *http.Client
	sleep          func(context.Context, time.Duration) error
	initialBackoff time.Duration
	maxBackoff     time.Duration
	maxRetries     int
	logger         *slog.Logger
	token          string
	closeOnce      sync.Once
}

type repoSlug struct {
	owner string
	name  string
	full  string
}

type release struct {
	Name       string         `json:"name"`
	Body       string         `json:"body"`
	TagName    string         `json:"tag_name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	TarballURL string         `json:"tarball_url"`
	Assets     []releaseAsset `json:"assets"`
	Author     releaseAuthor  `json:"author"`
}

type releaseAuthor struct {
	Login string `json:"login"`
}

type releaseAsset struct {
	URL                string `json:"url"`
	Name               string `json:"name"`
	ContentType        string `json:"content_type"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
	DownloadCount      int    `json:"download_count"`
}

type releaseSelection struct {
	asset      *releaseAsset
	useTarball bool
}

var _ registry.Source = (*Client)(nil)

// NewClient constructs a GitHub Releases registry client.
func NewClient(baseURL string, opts ...Option) *Client {
	client := &Client{
		baseURL:        strings.TrimSpace(baseURL),
		httpClient:     &http.Client{Timeout: defaultRequestTimeout},
		sleep:          sleepContext,
		initialBackoff: defaultInitialBackoff,
		maxBackoff:     defaultMaxBackoff,
		maxRetries:     defaultMaxRetries,
		logger:         slog.Default(),
		token:          strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
	}
	if client.baseURL == "" {
		client.baseURL = defaultBaseURL
	}

	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}

	if strings.TrimSpace(client.baseURL) == "" {
		client.baseURL = defaultBaseURL
	}
	client.baseURL = strings.TrimRight(client.baseURL, "/")

	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: defaultRequestTimeout}
	}
	if client.sleep == nil {
		client.sleep = sleepContext
	}
	if client.initialBackoff <= 0 {
		client.initialBackoff = defaultInitialBackoff
	}
	if client.maxBackoff <= 0 {
		client.maxBackoff = defaultMaxBackoff
	}
	if client.maxRetries < 0 {
		client.maxRetries = 0
	}
	if client.logger == nil {
		client.logger = slog.Default()
	}

	return client
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

// WithSleep overrides the retry sleep function.
func WithSleep(sleep func(context.Context, time.Duration) error) Option {
	return func(client *Client) {
		client.sleep = sleep
	}
}

// WithRetryPolicy overrides the retry policy used for retryable responses.
func WithRetryPolicy(initial, maxDelay time.Duration, retries int) Option {
	return func(client *Client) {
		client.initialBackoff = initial
		client.maxBackoff = maxDelay
		client.maxRetries = retries
	}
}

// WithLogger overrides the logger used for rate-limit warnings.
func WithLogger(logger *slog.Logger) Option {
	return func(client *Client) {
		client.logger = logger
	}
}

// WithToken overrides the GitHub token used for authenticated requests.
func WithToken(token string) Option {
	return func(client *Client) {
		client.token = strings.TrimSpace(token)
	}
}

// Name reports the registry source name.
func (c *Client) Name() string {
	return "github"
}

// Capabilities reports which registry operations GitHub supports.
func (c *Client) Capabilities() registry.SourceCaps {
	return registry.SourceCaps{Search: false}
}

// Search reports that the GitHub Releases adapter is slug-only.
func (c *Client) Search(context.Context, string, registry.SearchOpts) ([]registry.Listing, error) {
	return nil, registry.ErrNotSupported
}

// Info fetches metadata from the latest published release and page one of the
// releases listing.
func (c *Client) Info(ctx context.Context, slug string) (*registry.Detail, error) {
	repo, err := parseRepoSlug(slug)
	if err != nil {
		return nil, err
	}

	latest, err := c.fetchLatestRelease(ctx, repo)
	if err != nil {
		return nil, err
	}
	releases, err := c.fetchReleasePage(ctx, repo)
	if err != nil {
		return nil, err
	}

	return &registry.Detail{
		Listing: registry.Listing{
			Slug:        repo.full,
			Name:        firstNonEmpty(latest.Name, repo.name),
			Description: releaseDescription(latest),
			Author:      firstNonEmpty(latest.Author.Login, repo.owner),
			Version:     strings.TrimSpace(latest.TagName),
			Downloads:   releaseDownloadCount(latest),
			Source:      c.Name(),
		},
		Readme:     strings.TrimSpace(latest.Body),
		Repository: githubRepositoryBaseURL + "/" + repo.full,
		Versions:   releaseVersions(releases),
	}, nil
}

// Download fetches the selected release archive stream.
func (c *Client) Download(
	ctx context.Context,
	slug string,
	opts registry.DownloadOpts,
) (*registry.DownloadResult, error) {
	repo, err := parseRepoSlug(slug)
	if err != nil {
		return nil, err
	}

	release, err := c.fetchRequestedRelease(ctx, repo, strings.TrimSpace(opts.Version))
	if err != nil {
		return nil, err
	}

	selection, err := selectReleaseDownload(release, strings.TrimSpace(opts.Asset))
	if err != nil {
		return nil, fmt.Errorf("github: resolve release asset for %q: %w", repo.full, err)
	}

	response, checksum, contentSize, err := c.openDownloadResponse(ctx, repo, release, selection)
	if err != nil {
		return nil, err
	}

	reader, contentType, contentSize, err := finalizeDownloadResponse(
		response,
		repo.full,
		contentSize,
		normalizeArchiveSizeLimit(opts.MaxArchiveSize),
	)
	if err != nil {
		return nil, err
	}

	return &registry.DownloadResult{
		Reader:      reader,
		Slug:        repo.full,
		Version:     strings.TrimSpace(release.TagName),
		ContentSize: contentSize,
		Checksum:    checksum,
		ContentType: contentType,
	}, nil
}

// Close releases any idle HTTP connections held by the client.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		closeIdleConnections(c.httpClient)
	})
	return nil
}
