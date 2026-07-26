package main

import (
	"context"

	"io"
	"net/http"

	"os"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

//nolint:funlen // Construction keeps the provider's declarative runtime wiring visible in one place.
func newTelegramProvider(stderr io.Writer) (*telegramProvider, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	provider := &telegramProvider{
		stderr:  stderr,
		markers: bridgesdk.NewAdapterMarkers(providerTelegramKey, stderr),
		now:     func() time.Time { return time.Now().UTC() },
		parents: bridgesdk.NewParentMessageCache(0),
		routes: bridgesdk.NewRouteTable(func(config resolvedInstanceConfig) []string {
			if config.configError != nil {
				return nil
			}
			return []string{config.webhookPath}
		}),
		deliveries: bridgesdk.NewDeliveryStateStore[deliveryState](),
	}
	provider.apiFactory = func(cfg *resolvedInstanceConfig) telegramAPI {
		return &telegramBotClient{
			baseURL:  cfg.apiBaseURL,
			botToken: cfg.botToken,
			httpClient: &http.Client{
				Timeout: 10 * time.Second,
			},
			reportResponseCleanup: func(err error) {
				provider.markers.ReportError("clean up Telegram API response", err)
			},
		}
	}

	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: providerTelegramKey,
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
		ReadHeaderTimeout: telegramWebhookReadHeaderTimeout,
		IdleTimeout:       telegramWebhookIdleTimeout,
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
			Name:    providerTelegramKey,
			Version: telegramProviderVersion,
			SDKName: "bridgesdk",
		},
		Initialize:      lifecycle.Initialize,
		Deliver:         provider.handleBridgesDeliver,
		Progress:        provider.handleBridgesProgress,
		Check:           provider.handleBridgeCheck,
		RegisterWebhook: provider.handleBridgeWebhookRegistration,
		HealthCheck:     func(context.Context, *bridgesdk.Session) error { return provider.healthCheck() },
		Shutdown:        lifecycle.Shutdown,
		Now:             func() time.Time { return provider.now() },
	})
	if err != nil {
		return nil, err
	}
	provider.sdk = sdkRuntime
	return provider, nil
}

func (p *telegramProvider) serve(stdin io.Reader, stdout io.Writer) error {
	return p.lifecycle.Serve(context.Background(), p.sdk, stdin, stdout)
}

func (p *telegramProvider) handleBridgesDeliver(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	marker := bridgesdk.DeliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	}

	cfg, err := p.waitForInstanceConfig(
		strings.TrimSpace(request.Event.BridgeInstanceID),
		500*time.Millisecond,
	)
	if err != nil {
		marker.Error = err.Error()
		p.markers.RecordDelivery(marker)
		p.setLastError(err)
		return bridgepkg.DeliveryAck{}, err
	}

	if p.markers.ShouldCrashOnce() {
		p.markers.RecordDelivery(marker)
		p.markers.RecordCrash(map[string]any{
			"crashed":            true,
			"pid":                os.Getpid(),
			"delivery_id":        strings.TrimSpace(request.Event.DeliveryID),
			"bridge_instance_id": cfg.instanceID,
		})
		os.Exit(23)
	}

	ack, state, err := p.executeTextDeliveryWithProgress(ctx, &cfg, request)
	if err != nil {
		if bridgesdk.IsCommittedMutation(err) {
			p.deliveries.Delete(deliveryStateKey(cfg.instanceID, request.Event.DeliveryID))
			closeProgressDispatcher(state.Progress)
		} else {
			p.storeDeliveryRetryState(cfg.instanceID, request.Event.DeliveryID, state)
		}
		marker.Error = err.Error()
		p.markers.RecordDelivery(marker)
		classified := bridgesdk.ClassifyError(err)
		_, _, reportErr := session.ReportClassifiedError(ctx, cfg.instanceID, classified)
		if reportErr != nil {
			p.setLastError(reportErr)
		} else {
			p.setLastError(err)
		}
		return bridgepkg.DeliveryAck{}, err
	}

	progressCleanupErr := p.completeTextDeliveryProgress(ctx, cfg.instanceID, request, state)
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	} else if progressCleanupErr != nil {
		p.recordProgressCleanupError("clear progress after text delivery", progressCleanupErr)
	} else {
		p.clearLastError()
	}

	marker.Ack = &ack
	p.markers.RecordDelivery(marker)
	return ack, nil
}
