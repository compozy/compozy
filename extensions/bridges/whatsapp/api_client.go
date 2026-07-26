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

type whatsappGraphClient struct {
	baseURL               string
	apiVersion            string
	accessToken           string
	httpClient            *http.Client
	reportRetry           func(context.Context, bridgesdk.RetryAttempt) error
	reportRetryFailure    func(error)
	reportResponseCleanup func(error)
}

func (c *whatsappGraphClient) ReportRetry(ctx context.Context, attempt bridgesdk.RetryAttempt) {
	if c == nil || c.reportRetry == nil {
		if c != nil && c.reportRetryFailure != nil {
			c.reportRetryFailure(errors.New("whatsapp: retry reporter is required"))
		}
		return
	}
	if err := c.reportRetry(ctx, attempt); err != nil {
		if c.reportRetryFailure != nil {
			c.reportRetryFailure(fmt.Errorf("whatsapp: report retry attempt: %w", err))
		}
	}
}

func (c *whatsappGraphClient) GetPhoneNumber(
	ctx context.Context,
	phoneNumberID string,
) (*whatsappPhoneNumber, error) {
	var result whatsappPhoneNumber
	if err := c.call(
		ctx,
		http.MethodGet,
		"/"+strings.TrimSpace(phoneNumberID),
		nil,
		&result,
		bridgesdk.HTTPResponseNoCommit,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *whatsappGraphClient) SendTextMessage(
	ctx context.Context,
	phoneNumberID string,
	payload whatsappSendMessageRequest,
) (*whatsappSendMessageResponse, error) {
	var result whatsappSendMessageResponse
	if err := c.call(
		ctx,
		http.MethodPost,
		"/"+strings.TrimSpace(phoneNumberID)+"/messages",
		payload,
		&result,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	); err != nil {
		return nil, err
	}
	if _, err := lastWhatsAppResponseMessageID(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *whatsappGraphClient) call(
	ctx context.Context,
	method string,
	path string,
	payload any,
	out any,
	commitPolicy bridgesdk.HTTPResponseCommitPolicy,
) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c == nil {
		return errors.New("whatsapp: graph api client is required")
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	var body io.Reader
	if payload != nil {
		raw, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return fmt.Errorf("whatsapp: marshal %s payload: %w", strings.TrimSpace(path), marshalErr)
		}
		body = bytes.NewReader(raw)
	}

	endpoint := strings.TrimRight(
		strings.TrimSpace(c.baseURL),
		"/",
	) + "/" + strings.Trim(
		strings.TrimSpace(c.apiVersion),
		"/",
	) + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("whatsapp: build %s request: %w", strings.TrimSpace(path), err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.accessToken))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := bridgesdk.CredentialedHTTPClient(c.httpClient).Do(req)
	if err != nil {
		return err
	}
	commitEvidence := bridgesdk.HTTPResponseCommitUnconfirmed
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		commitEvidence = commitPolicy.Evidence()
	}
	defer func() {
		err = bridgesdk.FinalizeHTTPResponseBody(
			resp.Body,
			err,
			commitEvidence,
			c.reportResponseCleanup,
		)
	}()

	rawBody, readErr := bridgesdk.ReadHTTPResponseBody(resp.Body)
	raw := []byte(rawBody)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return classifyWhatsAppNonSuccessResponse(path, resp.StatusCode, resp.Header, raw, readErr)
	}
	if readErr != nil {
		return fmt.Errorf("whatsapp: read %s response: %w", strings.TrimSpace(path), readErr)
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("whatsapp: decode %s response: %w", strings.TrimSpace(path), err)
	}
	return nil
}

func classifyWhatsAppNonSuccessResponse(
	path string,
	statusCode int,
	headers http.Header,
	raw []byte,
	readErr error,
) error {
	var wrappedReadErr error
	if readErr != nil {
		wrappedReadErr = fmt.Errorf("whatsapp: read %s response: %w", strings.TrimSpace(path), readErr)
	}
	return errors.Join(
		classifyWhatsAppHTTPError(statusCode, headers.Get("Retry-After"), raw),
		wrappedReadErr,
	)
}

func lastWhatsAppResponseMessageID(response *whatsappSendMessageResponse) (string, error) {
	if response == nil || len(response.Messages) == 0 {
		return "", bridgesdk.MarkCommittedMutation(
			errors.New("whatsapp: send message response omitted a message id"),
		)
	}
	messageID := strings.TrimSpace(response.Messages[len(response.Messages)-1].ID)
	if messageID == "" {
		return "", bridgesdk.MarkCommittedMutation(
			errors.New("whatsapp: send message response omitted a message id"),
		)
	}
	return messageID, nil
}
