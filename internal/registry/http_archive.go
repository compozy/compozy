package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

const (
	httpArchiveRequestTimeout = 30 * time.Second
	httpArchiveMaxRedirects   = 10
	httpArchiveDrainLimit     = int64(64 << 10)
)

// HTTPArchiveDownloader acquires one catalog-pinned archive over HTTP(S).
type HTTPArchiveDownloader struct {
	artifactURL string
	version     string
	client      *http.Client
}

var _ Downloader = (*HTTPArchiveDownloader)(nil)

// NewHTTPArchiveDownloader constructs a downloader for one exact curated artifact.
func NewHTTPArchiveDownloader(
	artifactURL string,
	version string,
	client *http.Client,
) (*HTTPArchiveDownloader, error) {
	parsed, err := validateHTTPArchiveURL(artifactURL)
	if err != nil {
		return nil, err
	}
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		return nil, errors.New("registry: HTTP archive version is required")
	}
	policy := outboundpolicy.New(outboundpolicy.IsExplicitLoopbackHost(parsed.Hostname()))
	if err := policy.ValidateURL(parsed); err != nil {
		return nil, fmt.Errorf("registry: HTTP archive destination: %w", err)
	}
	return &HTTPArchiveDownloader{
		artifactURL: parsed.String(),
		version:     trimmedVersion,
		client:      httpArchiveClient(client, policy),
	}, nil
}

// Download fetches the configured artifact while preserving its feed-owned version.
func (d *HTTPArchiveDownloader) Download(
	ctx context.Context,
	slug string,
	opts DownloadOpts,
) (*DownloadResult, error) {
	if d == nil || d.client == nil {
		return nil, errors.New("registry: HTTP archive downloader is required")
	}
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return nil, errors.New("registry: HTTP archive slug is required")
	}
	if requested := strings.TrimSpace(opts.Version); requested != "" && requested != d.version {
		return nil, fmt.Errorf(
			"registry: HTTP archive version %q does not match curated version %q",
			requested,
			d.version,
		)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, d.artifactURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("registry: create HTTP archive request: %w", err)
	}
	request.Header.Set("Accept", "application/gzip, application/x-gzip, application/octet-stream")
	//nolint:bodyclose // Failure branches drain/close; successful reader ownership transfers to the caller.
	response, err := d.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("registry: download curated HTTP archive for %q: %w", trimmedSlug, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		cleanupErr := drainAndCloseHTTPArchiveResponse(response.Body, trimmedSlug)
		return nil, errors.Join(
			fmt.Errorf(
				"registry: download curated HTTP archive for %q: status %s",
				trimmedSlug,
				response.Status,
			),
			cleanupErr,
		)
	}
	maxArchiveSize := opts.MaxArchiveSize
	if maxArchiveSize <= 0 {
		maxArchiveSize = DefaultMaxArchiveSize
	}
	if response.ContentLength > maxArchiveSize {
		cleanupErr := drainAndCloseHTTPArchiveResponse(response.Body, trimmedSlug)
		return nil, errors.Join(
			fmt.Errorf(
				"registry: download curated HTTP archive for %q: %w: size=%d limit=%d",
				trimmedSlug,
				ErrArchiveTooLargeCompressed,
				response.ContentLength,
				maxArchiveSize,
			),
			cleanupErr,
		)
	}
	return &DownloadResult{
		Reader:      response.Body,
		Slug:        trimmedSlug,
		Version:     d.version,
		ContentSize: response.ContentLength,
		ContentType: strings.TrimSpace(response.Header.Get("Content-Type")),
	}, nil
}

func validateHTTPArchiveURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("registry: HTTP archive URL must be an absolute HTTPS URL without credentials")
	}
	if parsed.Scheme != "https" &&
		(parsed.Scheme != "http" || !outboundpolicy.IsExplicitLoopbackHost(parsed.Hostname())) {
		return nil, errors.New(
			"registry: HTTP archive URL must be an absolute HTTPS URL without credentials; " +
				"HTTP is allowed only for loopback hosts",
		)
	}
	if parsed.Fragment != "" {
		return nil, errors.New("registry: HTTP archive URL must not contain a fragment")
	}
	return parsed, nil
}

func httpArchiveClient(base *http.Client, policy outboundpolicy.Policy) *http.Client {
	return outboundpolicy.NewHTTPClient(base, policy, httpArchiveRequestTimeout, httpArchiveMaxRedirects)
}

func drainAndCloseHTTPArchiveResponse(body io.ReadCloser, slug string) error {
	if body == nil {
		return nil
	}
	var cleanupErr error
	if _, err := io.Copy(io.Discard, io.LimitReader(body, httpArchiveDrainLimit)); err != nil {
		cleanupErr = fmt.Errorf("registry: drain curated HTTP archive response for %q: %w", slug, err)
	}
	if err := body.Close(); err != nil {
		cleanupErr = errors.Join(
			cleanupErr,
			fmt.Errorf("registry: close curated HTTP archive response for %q: %w", slug, err),
		)
	}
	return cleanupErr
}
