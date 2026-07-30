package auth

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/mcp/securehttp"
)

// WithRegistrationStore enables durable DCR client registration storage.
func WithRegistrationStore(store RegistrationStore) ServiceOption {
	return func(service *Service) { service.registrations = store }
}

// WithSecretRefResolver supplies Vault secret resolution for durable DCR registrations.
func WithSecretRefResolver(resolve SecretRefResolver) ServiceOption {
	return func(service *Service) { service.secretResolver = resolve }
}

// WithClientMetadataURL overrides the public CIMD URL for self-hosted deployments.
func WithClientMetadataURL(raw string) ServiceOption {
	return func(service *Service) { service.clientMetadataURL = strings.TrimSpace(raw) }
}

// WithDefaultRedirectURL supplies the global redirect URI used during refresh.
func WithDefaultRedirectURL(raw string) ServiceOption {
	return func(service *Service) { service.defaultRedirectURL = strings.TrimSpace(raw) }
}

// WithHTTPClient overrides the HTTP client used for metadata and token calls.
func WithHTTPClient(client *http.Client) ServiceOption {
	return func(service *Service) {
		service.client = client
		service.secureClient = nil
	}
}

// WithSecureHTTPClient uses Compozy's policy-enforcing remote client for OAuth calls.
func WithSecureHTTPClient(client *securehttp.Client) ServiceOption {
	return func(service *Service) {
		if client != nil {
			service.secureClient = client
			service.client = client.HTTPClient()
		}
	}
}

// WithRandom overrides the entropy source for tests.
func WithRandom(random io.Reader) ServiceOption {
	return func(service *Service) { service.random = random }
}

// WithNow overrides the clock for tests.
func WithNow(now func() time.Time) ServiceOption {
	return func(service *Service) { service.now = now }
}
