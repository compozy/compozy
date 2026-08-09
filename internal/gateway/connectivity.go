package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

var (
	// ErrEndpointUnverified reports that a provider endpoint did not prove the assigned tier.
	ErrEndpointUnverified = errors.New("gateway endpoint unverified")
	// ErrProviderDegraded reports that an active connectivity provider is unhealthy.
	ErrProviderDegraded = errors.New("gateway provider degraded")
)

// EndpointStability describes whether a provider endpoint is expected to survive restarts.
type EndpointStability string

// EndpointSchemePolicy declares a trusted transport policy for a provider-owned scheme.
type EndpointSchemePolicy string

const (
	EndpointStable      EndpointStability = "stable"
	EndpointEphemeral   EndpointStability = "ephemeral"
	endpointSchemeHTTPS                   = "https"
	// EndpointSchemePolicyTailnetInternal allows a private-tier URI scheme over verified TLS.
	EndpointSchemePolicyTailnetInternal EndpointSchemePolicy = "tailnet_internal"
)

// Validate reports unsupported endpoint stability values.
func (s EndpointStability) Validate() error {
	switch s {
	case EndpointStable, EndpointEphemeral:
		return nil
	default:
		return fmt.Errorf("gateway: invalid endpoint stability %q", s)
	}
}

// AdvertisedEndpoint is one untrusted provider endpoint pending core verification.
type AdvertisedEndpoint struct {
	URL          string
	Scheme       string
	SchemePolicy EndpointSchemePolicy
	Stability    EndpointStability
}

// Validate checks the provider-owned descriptor without performing network I/O.
func (e AdvertisedEndpoint) Validate() error {
	rawURL := strings.TrimSpace(e.URL)
	if diagnostics.Redact(rawURL) != rawURL {
		return errors.New("gateway: endpoint URL cannot contain sensitive material")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || !parsed.IsAbs() || parsed.Opaque != "" || parsed.Hostname() == "" {
		return errors.New("gateway: endpoint URL is invalid")
	}
	if parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return errors.New("gateway: endpoint URL cannot contain credentials, a query, or a fragment")
	}
	scheme := strings.ToLower(strings.TrimSpace(e.Scheme))
	if scheme == "" || !strings.EqualFold(scheme, parsed.Scheme) {
		return errors.New("gateway: endpoint scheme must match its URL")
	}
	policy := EndpointSchemePolicy(strings.ToLower(strings.TrimSpace(string(e.SchemePolicy))))
	switch {
	case scheme == endpointSchemeHTTPS && policy == "":
	case scheme != endpointSchemeHTTPS && scheme != "http" && policy == EndpointSchemePolicyTailnetInternal:
	default:
		return errors.New("gateway: endpoint scheme is not allowed by policy")
	}
	return e.Stability.Validate()
}

// EstablishRequest is sent to a connectivity source after the tier listener is bound.
type EstablishRequest struct {
	Tier          Tier
	ForwardTarget netip.AddrPort
	ChallengePath string
	Deadline      time.Time
}

// ConnectivitySource is the daemon-owned consumer contract for one extension provider.
type ConnectivitySource interface {
	Establish(context.Context, EstablishRequest) (Reachability, error)
	Status(context.Context, Tier) (Reachability, error)
	Teardown(context.Context, Tier, time.Time) error
}

// ConnectivitySourceResolver resolves exactly one globally installed provider by name.
type ConnectivitySourceResolver interface {
	ResolveConnectivitySource(context.Context, string) (ConnectivitySource, error)
}
