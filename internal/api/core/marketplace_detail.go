package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

func (h *BaseHandlers) marketplaceEntry(
	ctx context.Context,
	kind contract.MarketplaceKind,
	entryID string,
	installedName string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	if installedName != "" {
		switch kind {
		case contract.MarketplaceKindMCP:
			return h.installedMCPMarketplaceEntryByName(ctx, installedName, scope)
		case contract.MarketplaceKindExtension:
			return h.installedExtensionMarketplaceEntryByName(ctx, installedName, scope)
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
		return contract.MarketplaceEntryResponse{}, normalizeCuratedMarketplaceError(err)
	}
	if entry == nil {
		return contract.MarketplaceEntryResponse{}, errors.Join(
			ErrMarketplaceNotFound, fmt.Errorf("marketplace %s entry %q not found", kind, entryID),
		)
	}
	installed, err := h.marketplaceInstallIndex(ctx, contract.MarketplaceKind(kind), scope)
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
		mcpDetail, err := marketplaceMCPDetail(details)
		if err != nil {
			return contract.MarketplaceEntryResponse{}, err
		}
		response.MCP = mcpDetail
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

func marketplaceMCPDetail(details marketplacepkg.EntryDetails) (*contract.MarketplaceMCPDetailPayload, error) {
	if details.MCP == nil {
		return nil, nil
	}
	result := &contract.MarketplaceMCPDetailPayload{
		Launch: contract.MarketplaceMCPLaunchPayload{
			Type: details.MCP.Launch.Type, Package: details.MCP.Launch.Package,
			Version: details.MCP.Launch.Version, Image: details.MCP.Launch.Image,
			Digest: details.MCP.Launch.Digest, URL: details.MCP.Launch.URL,
			Args: append([]string(nil), details.MCP.Launch.Args...),
		},
		DefaultScope: details.MCP.DefaultScope,
		Inputs:       make([]contract.MarketplaceMCPInputPayload, 0, len(details.MCP.Inputs)),
	}
	for _, input := range details.MCP.Inputs {
		var defaultValue any
		if len(input.Default) > 0 {
			if err := json.Unmarshal(input.Default, &defaultValue); err != nil {
				return nil, fmt.Errorf("marketplace MCP input %q default: %w", input.ID, err)
			}
		}
		result.Inputs = append(result.Inputs, contract.MarketplaceMCPInputPayload{
			ID: input.ID, Prompt: input.Prompt, Type: input.Type, Required: input.Required,
			Binding: contract.MarketplaceMCPInputBindingPayload{Type: input.Binding.Type, Name: input.Binding.Name},
			Default: defaultValue,
		})
	}
	if details.MCP.Auth != nil {
		result.Auth = &contract.MarketplaceMCPOAuthPayload{
			Method: details.MCP.Auth.Method, Registration: details.MCP.Auth.Registration,
			Scopes: append([]string(nil), details.MCP.Auth.Scopes...),
		}
	}
	return result, nil
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
