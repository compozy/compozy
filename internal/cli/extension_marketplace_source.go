package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/marketplace"
)

// resolvePluginInstallPlan preserves existing curated acquisition refs before choosing a plugin source.
func resolvePluginInstallPlan(
	ctx context.Context,
	deps commandDeps,
	plan extensionInstallPlan,
) (extensionInstallPlan, error) {
	if len(plan.Attempts) == 0 {
		return plan, nil
	}
	request := plan.Attempts[0]
	explicit := request.Source == contract.InstallExtensionSourceMarketplace
	if !explicit && (request.Source != contract.InstallExtensionSourceCurated || len(plan.Attempts) < 2) {
		return plan, nil
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return extensionInstallPlan{}, err
	}
	sourcesClient, ok := client.(marketplaceSourcesReader)
	if !ok {
		return extensionInstallPlan{}, errors.New("cli: marketplace source index is unavailable")
	}
	response, err := sourcesClient.ListMarketplaceSources(ctx)
	if err != nil {
		return extensionInstallPlan{}, err
	}
	states := make([]marketplace.SourceState, 0, len(response.Sources))
	for _, source := range response.Sources {
		states = append(states, marketplace.SourceState{Source: source.Name, SourceRef: source.Source})
	}
	origin, err := marketplace.ParseInstallSlug(request.Ref, states)
	if !explicit && errors.Is(err, marketplace.ErrSourceNotFound) {
		return plan, nil
	}
	if err != nil {
		return extensionInstallPlan{}, err
	}
	if origin.SourceRef == marketplace.CompozyCatalogRef {
		return plan, nil
	}
	if !explicit {
		preview, previewErr := client.PreviewExtensionInstall(ctx, request)
		if previewErr == nil {
			request.ExpectedDigest = preview.DigestSHA256
			return extensionInstallPlan{Attempts: []InstallExtensionRequest{request}}, nil
		}
		if !extensionInstallFallbackAllowed(previewErr) {
			return extensionInstallPlan{}, previewErr
		}
	}
	name, _, _ := strings.Cut(request.Ref, "/")
	name = strings.ToLower(name)
	entry, err := client.MarketplaceInfo(
		ctx,
		origin.EntryID,
		name,
		"",
		MarketplaceReadScope{Scope: contract.SettingsLayeredScopeUser},
	)
	if err != nil {
		return extensionInstallPlan{}, err
	}
	request.Source, request.Ref = contract.InstallExtensionSourceMarketplace, name+"/"+origin.EntryID
	request.ExpectedDigest = entry.Entry.DigestSHA256
	return extensionInstallPlan{Attempts: []InstallExtensionRequest{request}}, nil
}
