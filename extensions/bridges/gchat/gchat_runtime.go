package main

import (
	"context"

	"crypto/sha256"

	"fmt"
	"io"
	"net/http"

	"strings"

	"time"

	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

//nolint:funlen // Construction keeps the provider's declarative runtime wiring visible in one place.
func newGChatProvider(stderr io.Writer) (*gchatProvider, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	provider := &gchatProvider{
		stderr:  stderr,
		markers: bridgesdk.NewAdapterMarkers(providerGchatKey, stderr),
		now:     func() time.Time { return time.Now().UTC() },
		parents: bridgesdk.NewParentMessageCache(0),
		routes: bridgesdk.NewRouteTable(func(config resolvedInstanceConfig) []string {
			return []string{config.webhookPath}
		}),
		deliveries: bridgesdk.NewDeliveryStateStore[deliveryState](),
		apiClients: make(map[string]cachedGChatAPIClient),
	}
	provider.apiFactory = provider.apiForConfig
	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: providerGchatKey,
		Markers:      provider.markers,
		Reconcile: func(
			ctx context.Context,
			managed []subprocess.InitializeBridgeManagedInstance,
		) ([]bridgesdk.ProviderInitialState, error) {
			configs := provider.reconcileInstanceConfigs(ctx, provider.lifecycle.Session(), managed)
			states := make([]bridgesdk.ProviderInitialState, 0, len(configs))
			for idx := range configs {
				states = append(states, bridgesdk.ProviderInitialState{
					BridgeInstanceID: configs[idx].instanceID,
					Status:           configs[idx].initialStatus,
					Degradation:      configs[idx].initialDegradation,
				})
			}
			return states, nil
		},
		FinalizeInitialize: func(err error) {
			if err != nil {
				provider.setLastError(err)
			}
		},
		OnStop: provider.stopResources,
		ShutdownResources: func(ctx context.Context) error {
			if provider.http == nil {
				return nil
			}
			return provider.http.Shutdown(ctx)
		},
	})
	if err != nil {
		return nil, err
	}
	provider.lifecycle = lifecycle
	providerHTTP, err := bridgesdk.NewProviderHTTPServer(bridgesdk.ProviderHTTPConfig{
		ReadHeaderTimeout: gchatWebhookReadHeaderTimeout,
		IdleTimeout:       gchatWebhookIdleTimeout,
		Handler:           http.HandlerFunc(provider.serveWebhookHTTP),
		Go:                lifecycle.Go,
		OnError:           provider.setLastError,
	})
	if err != nil {
		return nil, err
	}
	provider.http = providerHTTP

	sdkRuntime, err := bridgesdk.NewRuntime(bridgesdk.RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    providerGchatKey,
			Version: "0.1.0",
			SDKName: "bridgesdk",
		},
		Initialize:  lifecycle.Initialize,
		Deliver:     provider.handleBridgesDeliver,
		Progress:    provider.handleBridgesProgress,
		Check:       provider.handleBridgeCheck,
		HealthCheck: func(context.Context, *bridgesdk.Session) error { return provider.healthCheck() },
		Shutdown:    lifecycle.Shutdown,
		Now:         func() time.Time { return provider.now() },
	})
	if err != nil {
		return nil, err
	}
	provider.sdk = sdkRuntime
	return provider, nil
}

func (p *gchatProvider) apiForConfig(cfg *resolvedInstanceConfig) gchatAPI {
	cacheKey := gchatAPIClientCacheKey(cfg)
	instanceID := strings.TrimSpace(cfg.instanceID)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.apiClients == nil {
		p.apiClients = make(map[string]cachedGChatAPIClient)
	}
	if cached, ok := p.apiClients[instanceID]; ok && cached.key == cacheKey {
		return cached.client
	}
	client := newGChatBotClient(cfg, p.markers)
	p.apiClients[instanceID] = cachedGChatAPIClient{key: cacheKey, client: client}
	return client
}

func gchatAPIClientCacheKey(cfg *resolvedInstanceConfig) string {
	privateKeyHash := sha256.Sum256([]byte(strings.TrimSpace(cfg.credentials.PrivateKey)))
	return strings.Join([]string{
		strings.TrimSpace(cfg.instanceID),
		normalizeURL(cfg.apiBaseURL),
		normalizeURL(cfg.tokenURL),
		strings.TrimSpace(cfg.credentials.ClientEmail),
		fmt.Sprintf("%x", privateKeyHash),
	}, "\x00")
}

func (p *gchatProvider) serve(stdin io.Reader, stdout io.Writer) error {
	return p.lifecycle.Serve(context.Background(), p.sdk, stdin, stdout)
}
