package connectivitytailscale

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	compozysdk "github.com/compozy/compozy/sdk/go"
)

const (
	tierPrivate             = "private"
	tierPublic              = "public"
	providerHealthHealthy   = "healthy"
	providerHealthDegraded  = "degraded"
	providerHealthDown      = "down"
	providerEndpointStable  = "stable"
	providerShutdownTimeout = 5 * time.Second
)

type activeTier struct {
	endpoint  compozysdk.ConnectivityAdvertisedEndpoint
	domain    string
	forwarder *tierForwarder
}

type activeTierEntry struct {
	name   string
	active *activeTier
}

// Provider owns one embedded Tailscale node and at most one listener per gateway tier.
type Provider struct {
	operations *operationGate
	mu         sync.RWMutex
	stateDir   string
	logger     *slog.Logger
	newNode    tailscaleNodeFactory
	node       tailscaleNode
	nodeErr    error
	tiers      map[string]*activeTier
}

// NewProvider constructs the bundled provider state machine.
func NewProvider(stateDir string, logger *slog.Logger) (*Provider, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, errors.New("connectivity-tailscale: state directory is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		operations: newOperationGate(),
		stateDir:   stateDir,
		logger:     logger,
		newNode:    newTSNetNode,
		tiers:      make(map[string]*activeTier),
	}, nil
}

// Establish opens a TLS tailnet or Funnel listener and forwards it to the daemon tier listener.
//
//nolint:gocritic // The public SDK handler contract passes ExtensionContext by value.
func (p *Provider) Establish(
	ctx context.Context,
	_ compozysdk.ExtensionContext,
	req compozysdk.ConnectivityEstablishRequest,
) (result compozysdk.ConnectivityReachability, resultErr error) {
	if p == nil {
		return result, errors.New("connectivity-tailscale: provider is required")
	}
	tier, target, deadline, err := validateEstablishRequest(req)
	if err != nil {
		return result, err
	}
	callCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	if err := p.operations.acquire(callCtx); err != nil {
		return result, err
	}
	defer p.operations.release()
	if p.active(tier) != nil {
		return result, fmt.Errorf("connectivity-tailscale: tier %q is already established", tier)
	}
	node, created, err := p.ensureNode()
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr == nil || !created || p.hasActiveTiers() {
			return
		}
		resultErr = errors.Join(resultErr, p.closeNode(callCtx))
	}()
	if err := node.Up(callCtx); err != nil {
		return result, err
	}
	domain, err := selectCertificateDomain(node.CertificateDomains())
	if err != nil {
		return result, err
	}
	listener, endpoint, err := listenForTier(callCtx, node, tier, domain)
	if err != nil {
		return result, fmt.Errorf("connectivity-tailscale: listen for %s gateway: %w", tier, err)
	}
	if err := callCtx.Err(); err != nil {
		return result, errors.Join(err, closeListener(listener))
	}
	forwarder, err := newTierForwarder(listener, target.String(), p.logger)
	if err != nil {
		return result, errors.Join(err, closeListener(listener))
	}
	forwarder.Start()
	active := &activeTier{endpoint: endpoint, domain: domain, forwarder: forwarder}
	p.mu.Lock()
	p.tiers[tier] = active
	p.mu.Unlock()
	return compozysdk.ConnectivityReachability{
		Tier:      tier,
		Endpoints: []compozysdk.ConnectivityAdvertisedEndpoint{endpoint},
		Health:    providerHealthHealthy,
	}, nil
}

// Status reports the current listener health without changing reachability.
//
//nolint:gocritic // The public SDK handler contract passes ExtensionContext by value.
func (p *Provider) Status(
	ctx context.Context,
	_ compozysdk.ExtensionContext,
	req compozysdk.ConnectivityStatusRequest,
) (compozysdk.ConnectivityReachability, error) {
	if p == nil {
		return compozysdk.ConnectivityReachability{}, errors.New("connectivity-tailscale: provider is required")
	}
	tier, err := validateTier(req.Tier)
	if err != nil {
		return compozysdk.ConnectivityReachability{}, err
	}
	active := p.active(tier)
	if active == nil {
		return compozysdk.ConnectivityReachability{
			Tier: tier, Health: providerHealthDown, Reason: "tier is not established",
		}, nil
	}
	health, reason := active.forwarder.Health()
	if health == providerHealthHealthy {
		node := p.currentNode()
		if node == nil || node.Health(ctx) != nil || !sameCertificateDomain(node.CertificateDomains(), active.domain) {
			health = providerHealthDegraded
			reason = "connectivity is unavailable"
		}
	}
	return compozysdk.ConnectivityReachability{
		Tier:      tier,
		Endpoints: []compozysdk.ConnectivityAdvertisedEndpoint{active.endpoint},
		Health:    health,
		Reason:    reason,
	}, nil
}

// Teardown stops forwarding within the daemon-provided deadline.
//
//nolint:gocritic // The public SDK handler contract passes ExtensionContext by value.
func (p *Provider) Teardown(
	ctx context.Context,
	_ compozysdk.ExtensionContext,
	req compozysdk.ConnectivityTeardownRequest,
) (compozysdk.ConnectivityTeardownResponse, error) {
	if p == nil {
		return compozysdk.ConnectivityTeardownResponse{}, errors.New("connectivity-tailscale: provider is required")
	}
	tier, err := validateTier(req.Tier)
	if err != nil {
		return compozysdk.ConnectivityTeardownResponse{}, err
	}
	if req.Deadline.IsZero() || !req.Deadline.After(time.Now()) {
		return compozysdk.ConnectivityTeardownResponse{}, errors.New(
			"connectivity-tailscale: future teardown deadline is required",
		)
	}
	stopCtx, cancel := context.WithDeadline(ctx, req.Deadline)
	defer cancel()
	if err := p.operations.acquire(stopCtx); err != nil {
		return compozysdk.ConnectivityTeardownResponse{}, err
	}
	defer p.operations.release()
	active := p.active(tier)
	var stopErr error
	if active != nil {
		stopErr = active.forwarder.Close(stopCtx)
		if stopErr == nil {
			p.removeActiveIf(tier, active)
		}
	}
	if stopErr == nil && !p.hasActiveTiers() {
		stopErr = p.closeNode(stopCtx)
	}
	if stopErr != nil {
		return compozysdk.ConnectivityTeardownResponse{}, stopErr
	}
	return compozysdk.ConnectivityTeardownResponse{Stopped: true}, nil
}

// Close tears down all listeners and the embedded node during subprocess shutdown.
func (p *Provider) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if err := p.operations.acquire(ctx); err != nil {
		return err
	}
	defer p.operations.release()
	p.mu.RLock()
	entries := make([]activeTierEntry, 0, len(p.tiers))
	for tier, active := range p.tiers {
		entries = append(entries, activeTierEntry{name: tier, active: active})
	}
	p.mu.RUnlock()
	var errs []error
	for _, entry := range entries {
		if err := entry.active.forwarder.Close(ctx); err != nil {
			errs = append(errs, err)
			continue
		}
		p.removeActiveIf(entry.name, entry.active)
	}
	if !p.hasActiveTiers() {
		errs = append(errs, p.closeNode(ctx))
	}
	return errors.Join(errs...)
}

func (p *Provider) ensureNode() (tailscaleNode, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.nodeErr != nil {
		return nil, false, fmt.Errorf("connectivity-tailscale: embedded node cleanup is incomplete: %w", p.nodeErr)
	}
	if p.node != nil {
		return p.node, false, nil
	}
	node, err := p.newNode(p.stateDir, p.logger)
	if err != nil {
		return nil, false, err
	}
	p.node = node
	return node, true, nil
}

func (p *Provider) closeNode(ctx context.Context) error {
	p.mu.RLock()
	node := p.node
	p.mu.RUnlock()
	if node == nil {
		return nil
	}
	if err := node.Close(ctx); err != nil {
		wrapped := fmt.Errorf("connectivity-tailscale: close embedded node: %w", err)
		p.mu.Lock()
		if p.node == node {
			p.nodeErr = wrapped
		}
		p.mu.Unlock()
		return wrapped
	}
	p.mu.Lock()
	if p.node == node {
		p.node = nil
		p.nodeErr = nil
	}
	p.mu.Unlock()
	return nil
}

func (p *Provider) currentNode() tailscaleNode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.node
}

func (p *Provider) active(tier string) *activeTier {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tiers[tier]
}

func (p *Provider) removeActiveIf(tier string, expected *activeTier) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tiers[tier] == expected {
		delete(p.tiers, tier)
	}
}

func (p *Provider) hasActiveTiers() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.tiers) > 0
}
