package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/golang-jwt/jwt/v5"
)

func (c *gchatBotClient) ValidateAuth(ctx context.Context) error {
	_, err := c.accessToken(ctx)
	return err
}

func (c *gchatBotClient) CreateMessage(ctx context.Context, req gchatCreateMessageRequest) (*gchatSentMessage, error) {
	query := url.Values{}
	body := map[string]any{
		providerTextKey: req.Text,
	}
	if strings.TrimSpace(req.ThreadName) != "" {
		body["thread"] = map[string]string{providerNameKey: strings.TrimSpace(req.ThreadName)}
		query.Set("messageReplyOption", gchatReplyMode)
	}
	var out gchatSentMessage
	if err := c.callJSON(
		ctx,
		http.MethodPost,
		"/v1/"+strings.TrimPrefix(strings.TrimSpace(req.SpaceName), "/")+"/messages",
		query,
		body,
		&out,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *gchatBotClient) UpdateMessage(ctx context.Context, req gchatUpdateMessageRequest) (*gchatSentMessage, error) {
	query := url.Values{}
	query.Set("updateMask", providerTextKey)
	var out gchatSentMessage
	if err := c.callJSON(
		ctx,
		http.MethodPatch,
		"/v1/"+strings.TrimPrefix(strings.TrimSpace(req.MessageName), "/"),
		query,
		map[string]any{
			providerTextKey: req.Text,
		},
		&out,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *gchatBotClient) DeleteMessage(ctx context.Context, messageName string) error {
	return c.callJSON(
		ctx,
		http.MethodDelete,
		"/v1/"+strings.TrimPrefix(strings.TrimSpace(messageName), "/"),
		nil,
		nil,
		nil,
		bridgesdk.HTTPResponseCommitOnSuccessStatus,
	)
}

func (c *gchatBotClient) GetMessage(ctx context.Context, messageName string) (*gchatMessage, error) {
	var out gchatMessage
	if err := c.callJSON(
		ctx,
		http.MethodGet,
		"/v1/"+strings.TrimPrefix(strings.TrimSpace(messageName), "/"),
		nil,
		nil,
		&out,
		bridgesdk.HTTPResponseNoCommit,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func newGChatBotClient(
	cfg *resolvedInstanceConfig,
	markers *bridgesdk.AdapterMarkers,
) *gchatBotClient {
	return &gchatBotClient{
		cfg:        *cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		reportResponseCleanup: func(err error) {
			markers.ReportError("clean up Google Chat API response", err)
		},
	}
}

func (c *gchatBotClient) accessToken(ctx context.Context) (result string, err error) {
	c.mu.Lock()
	if c.cachedToken != "" && c.tokenExpiry.After(time.Now().UTC().Add(30*time.Second)) {
		token := c.cachedToken
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	privateKey, err := parseRSAPrivateKey(c.cfg.credentials.PrivateKey)
	if err != nil {
		return "", &bridgesdk.AuthError{Err: err}
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		providerIssKey: strings.TrimSpace(c.cfg.credentials.ClientEmail),
		"sub":          strings.TrimSpace(c.cfg.credentials.ClientEmail),
		"scope":        gchatBotScope,
		providerAudKey: strings.TrimSpace(c.cfg.tokenURL),
		providerIatKey: now.Unix(),
		providerExpKey: now.Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", &bridgesdk.AuthError{Err: err}
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", signed)

	endpoint, err := newValidatedGChatURL(c.cfg.tokenURL)
	if err != nil {
		return "", err
	}
	req, err := newGChatRequest(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client().Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, bridgesdk.DrainAndCloseHTTPResponseBody(resp.Body))
	}()
	if resp.StatusCode != http.StatusOK {
		body, readErr := bridgesdk.ReadHTTPResponseBody(resp.Body)
		return "", errors.Join(
			classifyGChatHTTPError(resp.StatusCode, resp.Header.Get("Retry-After"), body),
			readErr,
		)
	}
	var tokenResp gchatTokenResponse
	if err := bridgesdk.DecodeSingleJSONValue(resp.Body, &tokenResp); err != nil {
		return "", err
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", &bridgesdk.AuthError{Err: errors.New("gchat: token response omitted access token")}
	}
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	c.mu.Lock()
	c.cachedToken = strings.TrimSpace(tokenResp.AccessToken)
	c.tokenExpiry = time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
	value := c.cachedToken
	c.mu.Unlock()
	return value, nil
}

func (c *gchatBotClient) callJSON(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	payload any,
	out any,
	commitPolicy bridgesdk.HTTPResponseCommitPolicy,
) (err error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	fullURL, err := joinGChatURL(c.cfg.apiBaseURL, path)
	if err != nil {
		return err
	}
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	endpoint, err := newValidatedGChatURL(fullURL)
	if err != nil {
		return err
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := newGChatRequest(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client().Do(req)
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
		return errors.Join(
			classifyGChatHTTPError(resp.StatusCode, resp.Header.Get("Retry-After"), body),
			readErr,
		)
	}
	commitEvidence = commitPolicy.Evidence()
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := bridgesdk.DecodeSingleJSONValue(resp.Body, out); err != nil {
		return err
	}
	return nil
}

func (c *gchatBotClient) client() gchatHTTPDoer {
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	client, ok := c.httpClient.(*http.Client)
	if !ok {
		return c.httpClient
	}
	return bridgesdk.CredentialedHTTPClient(client)
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil {
		return nil, errors.New("gchat: decode private key: missing pem block")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("gchat: parse private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("gchat: private key was not rsa")
	}
	return key, nil
}

func classifyGChatHTTPError(statusCode int, retryAfterHeader string, raw string) error {
	message := strings.TrimSpace(raw)
	envelope := gchatGoogleErrorEnvelope{}
	if json.Unmarshal([]byte(raw), &envelope) == nil {
		if trimmed := strings.TrimSpace(envelope.Error.Message); trimmed != "" {
			message = trimmed
		}
	}
	if message == "" {
		message = fmt.Sprintf("gchat: http %d", statusCode)
	}
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &bridgesdk.AuthError{Err: errors.New(message)}
	case http.StatusTooManyRequests:
		return &bridgesdk.RateLimitError{Err: errors.New(message), RetryAfter: parseRetryAfter(retryAfterHeader)}
	default:
		return &bridgesdk.HTTPError{
			StatusCode: statusCode,
			Message:    message,
			RetryAfter: parseRetryAfter(retryAfterHeader),
		}
	}
}

func parseRetryAfter(header string) time.Duration {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return 0
	}
	seconds, err := strconv.Atoi(trimmed)
	if err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func stringHeader(header map[string]any, key string) string {
	value, ok := header[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func issuerMatches(issuer string, allowed ...string) bool {
	trimmed := strings.TrimSpace(issuer)
	if trimmed == "" {
		return false
	}
	for _, candidate := range allowed {
		if strings.EqualFold(trimmed, strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}
