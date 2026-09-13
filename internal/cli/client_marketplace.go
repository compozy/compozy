package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

// MarketplaceReadScope selects the installed-state projection for Marketplace reads.
type MarketplaceReadScope struct {
	Scope       contract.SettingsLayeredScopeKind
	WorkspaceID string
	Profile     string
}

// MarketplaceClient exposes curated discovery and refresh operations.
type MarketplaceClient interface {
	SearchMarketplace(
		ctx context.Context,
		query string,
		limit int,
		cursor string,
		scope MarketplaceReadScope,
	) (MarketplaceListRecord, error)
	MarketplaceInfo(
		ctx context.Context,
		entryID string,
		source string,
		installedName string,
		scope MarketplaceReadScope,
	) (MarketplaceEntryRecord, error)
	RefreshMarketplace(ctx context.Context) (MarketplaceRefreshRecord, error)
}

func (c *daemonClient) SearchMarketplace(
	ctx context.Context,
	query string,
	limit int,
	cursor string,
	scope MarketplaceReadScope,
) (MarketplaceListRecord, error) {
	values, err := scope.queryValues()
	if err != nil {
		return MarketplaceListRecord{}, err
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
	var response MarketplaceListRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/marketplace", values, nil, &response); err != nil {
		return MarketplaceListRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) MarketplaceInfo(
	ctx context.Context,
	entryID string,
	source string,
	installedName string,
	scope MarketplaceReadScope,
) (MarketplaceEntryRecord, error) {
	trimmedEntryID := strings.TrimSpace(entryID)
	if trimmedEntryID == "" {
		return MarketplaceEntryRecord{}, errors.New("cli: marketplace entry ID is required")
	}
	values, err := scope.queryValues()
	if err != nil {
		return MarketplaceEntryRecord{}, err
	}
	if trimmed := strings.TrimSpace(installedName); trimmed != "" {
		values.Set("installed_name", trimmed)
	}
	if trimmed := strings.TrimSpace(source); trimmed != "" {
		values.Set("source", trimmed)
	}
	var response MarketplaceEntryRecord
	path := "/api/marketplace/entries/" + url.PathEscape(trimmedEntryID)
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return MarketplaceEntryRecord{}, err
	}
	return response, nil
}

func (s MarketplaceReadScope) queryValues() (url.Values, error) {
	scope := contract.SettingsLayeredScopeKind(strings.TrimSpace(string(s.Scope)))
	workspaceID := strings.TrimSpace(s.WorkspaceID)
	profile := strings.TrimSpace(s.Profile)
	values := url.Values{}
	switch scope {
	case contract.SettingsLayeredScopeUser:
		if workspaceID != "" {
			return nil, errors.New("cli: --workspace requires --scope workspace")
		}
		if profile != "" {
			return nil, errors.New("cli: --profile requires --scope profile")
		}
	case contract.SettingsLayeredScopeProfile:
		if workspaceID != "" {
			return nil, errors.New("cli: --workspace requires --scope workspace")
		}
		if profile == "" || profile == configDefaultKey {
			return nil, errors.New("cli: --scope profile requires an active non-default profile")
		}
		values.Set(profileFlagName, profile)
	case contract.SettingsLayeredScopeWorkspace:
		if workspaceID == "" {
			return nil, errors.New("cli: --scope workspace requires --workspace")
		}
		values.Set("workspace_id", workspaceID)
	default:
		return nil, errors.New("cli: marketplace read scope must be user, profile, or workspace")
	}
	wireScope := string(scope)
	if scope == contract.SettingsLayeredScopeUser {
		wireScope = contract.MarketplaceScopeGlobal
	}
	values.Set("scope", wireScope)
	return values, nil
}

func (c *daemonClient) RefreshMarketplace(
	ctx context.Context,
) (MarketplaceRefreshRecord, error) {
	values := url.Values{}
	var response MarketplaceRefreshRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/marketplace/refresh", values, nil, &response); err != nil {
		return MarketplaceRefreshRecord{}, err
	}
	return response, nil
}
