package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/bridgesdk"
)

func doLinearGraphQL[T any](
	ctx context.Context,
	c *linearClient,
	request linearGraphQLRequest,
	commitPolicy bridgesdk.HTTPResponseCommitPolicy,
) (result T, err error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return result, fmt.Errorf("linear: marshal graphql request: %w", err)
	}

	if !validLinearCredentialedURL(c.cfg.apiBaseURL) {
		return result, &bridgesdk.PermanentError{Err: errors.New("linear: api base url is invalid")}
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.graphqlURL(), bytes.NewReader(payload))
	if err != nil {
		return result, fmt.Errorf("linear: build graphql request: %w", err)
	}
	token, err := c.authTokenForRequest(ctx)
	if err != nil {
		return result, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+token)

	httpResponse, err := bridgesdk.CredentialedHTTPClient(c.httpClient).Do(httpRequest)
	if err != nil {
		return result, classifyLinearTransportError(err)
	}
	commitEvidence := bridgesdk.HTTPResponseCommitUnconfirmed
	if httpResponse.StatusCode >= http.StatusOK && httpResponse.StatusCode < http.StatusMultipleChoices {
		commitEvidence = commitPolicy.Evidence()
	}
	defer func() {
		err = bridgesdk.FinalizeHTTPResponseBody(httpResponse.Body, err, commitEvidence, nil)
	}()

	body, readErr := readLinearHTTPResponseBody(httpResponse, "graphql")
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return result, errors.Join(classifyLinearHTTPError(httpResponse.StatusCode, body), readErr)
	}
	if readErr != nil {
		return result, readErr
	}

	envelope := linearGraphQLResponse[T]{}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return result, fmt.Errorf("linear: decode graphql response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, item := range envelope.Errors {
			if text := strings.TrimSpace(item.Message); text != "" {
				messages = append(messages, text)
			}
		}
		graphqlErr := &bridgesdk.PermanentError{
			Err: fmt.Errorf("linear: graphql error: %s", strings.Join(messages, "; ")),
		}
		return result, graphqlErr
	}

	return envelope.Data, nil
}
