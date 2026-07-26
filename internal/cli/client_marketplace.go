package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// MarketplaceReadScope selects the installed-state projection for Marketplace reads.
type MarketplaceReadScope struct {
	Scope       contract.SettingsWorkspaceScopeKind
	WorkspaceID string
}

// MarketplaceClient exposes curated discovery and refresh operations.
type MarketplaceClient interface {
	SearchMarketplace(
		ctx context.Context,
		query string,
		limit int,
		scope MarketplaceReadScope,
	) (MarketplaceSearchRecord, error)
	BrowseMarketplace(
		ctx context.Context,
		kind string,
		query string,
		limit int,
		cursor string,
		scope MarketplaceReadScope,
	) (MarketplaceKindRecord, error)
	MarketplaceInfo(
		ctx context.Context,
		kind string,
		entryID string,
		installedName string,
		scope MarketplaceReadScope,
	) (MarketplaceEntryRecord, error)
	RefreshMarketplace(ctx context.Context, kind string) (MarketplaceRefreshRecord, error)
}

func (c *unixSocketClient) SearchMarketplace(
	ctx context.Context,
	query string,
	limit int,
	scope MarketplaceReadScope,
) (MarketplaceSearchRecord, error) {
	values, err := scope.queryValues()
	if err != nil {
		return MarketplaceSearchRecord{}, err
	}
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		values.Set("q", trimmed)
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	var response MarketplaceSearchRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/marketplace/search", values, nil, &response); err != nil {
		return MarketplaceSearchRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) BrowseMarketplace(
	ctx context.Context,
	kind string,
	query string,
	limit int,
	cursor string,
	scope MarketplaceReadScope,
) (MarketplaceKindRecord, error) {
	trimmedKind := strings.TrimSpace(kind)
	if trimmedKind == "" {
		return MarketplaceKindRecord{}, errors.New("cli: marketplace kind is required")
	}
	values, err := scope.queryValues()
	if err != nil {
		return MarketplaceKindRecord{}, err
	}
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		values.Set("q", trimmed)
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	if trimmed := strings.TrimSpace(cursor); trimmed != "" {
		values.Set("cursor", trimmed)
	}
	var response MarketplaceKindRecord
	path := "/api/marketplace/" + url.PathEscape(trimmedKind)
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return MarketplaceKindRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) MarketplaceInfo(
	ctx context.Context,
	kind string,
	entryID string,
	installedName string,
	scope MarketplaceReadScope,
) (MarketplaceEntryRecord, error) {
	trimmedKind := strings.TrimSpace(kind)
	if trimmedKind == "" {
		return MarketplaceEntryRecord{}, errors.New("cli: marketplace kind is required")
	}
	trimmedEntryID := strings.TrimSpace(entryID)
	if trimmedEntryID == "" {
		return MarketplaceEntryRecord{}, errors.New("cli: marketplace entry ID is required")
	}
	values, err := scope.queryValues()
	if err != nil {
		return MarketplaceEntryRecord{}, err
	}
	if trimmed := strings.TrimSpace(installedName); trimmed != "" {
		switch trimmedKind {
		case contract.MarketplaceKindMCP, contract.MarketplaceKindExtension, contract.MarketplaceKindSkill:
			values.Set("installed_name", trimmed)
		default:
			return MarketplaceEntryRecord{}, errors.New(
				"cli: --installed-name is only supported for mcp, extension, or skill marketplace entries",
			)
		}
	}
	var response MarketplaceEntryRecord
	path := "/api/marketplace/" + url.PathEscape(trimmedKind) +
		"/" + url.PathEscape(trimmedEntryID)
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return MarketplaceEntryRecord{}, err
	}
	return response, nil
}

func (s MarketplaceReadScope) queryValues() (url.Values, error) {
	scope := contract.SettingsWorkspaceScopeKind(strings.TrimSpace(string(s.Scope)))
	workspaceID := strings.TrimSpace(s.WorkspaceID)
	values := url.Values{}
	switch scope {
	case contract.SettingsWorkspaceScopeGlobal:
		if workspaceID != "" {
			return nil, errors.New("cli: --workspace requires --scope workspace")
		}
	case contract.SettingsWorkspaceScopeWorkspace:
		if workspaceID == "" {
			return nil, errors.New("cli: --scope workspace requires --workspace")
		}
		values.Set("workspace_id", workspaceID)
	default:
		return nil, errors.New("cli: marketplace read scope must be global or workspace")
	}
	values.Set("scope", string(scope))
	return values, nil
}

func (c *unixSocketClient) RefreshMarketplace(
	ctx context.Context,
	kind string,
) (MarketplaceRefreshRecord, error) {
	values := url.Values{}
	if trimmed := strings.TrimSpace(kind); trimmed != "" {
		values.Set("kind", trimmed)
	}
	var response MarketplaceRefreshRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/marketplace/refresh", values, nil, &response); err != nil {
		return MarketplaceRefreshRecord{}, err
	}
	return response, nil
}
