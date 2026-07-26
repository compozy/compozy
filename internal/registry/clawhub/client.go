package clawhub

import (
	"bytes"

	"context"

	"errors"
	"fmt"

	"net/http"
	"net/url"

	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/registry"
)

const (
	sourceName            = "clawhub"
	defaultBaseURL        = "https://clawhub.ai/api/v1"
	defaultRequestTimeout = 30 * time.Second
	defaultInitialBackoff = time.Second
	defaultMaxBackoff     = 30 * time.Second
	defaultMaxRetries     = 3
	maxErrorBodyBytes     = 64 << 10
	maxJSONResponseBytes  = 1 << 20
	applicationGzipType   = "application/gzip"
)

var errResponseTooLarge = errors.New("clawhub: response exceeds max size")

// Option customizes a ClawHub client.
type Option func(*Client)

// Client implements the ClawHub registry source.
type Client struct {
	baseURL        string
	httpClient     *http.Client
	sleep          func(context.Context, time.Duration) error
	initialBackoff time.Duration
	maxBackoff     time.Duration
	maxRetries     int
	closeOnce      sync.Once
}

var _ registry.Source = (*Client)(nil)

// NewClient constructs a ClawHub registry client.
func NewClient(baseURL string, opts ...Option) *Client {
	client := &Client{
		baseURL:        strings.TrimSpace(baseURL),
		httpClient:     &http.Client{Timeout: defaultRequestTimeout},
		sleep:          sleepContext,
		initialBackoff: defaultInitialBackoff,
		maxBackoff:     defaultMaxBackoff,
		maxRetries:     defaultMaxRetries,
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
	client.baseURL = normalizeBaseURL(client.baseURL)

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

// Name reports the registry source name.
func (c *Client) Name() string {
	return sourceName
}

// Capabilities reports which registry operations ClawHub supports.
func (c *Client) Capabilities() registry.SourceCaps {
	return registry.SourceCaps{Search: true}
}

// Search queries ClawHub for skill listings.
func (c *Client) Search(ctx context.Context, query string, opts registry.SearchOpts) ([]registry.Listing, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return nil, errors.New("clawhub: search query is required")
	}
	if opts.Type == registry.PackageTypeExtension {
		return []registry.Listing{}, nil
	}

	values := url.Values{}
	values.Set("q", trimmedQuery)
	values.Set("type", string(registry.PackageTypeSkill))
	if opts.Limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Offset > 0 {
		values.Set("offset", fmt.Sprintf("%d", opts.Offset))
	}

	response, err := c.doRequest(ctx, "/search", values, "search", "")
	if err != nil {
		return nil, err
	}

	payload, err := readLimitedBody(response.Body, maxJSONResponseBytes)
	closeErr := response.Body.Close()
	if err != nil {
		return nil, joinErrors(
			fmt.Errorf("clawhub: read search response: %w", err),
			wrapSearchCloseError(closeErr),
		)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("clawhub: close search response: %w", closeErr)
	}

	listings, err := decodeListings(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("clawhub: decode search response: %w", err)
	}
	for index := range listings {
		listings[index].Source = c.Name()
		listings[index].Type = registry.PackageTypeSkill
	}

	return listings, nil
}

// Info fetches the full metadata for one skill slug.
func (c *Client) Info(ctx context.Context, slug string) (*registry.Detail, error) {
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return nil, errors.New("clawhub: skill slug is required")
	}

	response, err := c.doRequest(ctx, "/skills/"+url.PathEscape(trimmedSlug), nil, "info", trimmedSlug)
	if err != nil {
		return nil, err
	}

	payload, err := readLimitedBody(response.Body, maxJSONResponseBytes)
	closeErr := response.Body.Close()
	if err != nil {
		return nil, joinErrors(
			fmt.Errorf("clawhub: read info response for %q: %w", trimmedSlug, err),
			wrapCloseError(trimmedSlug, closeErr),
		)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("clawhub: close info response for %q: %w", trimmedSlug, closeErr)
	}

	detail, err := decodeDetail(payload, trimmedSlug)
	if err != nil {
		return nil, fmt.Errorf("clawhub: decode info response for %q: %w", trimmedSlug, err)
	}

	detail.Source = c.Name()
	detail.Type = registry.PackageTypeSkill
	return detail, nil
}

// Download fetches the archived skill package stream for one skill slug.
func (c *Client) Download(
	ctx context.Context,
	slug string,
	opts registry.DownloadOpts,
) (*registry.DownloadResult, error) {
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return nil, errors.New("clawhub: skill slug is required")
	}

	requestPath := "/skills/" + url.PathEscape(trimmedSlug) + "/download"
	if version := strings.TrimSpace(opts.Version); version != "" {
		requestPath = "/skills/" + url.PathEscape(trimmedSlug) + "/versions/" + url.PathEscape(version) + "/archive"
	}

	response, err := c.doRequest(ctx, requestPath, nil, "download", trimmedSlug)
	if err != nil {
		if errors.Is(err, registry.ErrPackageNotFound) {
			result, fallbackErr := c.downloadSkillArchiveFromInfo(ctx, trimmedSlug, opts, err)
			if fallbackErr == nil {
				return result, nil
			}
			return nil, fallbackErr
		}
		return nil, err
	}
	maxArchiveSize := normalizeArchiveSizeLimit(opts.MaxArchiveSize)
	if response.ContentLength > maxArchiveSize {
		closeErr := response.Body.Close()
		return nil, joinErrors(
			fmt.Errorf(
				"clawhub: download for %q: %w: size=%d limit=%d",
				trimmedSlug,
				registry.ErrArchiveTooLargeCompressed,
				response.ContentLength,
				maxArchiveSize,
			),
			wrapCloseError(trimmedSlug, closeErr),
		)
	}
	downloadReader, downloadSize, spoolErr := spoolDownloadResponse(response.Body, trimmedSlug, maxArchiveSize)
	closeErr := response.Body.Close()
	if spoolErr != nil {
		return nil, joinErrors(
			fmt.Errorf("clawhub: spool download for %q: %w", trimmedSlug, spoolErr),
			wrapCloseError(trimmedSlug, closeErr),
		)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("clawhub: close download response for %q: %w", trimmedSlug, closeErr)
	}

	return &registry.DownloadResult{
		Reader:      downloadReader,
		Slug:        trimmedSlug,
		Version:     strings.TrimSpace(response.Header.Get("X-Skill-Version")),
		ContentSize: contentSize(firstPositiveInt64(response.ContentLength, downloadSize)),
		ContentType: strings.TrimSpace(response.Header.Get("Content-Type")),
	}, nil
}

// Close releases any idle HTTP connections held by the client.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		closeIdleConnections(c.httpClient)
	})
	return nil
}
