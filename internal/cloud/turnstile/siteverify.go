package turnstile

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const siteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var ErrRequired = errors.New("turnstile_required")

type Client struct {
	endpoint string
	secret   string
	hostname string
	http     *http.Client
}

type Option func(*Client)

func WithEndpoint(endpoint string) Option {
	return func(c *Client) { c.endpoint = endpoint }
}

func New(secret, hostname string, options ...Option) (*Client, error) {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(hostname) == "" {
		return nil, errors.New("missing Turnstile secret or hostname")
	}
	c := &Client{
		endpoint: siteverifyURL,
		secret:   secret,
		hostname: hostname,
		http: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	for _, option := range options {
		option(c)
	}
	return c, nil
}

func (c *Client) Verify(ctx context.Context, token, clientIP, action string) error {
	if len(token) > 2_048 || strings.TrimSpace(token) == "" {
		return ErrRequired
	}
	values := url.Values{"secret": {c.secret}, "response": {token}}
	if addr, err := netip.ParseAddr(clientIP); err == nil {
		values.Set("remoteip", addr.String())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return ErrRequired
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return ErrRequired
	}
	const responseLimit = 8 << 10
	defer func() {
		// Bound the drain even when the provider returns an unexpectedly large body.
		_, drainErr := io.Copy(io.Discard, io.LimitReader(resp.Body, responseLimit))
		if err := errors.Join(drainErr, resp.Body.Close()); err != nil {
			slog.Debug("Turnstile response cleanup interrupted")
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return ErrRequired
	}
	var result struct {
		Success  bool   `json:"success"`
		Hostname string `json:"hostname"`
		Action   string `json:"action"`
	}
	if err := json.UnmarshalRead(io.LimitReader(resp.Body, responseLimit), &result); err != nil ||
		!result.Success || result.Hostname != c.hostname || (action != "" && result.Action != action) {
		return ErrRequired
	}
	return nil
}
