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
	APIKey, ContactAPIKey, SegmentID string
	Endpoint, From, PostalAddress    string
}

type Resend struct {
	config ResendConfig
	client *http.Client
}

func NewResend(config ResendConfig) (*Resend, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("Resend sending API key is required")
	}
	config.SegmentID = strings.TrimSpace(config.SegmentID)
	config.ContactAPIKey = strings.TrimSpace(config.ContactAPIKey)
	if (config.SegmentID == "") != (config.ContactAPIKey == "") {
		return nil, errors.New("Resend contacts API key and news segment are required together")
	}
	config.PostalAddress = strings.TrimSpace(config.PostalAddress)
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

type outboundMessage struct {
	From    string            `json:"from"`
	To      []string          `json:"to"`
	Subject string            `json:"subject"`
	Text    string            `json:"text"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (r *Resend) Send(ctx context.Context, key string, message Message) error {
	return r.send(ctx, key, message, nil)
}

func (r *Resend) send(ctx context.Context, key string, message Message, headers map[string]string) error {
	return r.request(ctx, r.config.APIKey, http.MethodPost, "/emails", key, outboundMessage{
		From: r.config.From, To: []string{message.To}, Subject: message.Subject, Text: message.Text, Headers: headers,
	})
}

type contactDetails struct {
	Email        string            `json:"email,omitempty"`
	Segments     []segment         `json:"segments,omitempty"`
	FirstName    string            `json:"first_name"`
	Unsubscribed bool              `json:"unsubscribed"`
	Properties   map[string]string `json:"properties"`
}

func (r *Resend) SyncsContacts() bool {
	return r != nil && r.config.ContactAPIKey != ""
}

func (r *Resend) UpsertContact(ctx context.Context, address, name, unsubscribeURL string) error {
	path := "/contacts/" + url.PathEscape(address)
	properties := map[string]string{"compozy_unsubscribe_url": unsubscribeURL}
	if r.config.PostalAddress != "" {
		properties["compozy_postal_address"] = r.config.PostalAddress
	}
	details := contactDetails{FirstName: name, Properties: properties}
	err := r.request(ctx, r.config.ContactAPIKey, http.MethodPatch, path, "", details)
	switch {
	case statusIs(err, http.StatusNotFound):
		details.Email = address
		details.Segments = []segment{{ID: r.config.SegmentID}}
		return r.request(ctx, r.config.ContactAPIKey, http.MethodPost, "/contacts", "", details)
	case err != nil:
		return err
	default:
		return r.request(ctx, r.config.ContactAPIKey, http.MethodPost,
			path+"/segments/"+url.PathEscape(r.config.SegmentID), "", nil)
	}
}

type segment struct {
	ID string `json:"id"`
}

func (r *Resend) DeleteContact(ctx context.Context, address string) error {
	err := r.request(ctx, r.config.ContactAPIKey, http.MethodDelete, "/contacts/"+url.PathEscape(address), "", nil)
	if statusIs(err, http.StatusNotFound) {
		return nil
	}
	return err
}

type resendStatus int

func (s resendStatus) Error() string { return fmt.Sprintf("Resend request failed with status %d", s) }

func statusIs(err error, status int) bool { return errors.Is(err, resendStatus(status)) }

func (r *Resend) request(ctx context.Context, apiKey, method, path, key string, payload any) (retErr error) {
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
	req.Header.Set("Authorization", "Bearer "+apiKey)
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
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	return resendStatus(res.StatusCode)
}
