// SPDX-License-Identifier: FSL-1.1-MIT

package email

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultEndpoint = "https://api.resend.com"
	defaultSender   = "no-reply@mail.compozy.com"
)

type ResendConfig struct {
	APIKey, SegmentID string
	Endpoint, From    string
}

type Resend struct {
	config ResendConfig
	client *http.Client
}

func NewResend(config ResendConfig) (*Resend, error) {
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.SegmentID) == "" {
		return nil, errors.New("Resend API key and news segment are required")
	}
	if config.Endpoint == "" {
		config.Endpoint = defaultEndpoint
	}
	if config.From == "" {
		config.From = defaultSender
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Path != "" ||
		endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		return nil, errors.New("invalid Resend API origin")
	}
	return &Resend{config: config, client: &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

type Message struct {
	To, Subject, Text string
}

func (r *Resend) Send(ctx context.Context, key string, message Message) error {
	return r.request(ctx, http.MethodPost, "/emails", key, struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}{r.config.From, []string{message.To}, message.Subject, message.Text})
}

func (r *Resend) UpsertContact(ctx context.Context, address, name, unsubscribeURL string) error {
	return r.request(ctx, http.MethodPost, "/contacts", "", struct {
		Email        string            `json:"email"`
		FirstName    string            `json:"first_name,omitempty"`
		Unsubscribed bool              `json:"unsubscribed"`
		Segments     []segment         `json:"segments"`
		Properties   map[string]string `json:"properties"`
	}{
		Email: address, FirstName: name, Segments: []segment{{ID: r.config.SegmentID}},
		Properties: map[string]string{"compozy_unsubscribe_url": unsubscribeURL},
	})
}

type segment struct {
	ID string `json:"id"`
}

func (r *Resend) DeleteContact(ctx context.Context, address string) error {
	return r.request(ctx, http.MethodDelete, "/contacts/"+url.PathEscape(address), "", nil)
}

func (r *Resend) request(ctx context.Context, method, path, key string, payload any) (retErr error) {
	var body io.Reader
	if payload != nil {
		content, err := json.Marshal(payload)
		if err != nil {
			return errors.New("cannot encode Resend request")
		}
		body = bytes.NewReader(content)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.config.Endpoint+path, body)
	if err != nil {
		return errors.New("cannot prepare Resend request")
	}
	req.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := r.client.Do(req)
	if err != nil {
		return errors.New("cannot reach Resend")
	}
	defer func() {
		_, drainErr := io.Copy(io.Discard, res.Body)
		retErr = errors.Join(retErr, drainErr, res.Body.Close())
	}()
	if res.StatusCode >= 200 && res.StatusCode < 300 ||
		method == http.MethodDelete && res.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("Resend request failed with status %d", res.StatusCode)
}
