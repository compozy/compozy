package clawhub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"strings"

	"github.com/compozy/agh/internal/registry"
)

func (c *Client) doRequest(
	ctx context.Context,
	requestPath string,
	query url.Values,
	operation string,
	slug string,
) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("clawhub: %s request aborted: %w", operation, err)
	}

	requestURL, err := c.buildURL(requestPath, query)
	if err != nil {
		return nil, fmt.Errorf("clawhub: build %s request URL: %w", operation, err)
	}

	backoff := c.initialBackoff
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("clawhub: create %s request: %w", operation, err)
		}
		request.Header.Set("Accept", "application/json, application/gzip, application/octet-stream")

		response, err := c.httpClient.Do(request)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil, fmt.Errorf("clawhub: %s request failed: %w", operation, err)
			}

			lastErr = fmt.Errorf("clawhub: %s request failed: %w", operation, err)
			if attempt == c.maxRetries {
				return nil, lastErr
			}

			if err := c.sleep(ctx, backoff); err != nil {
				return nil, fmt.Errorf("clawhub: %s retry wait aborted: %w", operation, err)
			}
			backoff = nextBackoff(backoff, c.maxBackoff)
			continue
		}

		if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
			return response, nil
		}

		lastErr = responseError(response, operation, slug)
		retryable := response.StatusCode >= http.StatusInternalServerError
		if attempt == c.maxRetries || !retryable {
			return nil, lastErr
		}

		if err := c.sleep(ctx, backoff); err != nil {
			return nil, fmt.Errorf("clawhub: %s retry wait aborted: %w", operation, err)
		}
		backoff = nextBackoff(backoff, c.maxBackoff)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("clawhub: %s request failed", operation)
	}

	return nil, lastErr
}

func responseError(response *http.Response, operation string, slug string) error {
	message := readErrorMessage(response.Body)

	if response.StatusCode == http.StatusNotFound && slug != "" {
		notFound := registry.NewPackageNotFoundError(slug)
		if message == "" {
			return fmt.Errorf("clawhub: skill not found: %w", notFound)
		}
		return fmt.Errorf("clawhub: skill not found: %w: %s", notFound, message)
	}

	if message == "" {
		return fmt.Errorf("clawhub: %s request failed: %s", operation, response.Status)
	}

	return fmt.Errorf("clawhub: %s request failed: %s: %s", operation, response.Status, message)
}

func readErrorMessage(body io.ReadCloser) string {
	payload, err := io.ReadAll(io.LimitReader(body, maxErrorBodyBytes))
	closeErr := body.Close()
	if err != nil {
		return ""
	}
	if closeErr != nil {
		return ""
	}

	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return ""
	}

	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil {
		for _, candidate := range []string{envelope.Error, envelope.Message, envelope.Detail} {
			if candidate = strings.TrimSpace(candidate); candidate != "" {
				return candidate
			}
		}
	}

	return trimmed
}

func readLimitedBody(body io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(body)
	}

	payload, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > maxBytes {
		return nil, fmt.Errorf("%w: limit=%d", errResponseTooLarge, maxBytes)
	}
	return payload, nil
}
