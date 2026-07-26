package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	settingspkg "github.com/compozy/agh/internal/settings"
	skillmarketplace "github.com/compozy/agh/internal/skills/marketplace"
	"github.com/gin-gonic/gin"
)

const (
	defaultMarketplaceLimit = 20
	maxMarketplaceLimit     = 100
)

var (
	ErrMarketplaceValidation  = errors.New("marketplace: validation error")
	ErrMarketplaceNotFound    = errors.New("marketplace: not found")
	ErrMarketplaceUnavailable = errors.New("marketplace: service unavailable")
)

var marketplaceKindOrder = []string{
	contract.MarketplaceKindMCP,
	contract.MarketplaceKindExtension,
	contract.MarketplaceKindSkill,
	contract.MarketplaceKindBundle,
}

type marketplaceReadScope struct {
	scope       settingspkg.ScopeKind
	workspaceID string
}

// MarketplaceSearchRequest captures transport-independent marketplace discovery filters.
type MarketplaceSearchRequest struct {
	Query       string
	Limit       int
	Scope       string
	WorkspaceID string
}

// MarketplaceKindRequest captures transport-independent single-kind discovery filters.
type MarketplaceKindRequest struct {
	Kind        string
	Query       string
	Cursor      string
	Limit       int
	Scope       string
	WorkspaceID string
}

// MarketplaceEntryRequest captures transport-independent marketplace detail identity and scope.
type MarketplaceEntryRequest struct {
	Kind          string
	EntryID       string
	InstalledName string
	Scope         string
	WorkspaceID   string
}

// SearchMarketplace returns fixed-order grouped marketplace discovery results.
func (h *BaseHandlers) SearchMarketplace(c *gin.Context) {
	limit, err := marketplaceLimit(c.Query("limit"))
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	response, err := h.MarketplaceSearch(c.Request.Context(), MarketplaceSearchRequest{
		Query:       c.Query("q"),
		Limit:       limit,
		Scope:       c.Query("scope"),
		WorkspaceID: c.Query("workspace_id"),
	})
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// MarketplaceSearch resolves grouped discovery for HTTP, UDS, CLI, and native-tool adapters.
func (h *BaseHandlers) MarketplaceSearch(
	ctx context.Context,
	request MarketplaceSearchRequest,
) (contract.MarketplaceSearchResponse, error) {
	limit, err := marketplaceLimitValue(request.Limit)
	if err != nil {
		return contract.MarketplaceSearchResponse{}, err
	}
	scope, err := parseMarketplaceReadScope(request.Scope, request.WorkspaceID)
	if err != nil {
		return contract.MarketplaceSearchResponse{}, err
	}
	query := strings.TrimSpace(request.Query)
	results := make([]contract.MarketplaceKindResult, 0, len(marketplaceKindOrder))
	unavailableKinds := 0
	for _, kind := range marketplaceKindOrder {
		page, kindErr := h.marketplaceKindResult(ctx, kind, query, 0, limit, scope)
		result := page.result
		if kindErr != nil {
			if errors.Is(kindErr, ErrMarketplaceUnavailable) {
				unavailableKinds++
			}
			errorMessage := kindErr.Error()
			if h != nil && h.MaskInternalErrors {
				errorMessage = http.StatusText(http.StatusInternalServerError)
			}
			result = contract.MarketplaceKindResult{
				Kind: kind, Error: errorMessage, Items: []contract.MarketplaceListingPayload{},
			}
		}
		results = append(results, result)
	}
	if unavailableKinds == len(marketplaceKindOrder) {
		return contract.MarketplaceSearchResponse{}, errors.Join(
			ErrMarketplaceUnavailable, errors.New("marketplace discovery dependencies are not configured"),
		)
	}
	return contract.MarketplaceSearchResponse{Query: query, Kinds: results}, nil
}

// BrowseMarketplaceKind returns one marketplace kind or a deterministic error.
func (h *BaseHandlers) BrowseMarketplaceKind(c *gin.Context) {
	limit, err := marketplaceLimit(c.Query("limit"))
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	response, err := h.MarketplaceKind(c.Request.Context(), MarketplaceKindRequest{
		Kind:        c.Param("kind"),
		Query:       c.Query("q"),
		Cursor:      c.Query("cursor"),
		Limit:       limit,
		Scope:       c.Query("scope"),
		WorkspaceID: c.Query("workspace_id"),
	})
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// MarketplaceKind resolves one discovery kind through the shared core seam.
func (h *BaseHandlers) MarketplaceKind(
	ctx context.Context,
	request MarketplaceKindRequest,
) (contract.MarketplaceKindResponse, error) {
	kind, err := parseMarketplaceKind(request.Kind)
	if err != nil {
		return contract.MarketplaceKindResponse{}, err
	}
	limit, err := marketplaceLimitValue(request.Limit)
	if err != nil {
		return contract.MarketplaceKindResponse{}, err
	}
	scope, err := parseMarketplaceReadScope(request.Scope, request.WorkspaceID)
	if err != nil {
		return contract.MarketplaceKindResponse{}, err
	}
	query := strings.TrimSpace(request.Query)
	offset, cursorFence, err := marketplaceCursorOffset(request.Cursor, kind, query, scope)
	if err != nil {
		return contract.MarketplaceKindResponse{}, err
	}
	page, err := h.marketplaceKindResult(ctx, kind, query, offset, limit, scope)
	if err != nil {
		return contract.MarketplaceKindResponse{}, err
	}
	if err := validateMarketplaceCursorFence(cursorFence, page.currentFence); err != nil {
		return contract.MarketplaceKindResponse{}, err
	}
	page.result.NextCursor, err = marketplaceNextCursor(
		kind,
		query,
		scope,
		page.nextFence,
		page.nextOffset,
		page.hasMore,
	)
	if err != nil {
		return contract.MarketplaceKindResponse{}, err
	}
	return contract.MarketplaceKindResponse(page.result), nil
}

// GetMarketplaceEntry returns one exact marketplace detail by stable entry_id.
func (h *BaseHandlers) GetMarketplaceEntry(c *gin.Context) {
	response, err := h.MarketplaceEntry(c.Request.Context(), MarketplaceEntryRequest{
		Kind:          c.Param("kind"),
		EntryID:       c.Param("entry_id"),
		InstalledName: c.Query("installed_name"),
		Scope:         c.Query("scope"),
		WorkspaceID:   c.Query("workspace_id"),
	})
	if err != nil {
		h.respondMarketplaceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// MarketplaceEntry resolves one exact marketplace entry through the shared core seam.
func (h *BaseHandlers) MarketplaceEntry(
	ctx context.Context,
	request MarketplaceEntryRequest,
) (contract.MarketplaceEntryResponse, error) {
	kind, err := parseMarketplaceKind(request.Kind)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	entryID := strings.TrimSpace(request.EntryID)
	if entryID == "" {
		return contract.MarketplaceEntryResponse{}, marketplaceValidationf("entry_id is required")
	}
	scope, err := parseMarketplaceReadScope(request.Scope, request.WorkspaceID)
	if err != nil {
		return contract.MarketplaceEntryResponse{}, err
	}
	return h.marketplaceEntry(ctx, kind, entryID, strings.TrimSpace(request.InstalledName), scope)
}

// RefreshMarketplaceCatalog refreshes all or one feed-backed marketplace kind.
func (h *BaseHandlers) RefreshMarketplaceCatalog(c *gin.Context) {
	if h == nil || h.MarketplaceCatalog == nil {
		h.respondMarketplaceError(c, errors.Join(ErrMarketplaceUnavailable, errors.New("catalog is not configured")))
		return
	}
	var kinds []marketplacepkg.Kind
	if rawKind := strings.TrimSpace(c.Query("kind")); rawKind != "" {
		kind, err := parseRefreshMarketplaceKind(rawKind)
		if err != nil {
			h.respondMarketplaceError(c, err)
			return
		}
		kinds = []marketplacepkg.Kind{kind}
	}
	report, refreshErr := h.MarketplaceCatalog.Refresh(c.Request.Context(), kinds...)
	if refreshErr != nil && len(report.Outcomes) == 0 {
		h.respondMarketplaceError(c, refreshErr)
		return
	}
	items := make([]contract.MarketplaceRefreshKindPayload, 0, len(report.Outcomes))
	for _, outcome := range report.Outcomes {
		items = append(items, contract.MarketplaceRefreshKindPayload{
			Kind: string(outcome.Kind), Outcome: outcome.Outcome, EntryCount: outcome.EntryCount,
			Stale: outcome.Stale, ErrorClass: outcome.ErrorClass,
		})
	}
	c.JSON(http.StatusOK, contract.MarketplaceRefreshResponse{Kinds: items})
}

func marketplaceLimit(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultMarketplaceLimit, nil
	}
	limit, err := strconv.Atoi(trimmed)
	if err != nil || limit <= 0 || limit > maxMarketplaceLimit {
		return 0, marketplaceValidationf("limit must be between 1 and %d", maxMarketplaceLimit)
	}
	return limit, nil
}

func marketplaceLimitValue(limit int) (int, error) {
	if limit == 0 {
		return defaultMarketplaceLimit, nil
	}
	if limit < 1 || limit > maxMarketplaceLimit {
		return 0, marketplaceValidationf("limit must be between 1 and %d", maxMarketplaceLimit)
	}
	return limit, nil
}

func parseMarketplaceReadScope(rawScope string, rawWorkspaceID string) (marketplaceReadScope, error) {
	scope := strings.ToLower(strings.TrimSpace(rawScope))
	workspaceID := strings.TrimSpace(rawWorkspaceID)
	if scope == "" {
		scope = contract.MarketplaceScopeGlobal
	}
	switch scope {
	case contract.MarketplaceScopeGlobal:
		if workspaceID != "" {
			return marketplaceReadScope{}, marketplaceValidationf("global scope must not include workspace_id")
		}
		return marketplaceReadScope{scope: settingspkg.ScopeGlobal}, nil
	case contract.MarketplaceScopeWorkspace:
		if workspaceID == "" {
			return marketplaceReadScope{}, marketplaceValidationf("workspace scope requires workspace_id")
		}
		return marketplaceReadScope{scope: settingspkg.ScopeWorkspace, workspaceID: workspaceID}, nil
	default:
		return marketplaceReadScope{}, marketplaceValidationf("unsupported scope %q", scope)
	}
}

func parseMarketplaceKind(raw string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if slices.Contains(marketplaceKindOrder, kind) {
		return kind, nil
	}
	return "", errors.Join(ErrMarketplaceNotFound, fmt.Errorf("unknown marketplace kind %q", kind))
}

func parseRefreshMarketplaceKind(raw string) (marketplacepkg.Kind, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case contract.MarketplaceKindMCP:
		return marketplacepkg.KindMCP, nil
	case contract.MarketplaceKindExtension:
		return marketplacepkg.KindExtension, nil
	case contract.MarketplaceKindSkill:
		return marketplacepkg.KindSkill, nil
	case contract.MarketplaceKindBundle:
		return "", marketplaceValidationf("bundle catalog is derived and cannot be refreshed")
	default:
		return "", marketplaceValidationf("unsupported refresh kind %q", strings.TrimSpace(raw))
	}
}

func marketplaceValidationf(format string, args ...any) error {
	return errors.Join(ErrMarketplaceValidation, fmt.Errorf(format, args...))
}

func normalizeSkillMarketplaceError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, skillmarketplace.ErrValidation):
		return errors.Join(ErrMarketplaceValidation, err)
	case errors.Is(err, skillmarketplace.ErrNotFound):
		return errors.Join(ErrMarketplaceNotFound, err)
	case errors.Is(err, skillmarketplace.ErrUnavailable), errors.Is(err, skillmarketplace.ErrNotConfigured):
		return errors.Join(ErrMarketplaceUnavailable, err)
	default:
		return err
	}
}

func (h *BaseHandlers) respondMarketplaceError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrMarketplaceValidation):
		status = http.StatusBadRequest
	case errors.Is(err, ErrMarketplaceNotFound), errors.Is(err, marketplacepkg.ErrEntryNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrMarketplaceUnavailable):
		status = http.StatusServiceUnavailable
	}
	h.respondError(c, status, err)
}
