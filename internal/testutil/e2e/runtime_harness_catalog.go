package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// SessionCatalogStreamResult is one completed catalog-stream observation.
type SessionCatalogStreamResult struct {
	Records []SSEEvent
	Err     error
}

// StartSessionCatalogHTTPStream subscribes before returning, then reads until predicate matches.
func (h *RuntimeHarness) StartSessionCatalogHTTPStream(
	ctx context.Context,
	predicate func(SSEEvent) bool,
) (stream <-chan SessionCatalogStreamResult, err error) {
	if err := validateSSEPredicate(predicate); err != nil {
		return nil, err
	}
	targetURL := h.HTTPURL("/api/sessions/catalog-stream")
	response, requestErr := doRequest(ctx, h.HTTPClient, targetURL, http.MethodGet, nil)
	if requestErr != nil {
		return nil, requestErr
	}
	ownsBody := true
	defer func() {
		if ownsBody {
			err = errors.Join(err, response.Body.Close())
		}
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		statusErr := fmt.Errorf("catalog stream status %d: %s", response.StatusCode, bytes.TrimSpace(payload))
		return nil, errors.Join(statusErr, readErr)
	}

	result := make(chan SessionCatalogStreamResult, 1)
	go func() {
		records, readErr := readSSERecordsUntil(response.Body, predicate)
		result <- SessionCatalogStreamResult{
			Records: records,
			Err:     errors.Join(readErr, response.Body.Close()),
		}
		close(result)
	}()
	ownsBody = false
	return result, nil
}
