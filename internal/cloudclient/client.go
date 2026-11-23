// Package cloudclient implements the daemon's typed Compozy Cloud protocol.
package cloudclient

import (
	"bytes"
	"cmp"
	"context"
	"crypto"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/version"
)

const (
	productionURL  = "https://cloud.compozy.com"
	requestTimeout = 10 * time.Second
	maxBody        = 4 << 20
)

type Failure struct {
	Status                int
	Code, Message, Reason string
	RetryAt               time.Time
}

func (e *Failure) Error() string { return e.Code }

func (e *Failure) Temporary() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= http.StatusInternalServerError
}

func FailureCode(err error) string {
	if failure, ok := errors.AsType[*Failure](err); ok {
		return failure.Code
	}
	return "account_operation_failed"
}

func RetryDelay(now time.Time, err error) time.Duration {
	if failure, ok := errors.AsType[*Failure](err); ok {
		return max(0, failure.RetryAt.Sub(now))
	}
	return 0
}

type Credentials struct {
	Key         crypto.Signer
	AccessToken string
}

type Option func(*Client)

func WithClock(now func() time.Time) Option { return func(c *Client) { c.now = now } }

type Client struct {
	base    *url.URL
	http    *http.Client
	now     func() time.Time
	mu      sync.Mutex
	backoff *Failure
}

func New(opts ...Option) (*Client, error) {
	return newClient(version.SignInRequired == "true", opts...)
}

func newClient(signInRequired bool, opts ...Option) (*Client, error) {
	raw := productionURL
	if !signInRequired {
		raw = cmp.Or(strings.TrimSpace(os.Getenv("COMPOZY_CLOUD_URL")), raw)
	}
	base, err := url.Parse(raw)
	if err != nil || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery ||
		base.Fragment != "" || (base.Path != "" && base.Path != "/") ||
		(base.Scheme != "https" && base.Scheme != "http") {
		return nil, errors.New("COMPOZY_CLOUD_URL must be an HTTP or HTTPS origin")
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("default HTTP transport has an unexpected type")
	}
	transport = transport.Clone()
	transport.ResponseHeaderTimeout = requestTimeout
	client := &Client{
		base: base, now: time.Now,
		http: &http.Client{
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

func (c *Client) json(
	ctx context.Context, method, path string, input, output any, token string, key crypto.Signer, bindToken bool,
) error {
	request, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	response, err := c.open(request, method, path, input, token, key, bindToken, "application/json")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return unavailable()
		}
		return err
	}
	defer drain(response.Body)
	if output != nil {
		if err := json.UnmarshalRead(io.LimitReader(response.Body, maxBody), output); err != nil {
			return unavailable()
		}
	}
	return nil
}

func (c *Client) open(
	ctx context.Context, method, path string, input any, token string, key crypto.Signer, bindToken bool, accept string,
) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	backoff := c.backoff
	c.mu.Unlock()
	if backoff != nil && c.now().Before(backoff.RetryAt) {
		return nil, backoff
	}
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(payload)
	}
	target := strings.TrimRight(c.base.String(), "/") + path
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, unavailable()
	}
	req.Header.Set("Accept", accept)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != nil {
		var credential string
		if bindToken {
			credential = token
		}
		proof, err := remoteproto.BuildDPoP(key, method, target, credential, c.now())
		if err != nil {
			return nil, err
		}
		req.Header.Set("DPoP", proof)
		req.Header.Set("Authorization", "DPoP "+token)
	}
	// #nosec G704 -- the configured cloud origin is fixed before constructing typed paths.
	response, err := c.http.Do(req)
	if err != nil {
		return nil, cmp.Or(ctx.Err(), error(unavailable()))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer drain(response.Body)
		return nil, c.readFailure(response)
	}
	return response, nil
}

func (c *Client) readFailure(response *http.Response) *Failure {
	var wire struct {
		Code   string `json:"code"`
		Error  string `json:"error"`
		Reason string `json:"reason"`
	}
	status := response.StatusCode
	var failure *Failure
	switch {
	case status == http.StatusTooManyRequests:
		failure = &Failure{Status: status, Code: "rate_limited", Message: "Too many requests. Try again shortly."}
	case status >= http.StatusInternalServerError || status < 400:
		failure = unavailable()
	case json.UnmarshalRead(io.LimitReader(response.Body, maxBody), &wire) == nil && wire.Code != "":
		return &Failure{Status: status, Code: wire.Code, Message: wire.Error, Reason: wire.Reason}
	default:
		return &Failure{Status: status, Code: "cloud_request_failed", Message: "Compozy Cloud rejected the request."}
	}
	failure.RetryAt = retryAt(c.now(), response.Header.Get("Retry-After"))
	if !failure.RetryAt.IsZero() {
		c.mu.Lock()
		if c.backoff == nil || failure.RetryAt.After(c.backoff.RetryAt) {
			c.backoff = failure
		}
		c.mu.Unlock()
	}
	return failure
}

func retryAt(now time.Time, value string) time.Time {
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds >= 0 {
		return now.Add(time.Duration(seconds) * time.Second)
	}
	if then, err := http.ParseTime(value); err == nil {
		return then
	}
	return time.Time{}
}

func unavailable() *Failure {
	return &Failure{Status: http.StatusServiceUnavailable, Code: "account_cloud_unreachable",
		Message: "Can't reach Compozy Cloud. Local work is unaffected."}
}

func drain(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxBody))
	_ = body.Close()
}
