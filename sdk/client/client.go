package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/pkg/logger"
)

const defaultUserAgent = "compozy-sdk-go/v2"

// Client provides typed access to Compozy execution endpoints.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	apiKey     string
	userAgent  string
}

// Option configures client construction.
type Option func(*clientConfig)

type clientConfig struct {
	apiKey     string
	httpClient *http.Client
	userAgent  string
}

// WithAPIKey configures the Authorization bearer token used for API calls.
func WithAPIKey(key string) Option {
	return func(cfg *clientConfig) {
		cfg.apiKey = strings.TrimSpace(key)
	}
}

// WithHTTPClient injects a custom HTTP client. Callers are responsible for TLS and timeout settings.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(cfg *clientConfig) {
		if httpClient != nil {
			cfg.httpClient = httpClient
		}
	}
}

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(agent string) Option {
	return func(cfg *clientConfig) {
		cfg.userAgent = strings.TrimSpace(agent)
	}
}

// New constructs a Client targeting the provided baseURL (e.g., https://api.compozy.dev).
func New(ctx context.Context, baseURL string, opts ...Option) (*Client, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("base url is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}
	if parsed.Scheme == "" {
		return nil, fmt.Errorf("base url must include scheme")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("base url must include host")
	}
	cfg := clientConfig{
		httpClient: http.DefaultClient,
		userAgent:  defaultUserAgent,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	log := logger.FromContext(ctx)
	if log != nil {
		log.Debug("initializing sdk client", "base_url", parsed.String())
	}
	return &Client{
		baseURL:    parsed,
		httpClient: cfg.httpClient,
		apiKey:     cfg.apiKey,
		userAgent:  cfg.userAgent,
	}, nil
}
