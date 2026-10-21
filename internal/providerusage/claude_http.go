package providerusage

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
)

type claudeTransport struct{ transport http.RoundTripper }

var _ http.RoundTripper = claudeTransport{}

func newClaudeHTTP(base string, transport http.RoundTripper, now func() time.Time) *HTTPClient {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return NewHTTPClient(base, claudeTransport{transport}, now)
}

func (t claudeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.transport.RoundTrip(request)
	if err != nil || request.URL.Query().Get("cedar_ember") == "" ||
		(response.StatusCode != 400 && response.StatusCode != 403) {
		return response, err
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	closeErr := response.Body.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	response.Body = io.NopCloser(bytes.NewReader(data))
	if len(data) > MaxResponseBytes || (response.StatusCode == 403 && strings.Contains(string(data), "user:profile")) {
		return response, nil
	}
	retry := request.Clone(request.Context())
	query := retry.URL.Query()
	query.Del("cedar_ember")
	retry.URL.RawQuery = query.Encode()
	retry.Header.Set("User-Agent", "claude-code/2.1.0")
	return t.transport.RoundTrip(retry)
}
