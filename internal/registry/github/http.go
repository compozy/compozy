package github

import (
	"context"

	"errors"
	"fmt"
	"io"

	"net/http"

	"strconv"
	"strings"

	"time"

	"github.com/compozy/agh/internal/registry"
)

func (c *Client) doRequest(
	ctx context.Context,
	rawURL string,
	accept string,
) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("github: request aborted: %w", err)
	}
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("github: request URL is required")
	}

	backoff := c.initialBackoff
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		request, err := c.newRequest(ctx, rawURL, accept)
		if err != nil {
			return nil, err
		}

		response, err := c.httpClient.Do(request)
		if err != nil {
			lastErr, err = c.handleRequestError(ctx, err)
			if err != nil || attempt == c.maxRetries {
				if err != nil {
					return nil, err
				}
				return nil, lastErr
			}
			backoff, err = c.waitForRetry(ctx, backoff)
			if err != nil {
				return nil, err
			}
			continue
		}

		if err := c.checkRateLimit(response); err != nil {
			return nil, err
		}

		if shouldRetryStatus(response.StatusCode) && attempt < c.maxRetries {
			c.prepareRetryResponse(response, attempt)
			backoff, err = c.waitForRetry(ctx, backoff)
			if err != nil {
				return nil, err
			}
			continue
		}

		return response, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("github: request failed: %s", rawURL)
	}
	return nil, lastErr
}

func (c *Client) openDownloadResponse(
	ctx context.Context,
	repo repoSlug,
	release *release,
	selection releaseSelection,
) (*http.Response, string, int64, error) {
	switch {
	case selection.asset != nil:
		response, err := c.doRequest(
			ctx,
			firstNonEmpty(selection.asset.URL, selection.asset.BrowserDownloadURL),
			acceptBinary,
		)
		if err != nil {
			return nil, "", -1, fmt.Errorf("github: download asset for %q: %w", repo.full, err)
		}
		size := int64(-1)
		if selection.asset.Size > 0 {
			size = selection.asset.Size
		}
		return response, strings.TrimSpace(selection.asset.Digest), size, nil
	case selection.useTarball:
		response, err := c.doRequest(ctx, strings.TrimSpace(release.TarballURL), acceptBinary)
		if err != nil {
			return nil, "", -1, fmt.Errorf("github: download source archive for %q: %w", repo.full, err)
		}
		return response, "", -1, nil
	default:
		return nil, "", -1, fmt.Errorf("github: no download candidate resolved for %q", repo.full)
	}
}

func finalizeDownloadResponse(
	response *http.Response,
	slug string,
	contentSize int64,
	maxArchiveSize int64,
) (_ io.ReadCloser, contentType string, finalSize int64, err error) {
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		err := responseError(response, "download", slug)
		return nil, "", 0, joinErrors(
			err,
			closeResponseBody(response.Body, fmt.Sprintf("download response for %q", slug)),
		)
	}

	contentType = strings.TrimSpace(response.Header.Get("Content-Type"))
	if err := validateDownloadContentType(contentType); err != nil {
		return nil, "", 0, joinErrors(
			err,
			closeResponseBody(response.Body, fmt.Sprintf("download response for %q", slug)),
		)
	}

	if response.ContentLength > 0 {
		if response.ContentLength > maxArchiveSize {
			return nil, "", 0, joinErrors(
				fmt.Errorf(
					"github: download for %q: %w: size=%d limit=%d",
					slug,
					registry.ErrArchiveTooLargeCompressed,
					response.ContentLength,
					maxArchiveSize,
				),
				closeResponseBody(response.Body, fmt.Sprintf("download response for %q", slug)),
			)
		}
		contentSize = response.ContentLength
	}

	reader, written, err := spoolDownloadResponse(response.Body, slug, maxArchiveSize)
	closeErr := closeResponseBody(response.Body, fmt.Sprintf("download response for %q", slug))
	if err != nil {
		return nil, "", 0, joinErrors(fmt.Errorf("github: spool download for %q: %w", slug, err), closeErr)
	}
	if closeErr != nil {
		return nil, "", 0, closeErr
	}
	if contentSize <= 0 {
		contentSize = written
	}
	response.Body = http.NoBody

	return reader, contentType, contentSize, nil
}

func (c *Client) newRequest(ctx context.Context, rawURL string, accept string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("github: create request %q: %w", rawURL, err)
	}
	request.Header.Set("Accept", accept)
	if token := strings.TrimSpace(c.token); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request, nil
}

func (c *Client) handleRequestError(ctx context.Context, err error) (error, error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return nil, fmt.Errorf("github: request failed: %w", err)
	}
	return fmt.Errorf("github: request failed: %w", err), nil
}

func (c *Client) waitForRetry(ctx context.Context, backoff time.Duration) (time.Duration, error) {
	if err := c.sleep(ctx, backoff); err != nil {
		return 0, fmt.Errorf("github: retry wait aborted: %w", err)
	}
	return nextBackoff(backoff, c.maxBackoff), nil
}

func shouldRetryStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func (c *Client) prepareRetryResponse(response *http.Response, attempt int) {
	closeErr := closeResponseBody(
		response.Body,
		fmt.Sprintf("retry response for %s", requestURLString(response)),
	)
	if closeErr != nil && c.logger != nil {
		c.logger.Debug(
			"github: close response body before retry",
			"error",
			closeErr,
			"url",
			requestURLString(response),
			"attempt",
			attempt+1,
		)
	}
}

func (c *Client) checkRateLimit(response *http.Response) error {
	if response == nil {
		return nil
	}
	remainingValue := strings.TrimSpace(response.Header.Get("X-RateLimit-Remaining"))
	if remainingValue == "" {
		return nil
	}
	remaining, err := strconv.Atoi(remainingValue)
	if err != nil {
		if c.logger != nil {
			c.logger.Debug(
				"github: invalid X-RateLimit-Remaining header",
				"value",
				remainingValue,
				"error",
				err,
				"url",
				requestURLString(response),
			)
		}
		return nil
	}
	if remaining == 0 {
		if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests {
			rateLimitErr := errors.New("github: rate limit exceeded; set GITHUB_TOKEN for higher limits")
			return joinErrors(
				rateLimitErr,
				closeResponseBody(response.Body, fmt.Sprintf("rate limit response for %s", requestURLString(response))),
			)
		}
		if c.logger != nil {
			c.logger.Warn(
				"github: rate limit exhausted after successful response",
				"status", response.StatusCode,
				"url", requestURLString(response),
			)
		}
	}
	if remaining < rateLimitWarnThreshold && c.logger != nil {
		c.logger.Warn("github: rate limit running low", "remaining", remaining, "url", requestURLString(response))
	}
	return nil
}
