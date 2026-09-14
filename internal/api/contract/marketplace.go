package contract

import (
	"encoding/json"
	"time"
)

const (
	MarketplaceScopeGlobal    = "global"
	MarketplaceScopeProfile   = "profile"
	MarketplaceScopeWorkspace = "workspace"
)

// MarketplaceListingPayload is one extension catalog discovery row.
type MarketplaceListingPayload struct {
	SourceRef        string                       `json:"source_ref,omitempty"`
	Icon             string                       `json:"icon,omitempty"`
	Layout           string                       `json:"layout,omitempty"`
	Installable      bool                         `json:"installable"`
	InstallBlocker   string                       `json:"install_blocker,omitempty"`
	DigestSHA256     string                       `json:"digest_sha256"`
	NameConflict     *MarketplaceOriginPayload    `json:"name_conflict,omitempty"`
	EntryID          string                       `json:"entry_id"`
	Name             string                       `json:"name"`
	Description      string                       `json:"description"`
	Version          string                       `json:"version,omitempty"`
	Author           string                       `json:"author,omitempty"`
	InstallSlug      string                       `json:"install_slug"`
	Source           string                       `json:"source"`
	Tier             string                       `json:"tier,omitempty"`
	Format           string                       `json:"format,omitempty"`
	PublishedAt      *time.Time                   `json:"published_at,omitempty"`
	UpdatedAt        *time.Time                   `json:"updated_at,omitempty"`
	Installed        bool                         `json:"installed"`
	InstalledName    string                       `json:"installed_name,omitempty"`
	InstalledVersion string                       `json:"installed_version,omitempty"`
	UpdateAvailable  bool                         `json:"update_available"`
	ManagePath       string                       `json:"manage_path,omitempty"`
	Trust            *ExtensionTrustReportPayload `json:"trust,omitempty"`
}

// MarketplaceInputBindingPayload identifies the sole allowed materialization target.
type MarketplaceInputBindingPayload struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// MarketplaceExtensionDetailPayload contains curated extension acquisition metadata.
type MarketplaceExtensionDetailPayload struct {
	ResolvedRef  string                     `json:"resolved_ref,omitempty"`
	Layout       string                     `json:"layout,omitempty"`
	Inputs       []MarketplaceInputPayload  `json:"inputs"`
	MCPServers   []MarketplaceServerPayload `json:"mcp_servers"`
	Contents     ExtensionContentsPayload   `json:"contents"`
	Diagnostics  []DiagnosticItem           `json:"diagnostics,omitempty"`
	InstallSlug  string                     `json:"install_slug"`
	ArtifactURL  string                     `json:"artifact_url,omitempty"`
	DigestSHA256 string                     `json:"digest_sha256"`
	Repository   string                     `json:"repository,omitempty"`
}

// MarketplaceEntryResponse is one exact detail resolved by entry_id.
type MarketplaceEntryResponse struct {
	Entry     MarketplaceListingPayload          `json:"entry"`
	Extension *MarketplaceExtensionDetailPayload `json:"extension,omitempty"`
}

// MarketplaceRefreshSourcePayload reports one feed-backed refresh outcome.
type MarketplaceRefreshSourcePayload struct {
	Source     string `json:"source"`
	Outcome    string `json:"outcome"`
	EntryCount int    `json:"entry_count"`
	Stale      bool   `json:"stale"`
	ErrorClass string `json:"error_class,omitempty"`
}

// MarketplaceRefreshResponse reports deterministic per-source refresh outcomes.
type MarketplaceRefreshResponse struct {
	Sources []MarketplaceRefreshSourcePayload `json:"sources"`
}

// MarketplaceOriginPayload identifies acquisition independently of its display name.
type MarketplaceOriginPayload struct {
	Source    string `json:"source"`
	SourceRef string `json:"source_ref"`
	EntryID   string `json:"entry_id"`
}

// ExtensionContentsPayload summarizes the resources shipped by one extension.
type ExtensionContentsPayload struct {
	Skills     int `json:"skills"`
	MCPServers int `json:"mcp_servers"`
	Hooks      int `json:"hooks"`
	Loops      int `json:"loops"`
	Agents     int `json:"agents"`
	Bridges    int `json:"bridges"`
}

// MarketplaceListResponse preserves the source and cursor metadata of the catalog page.
type MarketplaceListResponse struct {
	Refreshing bool                        `json:"refreshing,omitzero"`
	Total      int                         `json:"total"`
	NextCursor string                      `json:"next_cursor,omitempty"`
	Revision   string                      `json:"revision"`
	Stale      bool                        `json:"stale"`
	ErrorClass string                      `json:"error_class,omitempty"`
	Error      string                      `json:"error,omitempty"`
	Sources    []MarketplaceSourceSummary  `json:"sources"`
	Items      []MarketplaceListingPayload `json:"items"`
}

type MarketplaceSourceSummary struct {
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	State      string     `json:"state"`
	Count      int        `json:"count"`
	LastReadAt *time.Time `json:"last_read_at,omitempty"`
}

type MarketplaceCursorStalePayload struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Restart bool   `json:"restart"`
}

type MarketplaceInputPayload struct {
	ID       string                         `json:"id"`
	Prompt   string                         `json:"prompt"`
	Type     string                         `json:"type"`
	Required bool                           `json:"required"`
	Binding  MarketplaceInputBindingPayload `json:"binding"`
	Default  json.RawMessage                `json:"default,omitempty"`
}

// MCPAuthSummary excludes credentials and credential references.
type MCPAuthSummary struct {
	Method       string   `json:"method"`
	Registration string   `json:"registration,omitempty"`
	IssuerURL    string   `json:"issuer_url,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
}

type MarketplaceServerPayload struct {
	Name        string          `json:"name"`
	Owner       string          `json:"owner"`
	Scope       string          `json:"scope,omitempty"`
	Transport   string          `json:"transport"`
	Launch      string          `json:"launch"`
	Status      string          `json:"status,omitempty"`
	RuntimeName string          `json:"runtime_name,omitempty"`
	Auth        *MCPAuthSummary `json:"auth,omitempty"`
	Profile     string          `json:"profile,omitempty"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
}

type ExtensionInputStatePayload struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Set    bool   `json:"set"`
	Active bool   `json:"active"`
}
