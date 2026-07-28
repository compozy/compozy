package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	httpArchiveRequestTimeout = 30 * time.Second
	httpArchiveMaxRedirects   = 10
)

// HTTPArchiveDownloader acquires one catalog-pinned archive over HTTPS.
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
	parsed, err := validateHTTPSArchiveURL(artifactURL)
	if err != nil {
		return nil, err
	}
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		return nil, errors.New("registry: HTTP archive version is required")
	}
	return &HTTPArchiveDownloader{
		artifactURL: parsed.String(),
		version:     trimmedVersion,
		client:      httpArchiveClient(client),
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
	response, err := d.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("registry: download curated HTTP archive for %q: %w", trimmedSlug, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		closeErr := response.Body.Close()
		return nil, errors.Join(
			fmt.Errorf(
				"registry: download curated HTTP archive for %q: status %s",
				trimmedSlug,
				response.Status,
			),
			wrapHTTPArchiveCloseError(trimmedSlug, closeErr),
		)
	}
	maxArchiveSize := opts.MaxArchiveSize
	if maxArchiveSize <= 0 {
		maxArchiveSize = DefaultMaxArchiveSize
	}
	if response.ContentLength > maxArchiveSize {
		closeErr := response.Body.Close()
		return nil, errors.Join(
			fmt.Errorf(
				"registry: download curated HTTP archive for %q: %w: size=%d limit=%d",
				trimmedSlug,
				ErrArchiveTooLargeCompressed,
				response.ContentLength,
				maxArchiveSize,
			),
			wrapHTTPArchiveCloseError(trimmedSlug, closeErr),
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

func validateHTTPSArchiveURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("registry: HTTP archive URL must be an absolute HTTPS URL without credentials")
	}
	if parsed.Fragment != "" {
		return nil, errors.New("registry: HTTP archive URL must not contain a fragment")
	}
	return parsed, nil
}

func httpArchiveClient(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{Timeout: httpArchiveRequestTimeout}
	}
	client := *base
	if client.Timeout <= 0 {
		client.Timeout = httpArchiveRequestTimeout
	}
	priorCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if request.URL.Scheme != "https" {
			return errors.New("registry: HTTP archive redirect must remain HTTPS")
		}
		if priorCheckRedirect != nil {
			return priorCheckRedirect(request, via)
		}
		if len(via) >= httpArchiveMaxRedirects {
			return errors.New("registry: HTTP archive stopped after 10 redirects")
		}
		return nil
	}
	return &client
}

func wrapHTTPArchiveCloseError(slug string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("registry: close curated HTTP archive response for %q: %w", slug, err)
}
