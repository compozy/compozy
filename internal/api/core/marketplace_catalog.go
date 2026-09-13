package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

var ErrMarketplaceCursorStale = errors.New("marketplace: catalog revision changed")

// MarketplaceListRequest carries canonical catalog pagination and installed-state scope.
type MarketplaceListRequest struct {
	Query       string
	Cursor      string
	Limit       int
	Scope       string
	WorkspaceID string
	ProfileName string
	Actor       *taskpkg.ActorContext
}

// ListMarketplace serves the one-catalog envelope on HTTP and UDS.
func (h *BaseHandlers) ListMarketplace(c *gin.Context) {
	if c.Request.URL.Query().Has("kind") {
		h.respondMarketplaceError(c, marketplaceValidationf("kind is not supported by the Marketplace catalog"))
		return
	}

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
	response, err := h.MarketplaceList(c.Request.Context(), MarketplaceListRequest{
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
	request MarketplaceListRequest,
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
	offset, fence, err := marketplaceCursorOffset(request.Cursor, query, scope)
	if err != nil {
		return contract.MarketplaceListResponse{}, err
	}
	page, err := h.MarketplaceCatalog.Browse(ctx, query, offset, limit)
	if err != nil {
		return contract.MarketplaceListResponse{}, normalizeCuratedMarketplaceError(err)
	}
	if fence != "" && fence != page.Revision {
		return contract.MarketplaceListResponse{}, ErrMarketplaceCursorStale
	}
	installed, err := h.extensionInstallIndex(ctx, scope)
	if err != nil {
		return contract.MarketplaceListResponse{}, err
	}
	response := contract.MarketplaceListResponse{
		Total:      page.Total,
		Revision:   page.Revision,
		Stale:      page.Stale,
		Refreshing: page.Refreshing,
		ErrorClass: page.ErrorClass,
		Error:      h.marketplaceCatalogDiagnostic(page.LastError),
		Items: make(
			[]contract.MarketplaceListingPayload,
			0,
			len(page.Entries),
		),
		Sources: []contract.MarketplaceSourceSummary{},
	}
	for _, source := range page.Sources {
		if source.Enabled {
			response.Sources = append(response.Sources, marketplaceSourceSummary(source))
		}
	}
	for _, entry := range page.Entries {
		listing, err := h.catalogMarketplaceListing(ctx, entry, installed)
		if err != nil {
			return contract.MarketplaceListResponse{}, err
		}
		normalizeCatalogListing(&listing)
		response.Items = append(response.Items, listing)
	}
	response.NextCursor, err = marketplaceNextCursor(
		query,
		scope,
		page.Revision,
		offset+len(page.Entries),
		offset+len(page.Entries) < page.Total,
	)
	if err != nil {
		return contract.MarketplaceListResponse{}, err
	}
	return response, nil
}

func (h *BaseHandlers) GetMarketplaceCatalogEntry(c *gin.Context) {
	if c.Request.URL.Query().Has("kind") {
		h.respondMarketplaceError(c, marketplaceValidationf("kind is not supported by the Marketplace catalog"))
		return
	}

	source := strings.TrimSpace(c.Query("source"))
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
		response, err = h.installedExtensionMarketplaceEntry(c.Request.Context(), name, scope)
	} else {
		response, err = h.catalogMarketplaceEntry(
			c.Request.Context(), source,
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
	if listing.SourceRef == marketplacepkg.CompozyCatalogRef {
		listing.Source = marketplacepkg.CompozyCatalogSource
		listing.InstallSlug = "compozy/" + listing.EntryID
	}
	listing.ManagePath = marketplaceExtensionsInstalledPath
}

func marketplaceSourceSummary(state marketplacepkg.SourceState) contract.MarketplaceSourceSummary {
	source := contract.MarketplaceSourceSummary{
		Name:  state.Source,
		Kind:  state.Kind,
		State: "ok",
		Count: state.EntryCount,
	}
	if state.FetchedAt.IsZero() && state.LastError == "" {
		source.State = "never"
	} else if state.ErrorClass != "" || state.LastError != "" {
		source.State = "degraded"
	}
	if !state.FetchedAt.IsZero() {
		source.LastReadAt = new(state.FetchedAt)
	}
	return source
}
