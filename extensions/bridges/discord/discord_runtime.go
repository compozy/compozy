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
func newDiscordProvider(stderr io.Writer) (*discordProvider, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	provider := &discordProvider{
		stderr:  stderr,
		markers: bridgesdk.NewAdapterMarkers(providerDiscordKey, stderr),
		now:     func() time.Time { return time.Now().UTC() },
		routes: bridgesdk.NewRouteTable(func(config resolvedInstanceConfig) []string {
			if config.configError != nil {
				return nil
			}
			return []string{config.webhookPath}
		}),
		deliveries: bridgesdk.NewDeliveryStateStore[deliveryState](),
	}
	provider.apiFactory = func(cfg resolvedInstanceConfig) discordAPI {
		return newDiscordBotClient(&cfg, provider.markers)
	}

	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: providerDiscordKey,
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
		ReadHeaderTimeout: discordWebhookReadHeaderTimeout,
		IdleTimeout:       discordWebhookIdleTimeout,
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
			Name:    providerDiscordKey,
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

func (p *discordProvider) serve(stdin io.Reader, stdout io.Writer) error {
	return p.lifecycle.Serve(context.Background(), p.sdk, stdin, stdout)
}

func (p *discordProvider) handleInitialize(_ context.Context, session *bridgesdk.Session) error {
	return p.lifecycle.Initialize(context.Background(), session)
}

func (p *discordProvider) handleBridgesDeliver(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	marker := bridgesdk.DeliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	}

	cfg, err := p.waitForInstanceConfig(strings.TrimSpace(request.Event.BridgeInstanceID), 500*time.Millisecond)
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

	state := p.deliveryState(cfg.instanceID, request.Event.DeliveryID)
	progress := state.Progress
	if progress != nil {
		if err := progress.Flush(ctx); err != nil {
			if !p.shouldContinueAfterProgressFailure(ctx, session, cfg.instanceID, &marker, err) {
				return bridgepkg.DeliveryAck{}, err
			}
		}
	}

	api := p.apiFactory(cfg)
	ack, state, err := executeDiscordDelivery(
		ctx,
		api,
		request,
		state,
	)
	if err != nil {
		p.recordDeliveryFailure(ctx, session, cfg.instanceID, request, state, marker, err)
		return bridgepkg.DeliveryAck{}, err
	}

	p.storeDeliveryState(cfg.instanceID, request.Event.DeliveryID, state)
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	}
	var cleanupErr error
	if progress != nil {
		cleanupErr = progress.OnContent(ctx)
		if request.Event.Final {
			detached := p.detachDiscordProgressDispatcher(
				cfg.instanceID,
				request.Event.DeliveryID,
				progress,
			)
			if detached != nil {
				detached.Close()
			}
		}
	}

	marker.Ack = &ack
	p.markers.RecordDelivery(marker)
	if cleanupErr != nil {
		p.reportDiscordProgressError("clean up progress after text delivery", cleanupErr)
	} else {
		p.clearLastError()
	}
	return ack, nil
}
