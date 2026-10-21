package providerusage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const MaxResponseBytes = 1 << 20

// HTTPClient confines authenticated requests to their original origin.
type HTTPClient struct {
	client  *http.Client
	baseURL string
	now     func() time.Time
}

func NewHTTPClient(baseURL string, transport http.RoundTripper, now func() time.Time) *HTTPClient {
	if now == nil {
		now = time.Now
	}
	return &HTTPClient{baseURL: strings.TrimRight(baseURL, "/"), now: now, client: &http.Client{
		Timeout: 20 * time.Second, Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 || req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}}
}

func (c *HTTPClient) Do(ctx context.Context, method, path string, headers http.Header, body io.Reader, provider string) ([]byte, error) {
	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, NewUsageError(ErrorCodeNetworkError, provider)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, NewUsageError(ErrorCodeNetworkError, provider)
	}
	req.Header = headers.Clone()
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, NewUsageError(ErrorCodeNetworkError, provider)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	// A bounded drain preserves reuse without allowing an untrusted stream to defeat the body cap.
	if _, drainErr := io.Copy(io.Discard, io.LimitReader(resp.Body, MaxResponseBytes)); drainErr != nil && err == nil {
		err = drainErr
	}
	if len(data) > MaxResponseBytes {
		return nil, NewUsageError(ErrorCodeParseError, provider)
	}
	if err != nil {
		return nil, NewUsageError(ErrorCodeNetworkError, provider)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, NewUsageError(ErrorCodeAuthMissing, provider)
	case resp.StatusCode == http.StatusForbidden:
		return nil, NewUsageError(ErrorCodeAuthScope, provider)
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, RateLimited(provider, ParseRetryAfter(resp.Header.Get("Retry-After"), c.now()))
	case resp.StatusCode >= 500:
		e := NewUsageError(ErrorCodeProviderUnavailable, provider)
		e.Message = fmt.Sprintf("%s usage service returned %d", provider, resp.StatusCode)
		return nil, e
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, NewUsageError(ErrorCodeProviderUnavailable, provider)
	}
	return data, nil
}

func ParseRetryAfter(value string, now time.Time) time.Time {
	delay := 5 * time.Minute
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		if seconds > 3600 {
			seconds = 3600
		}
		delay = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(value); err == nil && !date.Before(now) {
		delay = date.Sub(now)
	}
	return now.Add(min(delay, time.Hour))
}
