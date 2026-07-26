package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
)

type bridgeProviderCatalog struct {
	available bool
	providers map[string]bridgepkg.BridgeProvider
}

func (h *BaseHandlers) bridgeHealthPayloadForInstance(
	ctx context.Context,
	bridges BridgeService,
	instance bridgepkg.BridgeInstance,
	health contract.BridgeHealthPayload,
	providerCatalog *bridgeProviderCatalog,
) (contract.BridgeHealthPayload, error) {
	health = bridgeBaseHealthPayload(instance, health)
	diagnostics, err := h.bridgeDiagnosticsForInstance(ctx, bridges, instance, health, providerCatalog)
	if err != nil {
		return contract.BridgeHealthPayload{}, err
	}
	health.Diagnostics = diagnostics
	return health, nil
}

func (h *BaseHandlers) bridgeDiagnosticsForInstance(
	ctx context.Context,
	bridges BridgeService,
	instance bridgepkg.BridgeInstance,
	health contract.BridgeHealthPayload,
	providerCatalog *bridgeProviderCatalog,
) ([]bridgepkg.BridgeDiagnostic, error) {
	provider, catalogAvailable, err := bridgeProviderForInstance(ctx, bridges, instance, providerCatalog)
	if err != nil {
		return nil, err
	}
	bindings, err := bridgeSecretBindingsForDiagnostics(ctx, bridges, instance, provider)
	if err != nil {
		return nil, err
	}
	return bridgepkg.BuildBridgeDiagnostics(bridgepkg.BridgeDiagnosticsInput{
		Instance:                 instance,
		Provider:                 provider,
		ProviderCatalogAvailable: catalogAvailable,
		SecretBindings:           bindings,
		RouteCount:               health.RouteCount,
		DeliveryBacklog:          health.DeliveryBacklog,
		DeliveryFailuresTotal:    health.DeliveryFailuresTotal,
		AuthFailuresTotal:        health.AuthFailuresTotal,
		LastError:                health.LastError,
	}), nil
}

func bridgeBaseHealthPayload(
	instance bridgepkg.BridgeInstance,
	health contract.BridgeHealthPayload,
) contract.BridgeHealthPayload {
	instanceID := strings.TrimSpace(instance.ID)
	if health.BridgeInstanceID == "" {
		health.BridgeInstanceID = instanceID
	}
	if health.Status == "" {
		health.Status = instance.Status
	}
	health.Degradation = cloneBridgeDegradation(instance.Degradation)
	return health
}

func loadBridgeProviderCatalog(
	ctx context.Context,
	bridges BridgeService,
) (bridgeProviderCatalog, error) {
	providers, err := bridges.ListProviders(ctx)
	if err != nil {
		return bridgeProviderCatalog{}, fmt.Errorf("load bridge providers for diagnostics: %w", err)
	}
	catalog := bridgeProviderCatalog{
		available: len(providers) > 0,
		providers: make(map[string]bridgepkg.BridgeProvider, len(providers)),
	}
	for _, provider := range providers {
		catalog.providers[bridgeProviderCatalogKey(provider.Platform, provider.ExtensionName)] = provider
	}
	return catalog, nil
}

func bridgeProviderCatalogKey(platform string, extensionName string) string {
	return strings.ToLower(strings.TrimSpace(platform)) + "\x00" + strings.ToLower(strings.TrimSpace(extensionName))
}

func (c bridgeProviderCatalog) providerForInstance(
	instance bridgepkg.BridgeInstance,
) (*bridgepkg.BridgeProvider, bool) {
	provider, ok := c.providers[bridgeProviderCatalogKey(instance.Platform, instance.ExtensionName)]
	if !ok {
		return nil, c.available
	}
	matched := provider
	return &matched, c.available
}

func bridgeProviderForInstance(
	ctx context.Context,
	bridges BridgeService,
	instance bridgepkg.BridgeInstance,
	providerCatalog *bridgeProviderCatalog,
) (*bridgepkg.BridgeProvider, bool, error) {
	if providerCatalog == nil {
		loadedCatalog, err := loadBridgeProviderCatalog(ctx, bridges)
		if err != nil {
			return nil, false, err
		}
		providerCatalog = &loadedCatalog
	}
	provider, catalogAvailable := providerCatalog.providerForInstance(instance)
	return provider, catalogAvailable, nil
}

func bridgeSecretBindingsForDiagnostics(
	ctx context.Context,
	bridges BridgeService,
	instance bridgepkg.BridgeInstance,
	provider *bridgepkg.BridgeProvider,
) ([]bridgepkg.BridgeSecretBinding, error) {
	if provider == nil || !bridgeProviderHasRequiredSecretSlots(*provider) {
		return nil, nil
	}
	bindings, err := bridges.ListSecretBindings(ctx, strings.TrimSpace(instance.ID))
	if err != nil {
		return nil, fmt.Errorf("load bridge secret bindings for diagnostics: %w", err)
	}
	return bindings, nil
}

func bridgeProviderHasRequiredSecretSlots(provider bridgepkg.BridgeProvider) bool {
	for _, slot := range provider.SecretSlots {
		normalized := slot.Normalize()
		if normalized.Required && strings.TrimSpace(normalized.Name) != "" {
			return true
		}
	}
	return false
}
