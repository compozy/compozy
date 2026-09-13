package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	"github.com/gin-gonic/gin"
)

var ErrMarketplaceCursorStale = errors.New("marketplace: catalog revision changed")

// ListMarketplace serves the one-catalog envelope on HTTP and UDS.
func (h *BaseHandlers) ListMarketplace(c *gin.Context) {
	limit := maxMarketplaceLimit
	var err error
	if c.Query("limit") != "" {
		limit, err = marketplaceLimit(c.Query("limit"))
	}
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	actor, ok := h.marketplaceCatalogReadActor(c, "browse")
	if !ok {
		return
	}
	response, err := h.MarketplaceList(c.Request.Context(), MarketplaceKindRequest{
		Query: c.Query("q"), Cursor: c.Query("cursor"), Limit: limit,
		Scope: c.Query("scope"), WorkspaceID: c.Query("workspace_id"), ProfileName: c.Query("profile"), Actor: actor,
	})
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) MarketplaceList(
	ctx context.Context,
	request MarketplaceKindRequest,
) (contract.MarketplaceListResponse, error) {
	if h == nil || h.MarketplaceCatalog == nil {
		return contract.MarketplaceListResponse{}, ErrMarketplaceUnavailable
	}
	limit := request.Limit
	if limit == 0 {
		limit = maxMarketplaceLimit
	}
	limit, err := marketplaceLimitValue(limit)
	if err != nil {
		return contract.MarketplaceListResponse{}, err
	}
	scope, err := parseMarketplaceCatalogScope(request.Scope, request.WorkspaceID, request.ProfileName)
	if err != nil {
		return contract.MarketplaceListResponse{}, err
	}
	scope.actor = request.Actor
	query := marketplacepkg.NormalizeQuery(request.Query)
	offset, fence, err := marketplaceCursorOffset(request.Cursor, "catalog", query, scope)
	if err != nil {
		return contract.MarketplaceListResponse{}, err
	}
	page, err := h.MarketplaceCatalog.Browse(ctx, marketplacepkg.KindExtension, query, offset, limit)
	if err != nil && !page.State.Stale {
		return contract.MarketplaceListResponse{}, normalizeCuratedMarketplaceError(err)
	}
	if fence != "" && fence != page.State.Revision {
		return contract.MarketplaceListResponse{}, ErrMarketplaceCursorStale
	}
	installed, err := h.extensionInstallIndex(ctx, scope)
	if err != nil {
		return contract.MarketplaceListResponse{}, err
	}
	response := contract.MarketplaceListResponse{
		Total:      page.Total,
		Revision:   page.State.Revision,
		Stale:      page.State.Stale,
		ErrorClass: page.State.ErrorClass,
		Error:      h.marketplaceKindDiagnostic(page.State.LastError),
		Items: make(
			[]contract.MarketplaceListingPayload,
			0,
			len(page.Entries),
		),
		Sources: []contract.MarketplaceSourceSummary{marketplaceSourceSummary(page.State)},
	}
	for _, entry := range page.Entries {
		listing, err := h.curatedMarketplaceListing(ctx, entry, installed)
		if err != nil {
			return contract.MarketplaceListResponse{}, err
		}
		normalizeCatalogListing(&listing)
		response.Items = append(response.Items, listing)
	}
	response.NextCursor, err = marketplaceNextCursor(
		"catalog",
		query,
		scope,
		page.State.Revision,
		offset+len(page.Entries),
		offset+len(page.Entries) < page.Total,
	)
	if err != nil {
		return contract.MarketplaceListResponse{}, err
	}
	return response, nil
}

func (h *BaseHandlers) GetMarketplaceCatalogEntry(c *gin.Context) {
	source := strings.TrimSpace(c.Query("source"))
	if source != "" && source != marketplacepkg.CompozyCatalogSource {
		h.respondMarketplaceError(c, ErrMarketplaceNotFound)
		return
	}
	actor, ok := h.marketplaceCatalogReadActor(c, "entry")
	if !ok {
		return
	}
	scope, err := parseMarketplaceCatalogScope(c.Query("scope"), c.Query("workspace_id"), c.Query("profile"))
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	scope.actor = actor
	var response contract.MarketplaceEntryResponse
	if name := strings.TrimSpace(c.Query("installed_name")); name != "" {
		response, err = h.installedExtensionMarketplaceEntryByName(c.Request.Context(), name, scope)
	} else {
		response, err = h.curatedMarketplaceEntry(
			c.Request.Context(),
			marketplacepkg.KindExtension,
			c.Param("entry_id"),
			scope,
		)
	}
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	normalizeCatalogListing(&response.Entry)
	c.JSON(http.StatusOK, response)
}

func normalizeCatalogListing(listing *contract.MarketplaceListingPayload) {
	listing.Kind = ""
	if listing.SourceRef == marketplacepkg.CompozyCatalogRef {
		listing.Source = marketplacepkg.CompozyCatalogSource
		listing.InstallSlug = "compozy/" + listing.EntryID
	}
	listing.ManagePath = marketplaceExtensionsInstalledPath
}

func marketplaceSourceSummary(state marketplacepkg.KindState) contract.MarketplaceSourceSummary {
	source := contract.MarketplaceSourceSummary{
		Name:  marketplacepkg.CompozyCatalogSource,
		Kind:  "feed",
		State: "ok",
		Count: state.EntryCount,
	}
	if state.Stale {
		source.State = "degraded"
	} else if state.FetchedAt.IsZero() {
		source.State = "never"
	}
	if !state.FetchedAt.IsZero() {
		source.LastReadAt = new(state.FetchedAt)
	}
	return source
}
