package core

import (
	"context"
	"net/http"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) ListMarketplaceSections(c *gin.Context) {
	if c.Request.URL.Query().Has("kind") {
		h.respondMarketplaceError(c, marketplaceValidationf("kind is not supported by the Marketplace catalog"))
		return
	}
	perSection := 6
	if c.Request.URL.Query().Has("per_section") {
		raw := c.Query("per_section")
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 12 {
			c.JSON(
				http.StatusBadRequest,
				contract.ErrorPayload{Error: "per_section must be between 1 and 12", Code: "invalid_request"},
			)
			return
		}
		perSection = parsed
	}
	actor, ok := h.marketplaceCatalogReadActor(c, "browse")
	if !ok {
		return
	}
	response, err := h.MarketplaceSections(
		c.Request.Context(),
		MarketplaceListRequest{
			Query:       c.Query("q"),
			Scope:       c.Query("scope"),
			WorkspaceID: c.Query("workspace_id"),
			ProfileName: c.Query("profile"),
			Actor:       actor,
		},
		perSection,
	)
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) MarketplaceSections(
	ctx context.Context,
	request MarketplaceListRequest,
	perSection int,
) (contract.MarketplaceSectionsResponse, error) {
	if h == nil || h.MarketplaceCatalog == nil {
		return contract.MarketplaceSectionsResponse{}, ErrMarketplaceUnavailable
	}
	scope, err := parseMarketplaceCatalogScope(request.Scope, request.WorkspaceID, request.ProfileName)
	if err != nil {
		return contract.MarketplaceSectionsResponse{}, err
	}
	scope.actor = request.Actor
	installed, err := h.extensionInstallIndex(ctx, scope)
	if err != nil {
		return contract.MarketplaceSectionsResponse{}, err
	}
	origins := make([]marketplacepkg.Origin, 0, len(installed.byOrigin))
	for origin := range installed.byOrigin {
		origins = append(origins, origin)
	}
	page, err := h.MarketplaceCatalog.BrowseFiltered(
		ctx,
		marketplacepkg.BrowseFilter{Query: request.Query, Installed: origins},
		0,
		50_000,
	)
	if err != nil {
		return contract.MarketplaceSectionsResponse{}, normalizeCuratedMarketplaceError(err)
	}
	sections, err := marketplacepkg.BuildSections(page, perSection)
	if err != nil {
		return contract.MarketplaceSectionsResponse{}, err
	}
	response := contract.MarketplaceSectionsResponse{
		Sections:   []contract.MarketplaceSection{},
		Sources:    []contract.MarketplaceSourceSummary{},
		Revision:   page.Revision,
		Stale:      page.Stale,
		Refreshing: page.Refreshing,
		ErrorClass: page.ErrorClass,
		Error:      h.marketplaceCatalogDiagnostic(page.LastError),
	}
	for _, source := range page.Sources {
		if source.Enabled {
			response.Sources = append(response.Sources, marketplaceSourceSummary(source))
		}
	}
	for _, section := range sections.Sections {
		value := contract.MarketplaceSection{
			ID:    string(section.Category.ID),
			Label: section.Category.Label,
			Total: section.Total,
			Items: []contract.MarketplaceListingPayload{},
		}
		for _, entry := range section.Entries {
			listing, err := h.catalogMarketplaceListing(ctx, entry, installed)
			if err != nil {
				return contract.MarketplaceSectionsResponse{}, err
			}
			normalizeCatalogListing(&listing)
			value.Items = append(value.Items, listing)
		}
		response.Sections = append(response.Sections, value)
	}
	return response, nil
}
