package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/skills"
)

func (h *BaseHandlers) marketplaceCuratedOrInstalledEntry(
	ctx context.Context,
	kind marketplacepkg.Kind,
	entryID string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	curated, curatedErr := h.curatedMarketplaceEntry(ctx, kind, entryID, scope)
	if curatedErr == nil {
		return curated, nil
	}

	var installed contract.MarketplaceEntryResponse
	var installedErr error
	switch kind {
	case marketplacepkg.KindExtension:
		installed, installedErr = h.installedExtensionMarketplaceEntry(ctx, entryID, false)
	case marketplacepkg.KindMCP:
		installed, installedErr = h.installedMCPMarketplaceEntry(ctx, entryID, scope, false)
	default:
		installedErr = errors.Join(
			ErrMarketplaceNotFound,
			fmt.Errorf("installed marketplace %s entry %q not found", kind, entryID),
		)
	}
	if installedErr == nil {
		return installed, nil
	}
	if !errors.Is(installedErr, ErrMarketplaceNotFound) {
		return contract.MarketplaceEntryResponse{}, installedErr
	}
	return contract.MarketplaceEntryResponse{}, curatedErr
}

func (h *BaseHandlers) installedExtensionMarketplaceEntry(
	ctx context.Context,
	entryID string,
	exactName bool,
) (contract.MarketplaceEntryResponse, error) {
	if h == nil || h.Extensions == nil {
		return contract.MarketplaceEntryResponse{}, marketplaceInstalledEntryNotFound(
			contract.MarketplaceKindExtension,
			entryID,
		)
	}
	items, err := h.Extensions.List(ctx)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	for _, item := range items {
		catalogEntryID := ""
		if item.Provenance != nil {
			catalogEntryID = strings.TrimSpace(item.Provenance.CatalogEntryID)
		}
		name := strings.TrimSpace(item.Name)
		if name != entryID && (exactName || catalogEntryID != entryID) {
			continue
		}
		source := strings.TrimSpace(item.Source)
		if item.Provenance != nil && strings.TrimSpace(item.Provenance.InstalledFrom) != "" {
			source = strings.TrimSpace(item.Provenance.InstalledFrom)
		}
		return contract.MarketplaceEntryResponse{Entry: contract.MarketplaceListingPayload{
			Kind: contract.MarketplaceKindExtension, EntryID: entryID, Name: name,
			Version: strings.TrimSpace(item.Version), Source: source, Installed: true,
			InstalledName: name, InstalledVersion: strings.TrimSpace(item.Version),
			ManagePath: marketplaceExtensionsInstalledPath, Trust: item.Trust,
		}}, nil
	}
	return contract.MarketplaceEntryResponse{}, marketplaceInstalledEntryNotFound(
		contract.MarketplaceKindExtension,
		entryID,
	)
}

func (h *BaseHandlers) installedExtensionMarketplaceEntryByName(
	ctx context.Context,
	name string,
) (contract.MarketplaceEntryResponse, error) {
	return h.installedExtensionMarketplaceEntry(ctx, name, true)
}

func (h *BaseHandlers) installedMCPMarketplaceEntry(
	ctx context.Context,
	entryID string,
	scope marketplaceReadScope,
	exactName bool,
) (contract.MarketplaceEntryResponse, error) {
	if h == nil || h.Settings == nil {
		return contract.MarketplaceEntryResponse{}, marketplaceInstalledEntryNotFound(
			contract.MarketplaceKindMCP,
			entryID,
		)
	}
	envelope, err := h.Settings.ListCollection(ctx, settingspkg.CollectionRequest{
		Collection:  settingspkg.CollectionMCPServers,
		Scope:       scope.scope,
		WorkspaceID: scope.workspaceID,
	})
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	for _, item := range envelope.MCPServers {
		name := strings.TrimSpace(item.Name)
		catalogEntryID := strings.TrimSpace(item.CatalogEntry)
		if name != entryID && (exactName || catalogEntryID != entryID) {
			continue
		}
		version := strings.TrimSpace(item.CatalogVersion)
		return contract.MarketplaceEntryResponse{
			Entry: contract.MarketplaceListingPayload{
				Kind: contract.MarketplaceKindMCP, EntryID: entryID, Name: name,
				Version: version, Source: "installed", Transport: string(item.Transport),
				Installed: true, InstalledName: name, InstalledVersion: version,
				ManagePath: "/marketplace/mcps?tab=installed",
			},
			MCP: &contract.MarketplaceMCPDetailPayload{
				Transport: string(item.Transport), Command: item.Command,
				Args: append([]string(nil), item.Args...), URL: item.URL,
			},
		}, nil
	}
	return contract.MarketplaceEntryResponse{}, marketplaceInstalledEntryNotFound(
		contract.MarketplaceKindMCP,
		entryID,
	)
}

func (h *BaseHandlers) installedMCPMarketplaceEntryByName(
	ctx context.Context,
	name string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	return h.installedMCPMarketplaceEntry(ctx, name, scope, true)
}

func (h *BaseHandlers) installedSkillMarketplaceEntry(
	ctx context.Context,
	entryID string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	if h == nil || h.SkillsRegistry == nil {
		return contract.MarketplaceEntryResponse{}, marketplaceInstalledEntryNotFound(
			contract.MarketplaceKindSkill,
			entryID,
		)
	}
	var skillList []*skills.Skill
	if scope.scope == settingspkg.ScopeWorkspace {
		if h.Workspaces == nil {
			return contract.MarketplaceEntryResponse{}, errors.Join(
				ErrMarketplaceUnavailable,
				errors.New("workspace resolver is not configured"),
			)
		}
		resolved, err := h.Workspaces.Resolve(ctx, scope.workspaceID)
		if err != nil {
			return contract.MarketplaceEntryResponse{}, err
		}
		skillList, err = h.SkillsRegistry.ForWorkspace(ctx, &resolved)
		if err != nil {
			return contract.MarketplaceEntryResponse{}, err
		}
	} else {
		skillList = h.SkillsRegistry.List()
	}
	for _, item := range skillList {
		if item == nil || strings.TrimSpace(item.Meta.Name) != entryID {
			continue
		}
		payload := SkillPayloadFromSkill(item)
		installSlug := ""
		if item.Provenance != nil {
			installSlug = strings.TrimSpace(item.Provenance.Slug)
		}
		return contract.MarketplaceEntryResponse{
			Entry: contract.MarketplaceListingPayload{
				Kind: contract.MarketplaceKindSkill, EntryID: entryID,
				Name: payload.Name, Description: payload.Description, Version: payload.Version,
				Source: payload.Source, InstallSlug: installSlug, Installed: true,
				InstalledName: payload.Name, InstalledVersion: payload.Version,
				ManagePath: "/marketplace/skills?tab=installed",
			},
			Skill: &contract.MarketplaceSkillDetailPayload{InstallSlug: installSlug},
		}, nil
	}
	return contract.MarketplaceEntryResponse{}, marketplaceInstalledEntryNotFound(
		contract.MarketplaceKindSkill,
		entryID,
	)
}

func (h *BaseHandlers) installedSkillMarketplaceEntryByName(
	ctx context.Context,
	name string,
	scope marketplaceReadScope,
) (contract.MarketplaceEntryResponse, error) {
	return h.installedSkillMarketplaceEntry(ctx, name, scope)
}

func marketplaceInstalledEntryNotFound(kind string, entryID string) error {
	return errors.Join(
		ErrMarketplaceNotFound,
		fmt.Errorf("installed marketplace %s entry %q not found", kind, entryID),
	)
}
