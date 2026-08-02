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

	"github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/retry"
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
	if c.networkErr != nil {
		return nil, c.networkErr
	}

	response, err := retry.DoValue(
		ctx,
		c.requestRetryPolicy(operation),
		retryClawHubRequest,
		func(ctx context.Context) (*http.Response, error) {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
			if err != nil {
				return nil, &clawHubRequestAttemptError{
					cause: fmt.Errorf("clawhub: create %s request: %w", operation, err),
				}
			}
			request.Header.Set("Accept", "application/json, application/gzip, application/octet-stream")

			response, err := c.httpClient.Do(request)
			if err != nil {
				return nil, newClawHubTransportAttemptError(ctx, operation, err)
			}
			if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
				return response, nil
			}

			return nil, &clawHubRequestAttemptError{
				cause:     responseError(response, operation, slug),
				retryable: response.StatusCode >= http.StatusInternalServerError,
			}
		},
	)
	if err != nil {
		return nil, finishClawHubRequest(ctx, operation, err)
	}
	return response, nil
}

func responseError(response *http.Response, operation string, slug string) error {
	message, readErr := readErrorMessage(response.Body)
	cleanupErr := drainAndCloseResponseBody(response.Body, operation+" error response")

	var requestErr error
	switch {
	case response.StatusCode == http.StatusNotFound && slug != "":
		notFound := registry.NewPackageNotFoundError(slug)
		if message == "" {
			requestErr = fmt.Errorf("clawhub: skill not found: %w", notFound)
		} else {
			requestErr = fmt.Errorf("clawhub: skill not found: %w: %s", notFound, message)
		}
	case message == "":
		requestErr = fmt.Errorf("clawhub: %s request failed: %s", operation, response.Status)
	default:
		requestErr = fmt.Errorf("clawhub: %s request failed: %s: %s", operation, response.Status, message)
	}
	if readErr != nil {
		readErr = fmt.Errorf("clawhub: read %s error response: %w", operation, readErr)
	}
	return errors.Join(requestErr, readErr, cleanupErr)
}

func readErrorMessage(body io.Reader) (string, error) {
	payload, err := io.ReadAll(io.LimitReader(body, maxErrorBodyBytes))
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return "", nil
	}

	var envelope struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil {
		for _, candidate := range []string{envelope.Error, envelope.Message, envelope.Detail} {
			if candidate = strings.TrimSpace(candidate); candidate != "" {
				return candidate, nil
			}
		}
	}

	return trimmed, nil
}

func readLimitedBody(body io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = maxJSONResponseBytes
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
