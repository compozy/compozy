package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/gateway"
)

const (
	maxConnectivityProviderReasonBytes = 1024
	maxConnectivityProviderEndpoints   = 8
)

type connectivitySource struct {
	manager *Manager
	name    string
}

var (
	_ gateway.ConnectivitySource         = (*connectivitySource)(nil)
	_ gateway.ConnectivitySourceResolver = (*Manager)(nil)
)

// ResolveConnectivitySource returns the global extension provider selected by durable gateway policy.
func (m *Manager) ResolveConnectivitySource(
	ctx context.Context,
	name string,
) (gateway.ConnectivitySource, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, errors.New("extension: connectivity provider name is required")
	}
	if _, _, err := m.extensionServiceProcess(
		ctx,
		trimmed,
		extensionprotocol.ExtensionServiceMethodConnectivityStatus,
	); err != nil {
		return nil, err
	}
	return &connectivitySource{manager: m, name: trimmed}, nil
}

func (s *connectivitySource) Establish(
	ctx context.Context,
	req gateway.EstablishRequest,
) (gateway.Reachability, error) {
	if s == nil || s.manager == nil {
		return gateway.Reachability{}, errors.New("extension: connectivity source is required")
	}
	if err := req.Tier.Validate(); err != nil {
		return gateway.Reachability{}, err
	}
	if !req.ForwardTarget.IsValid() || req.ForwardTarget.Port() == 0 {
		return gateway.Reachability{}, errors.New("extension: connectivity forward target is required")
	}
	if strings.TrimSpace(req.ChallengePath) == "" {
		return gateway.Reachability{}, errors.New("extension: connectivity challenge path is required")
	}
	if req.Deadline.IsZero() {
		return gateway.Reachability{}, errors.New("extension: connectivity deadline is required")
	}
	process, name, err := s.manager.extensionServiceProcess(
		ctx,
		s.name,
		extensionprotocol.ExtensionServiceMethodConnectivityEstablish,
	)
	if err != nil {
		return gateway.Reachability{}, err
	}
	wireRequest := extensioncontract.ConnectivityEstablishRequest{
		Tier: string(req.Tier), ForwardTarget: req.ForwardTarget.String(),
		ChallengePath: strings.TrimSpace(req.ChallengePath), Deadline: req.Deadline.UTC(),
	}
	return callConnectivityReachability(
		ctx,
		process,
		name,
		extensionprotocol.ExtensionServiceMethodConnectivityEstablish,
		wireRequest,
		req.Tier,
	)
}

func (s *connectivitySource) Status(ctx context.Context, tier gateway.Tier) (gateway.Reachability, error) {
	if s == nil || s.manager == nil {
		return gateway.Reachability{}, errors.New("extension: connectivity source is required")
	}
	if err := tier.Validate(); err != nil {
		return gateway.Reachability{}, err
	}
	process, name, err := s.manager.extensionServiceProcess(
		ctx,
		s.name,
		extensionprotocol.ExtensionServiceMethodConnectivityStatus,
	)
	if err != nil {
		return gateway.Reachability{}, err
	}
	return callConnectivityReachability(
		ctx,
		process,
		name,
		extensionprotocol.ExtensionServiceMethodConnectivityStatus,
		extensioncontract.ConnectivityStatusRequest{Tier: string(tier)},
		tier,
	)
}

func (s *connectivitySource) Teardown(ctx context.Context, tier gateway.Tier, deadline time.Time) error {
	if s == nil || s.manager == nil {
		return errors.New("extension: connectivity source is required")
	}
	if err := tier.Validate(); err != nil {
		return err
	}
	if deadline.IsZero() {
		return errors.New("extension: connectivity teardown deadline is required")
	}
	process, name, err := s.manager.extensionServiceProcess(
		ctx,
		s.name,
		extensionprotocol.ExtensionServiceMethodConnectivityTeardown,
	)
	if err != nil {
		return err
	}
	var response extensioncontract.ConnectivityTeardownResponse
	if err := process.Call(
		ctx,
		string(extensionprotocol.ExtensionServiceMethodConnectivityTeardown),
		extensioncontract.ConnectivityTeardownRequest{Tier: string(tier), Deadline: deadline.UTC()},
		&response,
	); err != nil {
		abortErr := s.manager.abortConnectivityProvider(ctx, name, process)
		return errors.Join(
			fmt.Errorf("extension: teardown connectivity via %q: %w", name, err),
			abortErr,
		)
	}
	if !response.Stopped {
		refusal := fmt.Errorf("extension: connectivity provider %q did not acknowledge teardown", name)
		return errors.Join(refusal, s.manager.abortConnectivityProvider(ctx, name, process))
	}
	return nil
}

func callConnectivityReachability(
	ctx context.Context,
	process processHandle,
	name string,
	method extensionprotocol.ExtensionServiceMethod,
	request any,
	expectedTier gateway.Tier,
) (gateway.Reachability, error) {
	var response extensioncontract.ConnectivityReachability
	if err := process.Call(ctx, string(method), request, &response); err != nil {
		operation := strings.TrimPrefix(string(method), "connectivity/")
		return gateway.Reachability{}, fmt.Errorf(
			"extension: %s connectivity via %q: %w",
			operation,
			name,
			err,
		)
	}
	reachability, err := connectivityReachabilityFromWire(response)
	if err != nil {
		return gateway.Reachability{}, err
	}
	if reachability.Tier != expectedTier {
		return gateway.Reachability{}, fmt.Errorf(
			"%w: connectivity provider returned the wrong tier",
			gateway.ErrEndpointUnverified,
		)
	}
	return reachability, nil
}

func connectivityReachabilityFromWire(value extensioncontract.ConnectivityReachability) (gateway.Reachability, error) {
	tier := gateway.Tier(strings.TrimSpace(value.Tier))
	if err := tier.Validate(); err != nil {
		return gateway.Reachability{}, fmt.Errorf("%w: invalid provider tier", gateway.ErrEndpointUnverified)
	}
	health := gateway.ProviderHealth(strings.TrimSpace(value.Health))
	switch health {
	case gateway.HealthHealthy, gateway.HealthDegraded, gateway.HealthDown:
	default:
		return gateway.Reachability{}, fmt.Errorf("%w: invalid connectivity health", gateway.ErrProviderDegraded)
	}
	if len(value.Endpoints) > maxConnectivityProviderEndpoints {
		return gateway.Reachability{}, fmt.Errorf(
			"%w: provider returned too many endpoints",
			gateway.ErrEndpointUnverified,
		)
	}
	endpoints := make([]gateway.AdvertisedEndpoint, 0, len(value.Endpoints))
	seen := make(map[string]struct{}, len(value.Endpoints))
	for index, endpoint := range value.Endpoints {
		mapped := gateway.AdvertisedEndpoint{
			URL: strings.TrimSpace(endpoint.URL), Scheme: strings.TrimSpace(endpoint.Scheme),
			SchemePolicy: gateway.EndpointSchemePolicy(strings.TrimSpace(endpoint.SchemePolicy)),
			Stability:    gateway.EndpointStability(strings.TrimSpace(endpoint.Stability)),
		}
		mapped, err := normalizeConnectivityEndpoint(mapped)
		if err != nil {
			return gateway.Reachability{}, fmt.Errorf(
				"%w: invalid connectivity endpoint %d",
				gateway.ErrEndpointUnverified,
				index,
			)
		}
		if _, ok := seen[mapped.URL]; ok {
			return gateway.Reachability{}, fmt.Errorf(
				"%w: duplicate connectivity endpoint %d",
				gateway.ErrEndpointUnverified,
				index,
			)
		}
		seen[mapped.URL] = struct{}{}
		endpoints = append(endpoints, mapped)
	}
	return gateway.Reachability{
		Tier: tier, Endpoints: endpoints, Health: health,
		Reason: diagnostics.RedactAndBound(strings.TrimSpace(value.Reason), maxConnectivityProviderReasonBytes),
	}, nil
}

func normalizeConnectivityEndpoint(endpoint gateway.AdvertisedEndpoint) (gateway.AdvertisedEndpoint, error) {
	if err := endpoint.Validate(); err != nil {
		return gateway.AdvertisedEndpoint{}, err
	}
	parsed, err := url.Parse(strings.TrimSpace(endpoint.URL))
	if err != nil {
		return gateway.AdvertisedEndpoint{}, err
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	hostname := strings.ToLower(parsed.Hostname())
	if port := parsed.Port(); port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		parsed.Host = "[" + hostname + "]"
	} else {
		parsed.Host = hostname
	}
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	parsed.RawPath = ""
	endpoint.URL = parsed.String()
	endpoint.Scheme = parsed.Scheme
	endpoint.SchemePolicy = gateway.EndpointSchemePolicy(
		strings.ToLower(strings.TrimSpace(string(endpoint.SchemePolicy))),
	)
	return endpoint, nil
}
