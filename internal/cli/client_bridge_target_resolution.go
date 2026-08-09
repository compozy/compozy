package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *daemonClient) ResolveBridgeTarget(
	ctx context.Context,
	id string,
	name string,
) (record BridgeResolveTargetRecord, err error) {
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id)) + "/resolve"
	var response BridgeResolveTargetRecord
	requestBody := BridgeResolveTargetRequest{Name: strings.TrimSpace(name)}
	httpResponse, err := c.doRequest(
		ctx,
		http.MethodPost,
		path,
		nil,
		requestBody,
	)
	if err != nil {
		return BridgeResolveTargetRecord{}, err
	}
	defer func() {
		if closeErr := httpResponse.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: close %s %s response: %w", http.MethodPost, path, closeErr))
		}
	}()
	if httpResponse.StatusCode >= 200 && httpResponse.StatusCode < 300 {
		if err := decodeJSONResponseBody(http.MethodPost, path, httpResponse.Body, &response); err != nil {
			return BridgeResolveTargetRecord{}, err
		}
		return response, nil
	}
	if httpResponse.StatusCode == http.StatusNotFound || httpResponse.StatusCode == http.StatusUnprocessableEntity {
		body, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, apiErrorBodyLimit))
		drainErr := discardResponseBodyBounded(httpResponse.Body)
		if drainErr != nil {
			drainErr = fmt.Errorf("cli: drain bridge target resolve response: %w", drainErr)
		}
		if readErr != nil {
			return BridgeResolveTargetRecord{}, errors.Join(
				fmt.Errorf("cli: read bridge target resolve response: %w", readErr),
				drainErr,
			)
		}
		if drainErr != nil {
			return BridgeResolveTargetRecord{}, drainErr
		}
		if json.Unmarshal(body, &response) == nil && bridgeResolveTargetHasStructuredPayload(response) {
			return response, nil
		}
		return BridgeResolveTargetRecord{}, readAPIErrorBody(httpResponse.StatusCode, httpResponse.Status, body)
	}
	return BridgeResolveTargetRecord{}, readAPIError(httpResponse)
}

func bridgeResolveTargetHasStructuredPayload(response BridgeResolveTargetRecord) bool {
	return response.Diagnostic != nil ||
		response.Result.Match != nil ||
		response.Result.Ambiguous ||
		len(response.Result.Candidates) > 0 ||
		response.Result.Step != 0
}
