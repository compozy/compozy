package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	registrypkg "github.com/compozy/agh/internal/registry"
)

func (h *BaseHandlers) marketplaceEntry(
	ctx context.Context,
	kind string,
	entryID string,
	installedName string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	if installedName != "" {
		switch kind {
		case contract.MarketplaceKindMCP:
			return h.installedMCPMarketplaceEntryByName(ctx, installedName, scope)
		case contract.MarketplaceKindExtension:
			return h.installedExtensionMarketplaceEntryByName(ctx, installedName)
		case contract.MarketplaceKindSkill:
			return h.installedSkillMarketplaceEntryByName(ctx, installedName, scope)
		default:
			return contract.MarketplaceEntryResponse{}, marketplaceValidationf(
				"installed_name is not supported for marketplace kind %q", kind,
			)
		}
	}
	switch kind {
	case contract.MarketplaceKindMCP:
		return h.marketplaceCuratedOrInstalledEntry(ctx, marketplacepkg.KindMCP, entryID, scope)
	case contract.MarketplaceKindExtension:
		return h.marketplaceCuratedOrInstalledEntry(ctx, marketplacepkg.KindExtension, entryID, scope)
	case contract.MarketplaceKindSkill:
		return h.skillMarketplaceEntry(ctx, entryID, scope)
	case contract.MarketplaceKindBundle:
		return h.bundleMarketplaceEntry(ctx, entryID, scope)
	default:
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceNotFound, fmt.Errorf("unknown marketplace kind %q", kind),
		)
	}
}

func (h *BaseHandlers) curatedMarketplaceEntry(
	ctx context.Context,
	kind marketplacepkg.Kind,
	entryID string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	if h == nil || h.MarketplaceCatalog == nil {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceUnavailable, errors.New("catalog is not configured"),
		)
	}
	entry, err := h.MarketplaceCatalog.Detail(ctx, kind, entryID)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	if entry == nil {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceNotFound, fmt.Errorf("marketplace %s entry %q not found", kind, entryID),
		)
	}
	installed, err := h.marketplaceInstallIndex(ctx, string(kind), scope)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	listing, err := h.curatedMarketplaceListing(ctx, *entry, installed)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	details, err := marketplacepkg.ProjectEntry(*entry)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	response := contract.MarketplaceEntryResponse{Entry: listing}
	switch kind {
	case marketplacepkg.KindMCP:
		response.MCP = marketplaceMCPDetail(details)
	case marketplacepkg.KindExtension:
		response.Extension = marketplaceExtensionDetail(details)
	case marketplacepkg.KindSkill:
		response.Skill = marketplaceSkillDetail(details, nil)
	}
	return response, nil
}

func (h *BaseHandlers) skillMarketplaceEntry(
	ctx context.Context,
	entryID string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	if slug, ok := decodeRemoteSkillEntryID(entryID); ok {
		return h.remoteSkillMarketplaceEntry(ctx, entryID, slug)
	}

	response, err := h.curatedMarketplaceEntry(ctx, marketplacepkg.KindSkill, entryID, scope)
	if err != nil {
		installed, installedErr := h.installedSkillMarketplaceEntry(ctx, entryID, scope)
		if installedErr == nil {
			return installed, nil
		}
		if !errors.Is(installedErr, ErrMarketplaceNotFound) {
			return contract.MarketplaceEntryResponse{}, installedErr
		}
		return contract.MarketplaceEntryResponse{}, err
	}
	if response.Skill != nil {
		detail, detailErr := h.skillMarketplaceService().Info(ctx, response.Skill.InstallSlug)
		switch {
		case detailErr != nil:
			h.Logger.Warn(
				"api: marketplace curated skill enrichment failed",
				"entry_id", entryID,
				"install_slug", response.Skill.InstallSlug,
				"error", detailErr,
			)
		case detail == nil:
			h.Logger.Warn(
				"api: marketplace curated skill enrichment returned no detail",
				"entry_id", entryID,
				"install_slug", response.Skill.InstallSlug,
			)
		default:
			response.Skill.Readme = detail.Readme
			response.Skill.License = detail.License
			response.Skill.Repository = detail.Repository
			response.Skill.Versions = append([]string(nil), detail.Versions...)
			if len(response.Skill.Tags) == 0 {
				response.Skill.Tags = append([]string(nil), detail.Tags...)
			}
		}
	}
	return response, nil
}

func (h *BaseHandlers) remoteSkillMarketplaceEntry(
	ctx context.Context,
	entryID string,
	slug string,
) (contract.MarketplaceEntryResponse, error) {
	if h == nil {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceUnavailable,
			errors.New("skill marketplace is not configured"),
		)
	}
	detail, err := h.skillMarketplaceService().Info(ctx, slug)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, normalizeSkillMarketplaceError(err)
	}
	installed, err := h.skillInstallIndex(ctx)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	listing := remoteSkillMarketplaceListing(detail.Listing, installed)
	listing.EntryID = entryID
	return contract.MarketplaceEntryResponse{
		Entry: listing,
		Skill: marketplaceSkillDetail(marketplacepkg.EntryDetails{
			Skill: &marketplacepkg.SkillEntryDetails{InstallSlug: slug},
		}, detail),
	}, nil
}

func (h *BaseHandlers) bundleMarketplaceEntry(
	ctx context.Context,
	entryID string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	if h == nil || h.Bundles == nil {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceUnavailable, errors.New("bundle catalog is not configured"),
		)
	}
	extensionName, bundleName, ok := decodeBundleEntryID(entryID)
	if !ok {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceNotFound, fmt.Errorf("bundle entry %q not found", entryID),
		)
	}
	catalog, err := h.Bundles.Catalog(ctx)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	var found *bundlepkg.CatalogEntry
	for index := range catalog {
		entry := &catalog[index]
		if entry.ExtensionName == extensionName && entry.Bundle.Name == bundleName {
			found = entry
			break
		}
	}
	if found == nil {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceNotFound, fmt.Errorf("bundle entry %q not found", entryID),
		)
	}
	activations, err := h.Bundles.ListActivations(ctx)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	activation, installed := bundleActivationIndex(activations, scope)[bundleIdentityKey(extensionName, bundleName)]
	listing := contract.MarketplaceListingPayload{
		Kind: contract.MarketplaceKindBundle, EntryID: entryID, Name: found.Bundle.Name,
		Description: found.Bundle.Description, Source: found.ExtensionName,
		Installed: installed, UpdateAvailable: installed && activation.SpecDrift,
	}
	if installed {
		listing.ManagePath = marketplaceBundleActivationPath(activation.Activation.ID)
	}
	profiles := make([]contract.MarketplaceBundleProfilePayload, 0, len(found.Bundle.Profiles))
	for _, profile := range found.Bundle.Profiles {
		profiles = append(profiles, contract.MarketplaceBundleProfilePayload{
			Name: profile.Name, Description: profile.Description, Agents: len(profile.Agents),
			Jobs: len(profile.Jobs), Triggers: len(profile.Triggers), Bridges: len(profile.Bridges),
			Channels: len(profile.Channels.Items), Layouts: len(profile.Layouts),
		})
	}
	return contract.MarketplaceEntryResponse{
		Entry: listing,
		Bundle: &contract.MarketplaceBundleDetailPayload{
			ExtensionName: found.ExtensionName, Profiles: profiles,
		},
	}, nil
}

func marketplaceMCPDetail(details marketplacepkg.EntryDetails) *contract.MarketplaceMCPDetailPayload {
	if details.MCP == nil {
		return nil
	}
	result := &contract.MarketplaceMCPDetailPayload{
		Transport: details.MCP.Transport, Command: details.MCP.Command,
		Args: append([]string(nil), details.MCP.Args...), URL: details.MCP.URL,
		DefaultScope: details.MCP.DefaultScope,
		Env:          make([]contract.MarketplaceMCPEnvFieldPayload, 0, len(details.MCP.Env)),
	}
	for _, field := range details.MCP.Env {
		result.Env = append(result.Env, contract.MarketplaceMCPEnvFieldPayload{
			Name: field.Name, Prompt: field.Prompt, Required: field.Required,
			Secret: field.Secret, Default: field.Default,
		})
	}
	if details.MCP.OAuth != nil {
		result.OAuth = &contract.MarketplaceMCPOAuthPayload{
			IssuerURL: details.MCP.OAuth.IssuerURL, AuthorizationURL: details.MCP.OAuth.AuthorizationURL,
			TokenURL: details.MCP.OAuth.TokenURL, ClientID: details.MCP.OAuth.ClientID,
			Scopes: append([]string(nil), details.MCP.OAuth.Scopes...),
		}
	}
	return result
}

func marketplaceExtensionDetail(
	details marketplacepkg.EntryDetails,
) *contract.MarketplaceExtensionDetailPayload {
	if details.Extension == nil {
		return nil
	}
	return &contract.MarketplaceExtensionDetailPayload{
		InstallSlug:  details.Extension.InstallSlug,
		ArtifactURL:  details.Extension.ArtifactURL,
		DigestSHA256: details.Extension.DigestSHA256,
		Repository:   details.Extension.Repository,
	}
}

func marketplaceSkillDetail(
	details marketplacepkg.EntryDetails,
	remote *registrypkg.Detail,
) *contract.MarketplaceSkillDetailPayload {
	if details.Skill == nil {
		return nil
	}
	result := &contract.MarketplaceSkillDetailPayload{
		InstallSlug: details.Skill.InstallSlug, DisplayName: details.Skill.DisplayName,
		Tags: append([]string(nil), details.Skill.Tags...),
	}
	if remote != nil {
		result.Readme = remote.Readme
		result.License = remote.License
		result.Repository = remote.Repository
		result.Versions = append([]string(nil), remote.Versions...)
		if len(result.Tags) == 0 {
			result.Tags = append([]string(nil), remote.Tags...)
		}
	}
	return result
}
