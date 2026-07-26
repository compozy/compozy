package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridgesdk"
)

const slackAPIResponseLimit = 1 << 20

type slackBotClient struct {
	baseURL               string
	botToken              string
	httpClient            *http.Client
	reportResponseCleanup func(error)
}

func (c *slackBotClient) callSlack(
	ctx context.Context,
	method string,
	payload any,
	result any,
	responseHeaders *http.Header,
	commitPolicy bridgesdk.HTTPResponseCommitPolicy,
) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil {
		return errors.New("slack: api client is required")
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal %s payload: %w", method, err)
	}
	endpoint := strings.TrimRight(strings.TrimSpace(c.baseURL), "/") + "/" + strings.TrimSpace(method)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: build %s request: %w", method, err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.botToken))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	response, err := bridgesdk.CredentialedHTTPClient(c.httpClient).Do(request)
	if err != nil {
		return err
	}
	commitEvidence := bridgesdk.HTTPResponseCommitUnconfirmed
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		commitEvidence = commitPolicy.Evidence()
	}
	defer func() {
		err = bridgesdk.FinalizeHTTPResponseBody(
			response.Body,
			err,
			commitEvidence,
			c.reportResponseCleanup,
		)
	}()

	responseBody, responseBodyErr := readSlackAPIResponseBody(method, response.Body)

	retryAfter := parseRetryAfter(response.Header.Get("Retry-After"))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return classifySlackNonSuccessResponse(
			method,
			response.StatusCode,
			retryAfter,
			responseBody,
			responseBodyErr,
		)
	}
	if responseBodyErr != nil {
		return responseBodyErr
	}
	explicitlyRejected, responseErr := decodeSlackSuccessResponse(
		method,
		response.StatusCode,
		retryAfter,
		responseBody,
		result,
		responseHeaders,
		response.Header,
	)
	if explicitlyRejected {
		commitEvidence = bridgesdk.HTTPResponseCommitUnconfirmed
	}
	return responseErr
}

func decodeSlackSuccessResponse(
	method string,
	statusCode int,
	retryAfter time.Duration,
	responseBody []byte,
	result any,
	responseHeaders *http.Header,
	headers http.Header,
) (bool, error) {
	var envelope slackAPIEnvelope
	var envelopeStatus struct {
		OK *bool `json:"ok"`
	}
	if len(bytes.TrimSpace(responseBody)) > 0 {
		if err := json.Unmarshal(responseBody, &envelope); err != nil {
			return false, fmt.Errorf("slack: decode %s response: %w", method, err)
		}
		if err := json.Unmarshal(responseBody, &envelopeStatus); err != nil {
			return false, fmt.Errorf("slack: decode %s response status: %w", method, err)
		}
	}

	explicitlyRejected := envelopeStatus.OK != nil && !*envelopeStatus.OK
	if strings.EqualFold(strings.TrimSpace(envelope.Error), "ratelimited") {
		return explicitlyRejected, &bridgesdk.RateLimitError{
			Err:        fmt.Errorf("slack api %s rate limited", strings.TrimSpace(method)),
			RetryAfter: retryAfter,
		}
	}
	if !envelope.OK {
		return explicitlyRejected, classifySlackAPIError(statusCode, envelope.Error, retryAfter)
	}
	if result != nil && len(bytes.TrimSpace(responseBody)) > 0 {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return explicitlyRejected, fmt.Errorf("slack: decode %s result: %w", method, err)
		}
	}
	if responseHeaders != nil {
		*responseHeaders = headers.Clone()
	}
	return explicitlyRejected, nil
}

func readSlackAPIResponseBody(method string, body io.Reader) ([]byte, error) {
	responseBody, readErr := io.ReadAll(io.LimitReader(body, slackAPIResponseLimit+1))
	var responseBodyErr error
	if readErr != nil {
		responseBodyErr = fmt.Errorf("slack: read %s response: %w", method, readErr)
	}
	if len(responseBody) > slackAPIResponseLimit {
		responseBodyErr = errors.Join(
			responseBodyErr,
			fmt.Errorf("slack: %s response exceeds %d bytes", method, slackAPIResponseLimit),
		)
	}
	return responseBody, responseBodyErr
}

func classifySlackNonSuccessResponse(
	method string,
	statusCode int,
	retryAfter time.Duration,
	responseBody []byte,
	responseBodyErr error,
) error {
	classifiedErr := classifySlackAPIError(statusCode, "", retryAfter)
	var envelope slackAPIEnvelope
	var decodeErr error
	if len(bytes.TrimSpace(responseBody)) > 0 {
		if err := json.Unmarshal(responseBody, &envelope); err != nil {
			decodeErr = fmt.Errorf("slack: decode %s error response: %w", method, err)
		} else if strings.TrimSpace(envelope.Error) != "" {
			classifiedErr = classifySlackAPIError(statusCode, envelope.Error, retryAfter)
		}
	}
	return errors.Join(classifiedErr, responseBodyErr, decodeErr)
}
