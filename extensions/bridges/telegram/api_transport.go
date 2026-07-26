package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridgesdk"
)

const telegramAPIResponseLimit = 1 << 20

func (c *telegramBotClient) callTelegram(
	ctx context.Context,
	method string,
	payload any,
	result any,
	commitPolicy bridgesdk.HTTPResponseCommitPolicy,
) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil {
		return errors.New("telegram: bot api client is required")
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal %s payload: %w", method, err)
	}
	endpoint, err := telegramAPIEndpoint(c.baseURL, c.botToken, method)
	if err != nil {
		return fmt.Errorf("telegram: build %s endpoint: %w", method, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: build %s request: %w", method, err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := bridgesdk.CredentialedHTTPClient(c.httpClient).Do(request)
	if err != nil {
		return &bridgesdk.TransientError{Err: errors.New("telegram bot api transport failed")}
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

	raw, responseBodyErr := readTelegramAPIResponseBody(method, response.Body)
	responseEnvelope, explicitlyRejected, responseErr := decodeTelegramAPIResponse(
		method,
		response.StatusCode,
		raw,
		responseBodyErr,
	)
	if explicitlyRejected {
		commitEvidence = bridgesdk.HTTPResponseCommitUnconfirmed
	}
	if responseErr != nil {
		return responseErr
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(responseEnvelope.Result, result); err != nil {
		return fmt.Errorf("telegram: decode %s result: %w", method, err)
	}
	return nil
}

func readTelegramAPIResponseBody(method string, body io.Reader) ([]byte, error) {
	raw, readErr := bridgesdk.ReadHTTPResponseBody(io.LimitReader(body, telegramAPIResponseLimit+1))
	var responseBodyErr error
	if readErr != nil {
		responseBodyErr = fmt.Errorf("telegram: read %s response: %w", method, readErr)
	}
	if len(raw) > telegramAPIResponseLimit {
		responseBodyErr = errors.Join(
			responseBodyErr,
			fmt.Errorf("telegram: %s response exceeds %d bytes", method, telegramAPIResponseLimit),
		)
	}
	return []byte(raw), responseBodyErr
}

func decodeTelegramAPIResponse(
	method string,
	statusCode int,
	raw []byte,
	responseBodyErr error,
) (telegramAPIEnvelope[json.RawMessage], bool, error) {
	responseEnvelope := telegramAPIEnvelope[json.RawMessage]{}
	decodeErr := json.Unmarshal(raw, &responseEnvelope)
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		var wrappedDecodeErr error
		if decodeErr != nil {
			wrappedDecodeErr = fmt.Errorf("telegram: decode %s error response: %w", method, decodeErr)
		}
		return responseEnvelope, false, errors.Join(
			classifyTelegramHTTPError(statusCode, responseEnvelope),
			responseBodyErr,
			wrappedDecodeErr,
		)
	}
	if responseBodyErr != nil {
		return responseEnvelope, false, responseBodyErr
	}
	if decodeErr != nil {
		return responseEnvelope, false, &bridgesdk.HTTPError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("telegram %s returned invalid JSON", method),
		}
	}

	var envelopeStatus struct {
		OK *bool `json:"ok"`
	}
	if err := json.Unmarshal(raw, &envelopeStatus); err != nil {
		return responseEnvelope, false, fmt.Errorf("telegram: decode %s response status: %w", method, err)
	}
	explicitlyRejected := envelopeStatus.OK != nil && !*envelopeStatus.OK
	if !responseEnvelope.OK {
		return responseEnvelope, explicitlyRejected, classifyTelegramHTTPError(statusCode, responseEnvelope)
	}
	return responseEnvelope, explicitlyRejected, nil
}

func telegramAPIEndpoint(baseURL string, botToken string, method string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || !parsed.IsAbs() || strings.TrimSpace(parsed.Host) == "" {
		return "", errors.New("telegram: bot api base URL is invalid")
	}
	return parsed.JoinPath(
		url.PathEscape("bot"+strings.TrimSpace(botToken)),
		url.PathEscape(strings.TrimSpace(method)),
	).String(), nil
}
