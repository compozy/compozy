package providerusage

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const collectorJSONType = "application/json"

type CollectorOption func(*collectorHTTP)

type collectorHTTP struct {
	baseURL   string
	transport http.RoundTripper
	now       func() time.Time
}

func WithCollectorBaseURL(baseURL string) CollectorOption {
	return func(c *collectorHTTP) { c.baseURL = baseURL }
}
func WithCollectorTransport(transport http.RoundTripper) CollectorOption {
	return func(c *collectorHTTP) { c.transport = transport }
}
func WithCollectorClock(now func() time.Time) CollectorOption {
	return func(c *collectorHTTP) {
		if now != nil {
			c.now = now
		}
	}
}
func newCollectorHTTP(baseURL string, options []CollectorOption) collectorHTTP {
	c := collectorHTTP{baseURL: baseURL, now: time.Now}
	for _, option := range options {
		option(&c)
	}
	return c
}
func (c collectorHTTP) client() *HTTPClient { return NewHTTPClient(c.baseURL, c.transport, c.now) }
func bearerHeaders(token string) http.Header {
	return http.Header{
		"Authorization": {"Bearer " + token},
		"Accept":        {collectorJSONType},
		"Content-Type":  {collectorJSONType},
	}
}
func (c collectorHTTP) get(ctx context.Context, path, token, provider string, out any) error {
	data, err := c.client().Do(ctx, http.MethodGet, path, bearerHeaders(token), nil, provider)
	if err != nil {
		return err
	}
	return decodeCollectorJSON(data, out, provider)
}
func decodeCollectorJSON(data []byte, out any, provider string) error {
	if err := json.Unmarshal(data, out); err != nil {
		return NewUsageError(ErrorCodeParseError, provider)
	}
	return nil
}
func (c collectorHTTP) snapshot(t Target, source Source) Snapshot {
	now := c.now()
	return Snapshot{
		ProviderID:   t.ProviderID,
		ProviderName: t.ProviderName,
		Status:       StatusOK,
		Source:       &source,
		Confidence:   ConfidenceExact,
		MeasuredAt:   &now,
		CheckedAt:    &now,
	}
}
func boundKey(t Target) (string, error) {
	key := t.Route.BoundSecrets["api_key"]
	if key == "" {
		return "", NewUsageError(ErrorCodeAuthMissing, t.ProviderID)
	}
	return key, nil
}
func keyIdentity(t Target) (CredentialIdentity, error) {
	key, err := boundKey(t)
	if err != nil {
		return CredentialIdentity{}, err
	}
	return NewCredentialIdentity(SourceKindAPIKeyHTTP, []byte(key), ""), nil
}
func checkedKey(t Target, cred CredentialIdentity) (string, error) {
	key, err := boundKey(t)
	if err != nil {
		return "", err
	}
	if !cred.Same(NewCredentialIdentity(SourceKindAPIKeyHTTP, []byte(key), "")) {
		return "", NewUsageError(ErrorCodeAuthMissing, t.ProviderID)
	}
	return key, nil
}

// RegisterKeyCollectors adds only the collectors whose credentials are already resolved by their route.
func RegisterKeyCollectors(registry Registry, options ...CollectorOption) {
	collectors := []Collector{
		NewOpenRouterCollector(options...), NewVercelCollector(options...), NewZaiCollector(options...),
		NewKimiCollector(options...), NewKimiCLICollector(options...), NewCursorCollector(options...),
		NewGeminiCollector(options...),
	}
	for _, collector := range collectors {
		registry[collector.ProviderID()] = collector
	}
}
