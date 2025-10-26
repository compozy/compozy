package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const defaultTimeout = 30 * time.Second

// Builder constructs HTTP clients for talking to a Compozy server while
// aggregating validation errors until Build is invoked.
type Builder struct {
	endpoint string
	apiKey   string
	timeout  time.Duration
	errors   []error
}

// New seeds a builder with the provided endpoint. The value is trimmed but not
// validated until Build is called.
func New(endpoint string) *Builder {
	return &Builder{
		endpoint: strings.TrimSpace(endpoint),
		timeout:  defaultTimeout,
		errors:   make([]error, 0),
	}
}

// WithAPIKey configures the API key used for authenticated requests.
func (b *Builder) WithAPIKey(key string) *Builder {
	if b == nil {
		return nil
	}
	b.apiKey = strings.TrimSpace(key)
	return b
}

// WithTimeout overrides the default HTTP timeout used by the client.
func (b *Builder) WithTimeout(d time.Duration) *Builder {
	if b == nil {
		return nil
	}
	if d <= 0 {
		b.errors = append(b.errors, fmt.Errorf("timeout must be positive"))
		return b
	}
	b.timeout = d
	return b
}

// Build validates gathered configuration and returns a ready HTTP client.
func (b *Builder) Build(ctx context.Context) (*Client, error) {
	if b == nil {
		return nil, fmt.Errorf("client builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	collected := append([]error{}, b.errors...)
	if err := validate.ValidateRequired(ctx, "endpoint", b.endpoint); err != nil {
		collected = append(collected, err)
	} else if err := validate.ValidateURL(ctx, b.endpoint); err != nil {
		collected = append(collected, err)
	}
	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}
	parsed, err := url.Parse(b.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	base := buildBaseURL(parsed)
	client := &Client{
		httpClient: &http.Client{Timeout: b.timeout},
		baseURL:    base,
		apiKey:     b.apiKey,
		rawBase:    strings.TrimRight(b.endpoint, "/"),
	}
	log.Debug("constructed Compozy client", "base_url", client.baseURL, "timeout", b.timeout)
	return client, nil
}

func buildBaseURL(endpoint *url.URL) string {
	if endpoint == nil {
		return ""
	}
	copied := *endpoint
	if strings.Contains(copied.Path, routes.Base()) {
		copied.Path = strings.TrimRight(copied.Path, "/")
		return copied.String()
	}
	path := strings.TrimRight(copied.Path, "/")
	if path == "" {
		path = routes.Base()
	} else {
		path = strings.TrimRight(path, "/") + routes.Base()
	}
	copied.Path = path
	return copied.String()
}
