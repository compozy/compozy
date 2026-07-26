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

func (c *discordBotClient) GetBotUser(ctx context.Context) (*discordBotIdentity, error) {
	var result discordBotIdentity
	if err := c.callJSON(
		ctx,
		http.MethodGet,
		"/users/@me",
		nil,
		&result,
		bridgesdk.HTTPResponseNoCommit,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *discordBotClient) PostMessage(
	ctx context.Context,
	req discordPostMessageRequest,
) (*discordPostedMessage, error) {
	var result discordPostedMessage
	if err := c.callJSON(
		ctx,
		http.MethodPost,
		"/channels/"+strings.TrimSpace(req.ChannelID)+"/messages",
		req,
		&result,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *discordBotClient) UpdateMessage(ctx context.Context, req discordUpdateMessageRequest) error {
	return c.callJSON(
		ctx,
		http.MethodPatch,
		"/channels/"+strings.TrimSpace(req.ChannelID)+"/messages/"+strings.TrimSpace(req.MessageID),
		req,
		nil,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	)
}

func (c *discordBotClient) DeleteMessage(ctx context.Context, req discordDeleteMessageRequest) error {
	return c.callJSON(
		ctx,
		http.MethodDelete,
		"/channels/"+strings.TrimSpace(req.ChannelID)+"/messages/"+strings.TrimSpace(req.MessageID),
		nil,
		nil,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	)
}

func newDiscordBotClient(
	cfg *resolvedInstanceConfig,
	markers *bridgesdk.AdapterMarkers,
) *discordBotClient {
	return &discordBotClient{
		baseURL:    cfg.apiBaseURL,
		botToken:   cfg.botToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		reportResponseCleanup: func(err error) {
			markers.ReportError("clean up Discord API response", err)
		},
	}
}

func (c *discordBotClient) callJSON(
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
		return errors.New("discord: api client is required")
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	var body io.Reader
	if payload != nil {
		buf := &bytes.Buffer{}
		encoder := json.NewEncoder(buf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(payload); err != nil {
			return err
		}
		body = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+strings.TrimSpace(c.botToken))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := bridgesdk.CredentialedHTTPClient(c.httpClient).Do(req)
	if err != nil {
		return err
	}
	commitEvidence := bridgesdk.HTTPResponseCommitUnconfirmed
	defer func() {
		err = bridgesdk.FinalizeHTTPResponseBody(
			resp.Body,
			err,
			commitEvidence,
			c.reportResponseCleanup,
		)
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := bridgesdk.ReadHTTPResponseBody(resp.Body)
		message := strings.TrimSpace(body)
		httpErr := &bridgesdk.HTTPError{
			StatusCode: resp.StatusCode,
			Message:    firstNonEmpty(message, fmt.Sprintf("discord: http %d", resp.StatusCode)),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
		return errors.Join(httpErr, readErr)
	}
	commitEvidence = commitPolicy.Evidence()
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := bridgesdk.DecodeSingleJSONValue(resp.Body, out); err != nil {
		return fmt.Errorf("discord: decode response: %w", err)
	}
	return nil
}
