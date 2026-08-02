package contract

import "time"

const (
	MarketplaceKindMCP       = "mcp"
	MarketplaceKindExtension = "extension"
	MarketplaceKindSkill     = "skill"

	MarketplaceScopeGlobal    = "global"
	MarketplaceScopeWorkspace = "workspace"
)

// MarketplaceListingPayload is the common discovery row shared by every marketplace kind.
type MarketplaceListingPayload struct {
	Kind             string                       `json:"kind"`
	EntryID          string                       `json:"entry_id"`
	Name             string                       `json:"name"`
	Description      string                       `json:"description"`
	Version          string                       `json:"version,omitempty"`
	Author           string                       `json:"author,omitempty"`
	Downloads        *int                         `json:"downloads,omitempty"`
	InstallSlug      string                       `json:"install_slug,omitempty"`
	Source           string                       `json:"source"`
	Transport        string                       `json:"transport,omitempty"`
	Tier             string                       `json:"tier,omitempty"`
	PublishedAt      *time.Time                   `json:"published_at,omitempty"`
	UpdatedAt        *time.Time                   `json:"updated_at,omitempty"`
	Installed        bool                         `json:"installed"`
	InstalledName    string                       `json:"installed_name,omitempty"`
	InstalledVersion string                       `json:"installed_version,omitempty"`
	UpdateAvailable  bool                         `json:"update_available"`
	ManagePath       string                       `json:"manage_path,omitempty"`
	Trust            *ExtensionTrustReportPayload `json:"trust,omitempty"`
}

// MarketplaceKindResult is one independently resolved marketplace kind.
type MarketplaceKindResult struct {
	Kind       string                      `json:"kind"`
	Total      *int                        `json:"total,omitempty"`
	NextCursor string                      `json:"next_cursor,omitempty"`
	Stale      bool                        `json:"stale"`
	ErrorClass string                      `json:"error_class,omitempty"`
	Error      string                      `json:"error,omitempty"`
	Items      []MarketplaceListingPayload `json:"items"`
}

// MarketplaceSearchResponse is the deterministic grouped discovery response.
type MarketplaceSearchResponse struct {
	Query string                  `json:"query"`
	Kinds []MarketplaceKindResult `json:"kinds"`
}

// MarketplaceKindResponse is one kind's browse response.
type MarketplaceKindResponse struct {
	Kind       string                      `json:"kind"`
	Total      *int                        `json:"total,omitempty"`
	NextCursor string                      `json:"next_cursor,omitempty"`
	Stale      bool                        `json:"stale"`
	ErrorClass string                      `json:"error_class,omitempty"`
	Error      string                      `json:"error,omitempty"`
	Items      []MarketplaceListingPayload `json:"items"`
}

// MarketplaceMCPLaunchPayload contains the typed catalog launch declaration.
type MarketplaceMCPLaunchPayload struct {
	Type    string   `json:"type"`
	Package string   `json:"package,omitempty"`
	Version string   `json:"version,omitempty"`
	Image   string   `json:"image,omitempty"`
	Digest  string   `json:"digest,omitempty"`
	URL     string   `json:"url,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// MarketplaceMCPOAuthPayload contains OAuth discovery policy from the catalog.
type MarketplaceMCPOAuthPayload struct {
	Method       string   `json:"method"`
	Registration string   `json:"registration"`
	Scopes       []string `json:"scopes,omitempty"`
}

// MarketplaceMCPInputBindingPayload identifies the sole allowed materialization target.
type MarketplaceMCPInputBindingPayload struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// MarketplaceMCPInputPayload describes one typed operator input.
type MarketplaceMCPInputPayload struct {
	ID       string                            `json:"id"`
	Prompt   string                            `json:"prompt"`
	Type     string                            `json:"type"`
	Required bool                              `json:"required"`
	Binding  MarketplaceMCPInputBindingPayload `json:"binding"`
	Default  any                               `json:"default,omitempty"`
}

// MarketplaceMCPDetailPayload contains the feed-locked MCP install template.
type MarketplaceMCPDetailPayload struct {
	Launch       MarketplaceMCPLaunchPayload  `json:"launch"`
	Auth         *MarketplaceMCPOAuthPayload  `json:"auth,omitempty"`
	Inputs       []MarketplaceMCPInputPayload `json:"inputs,omitempty"`
	DefaultScope string                       `json:"default_scope"`
}

// MarketplaceExtensionDetailPayload contains curated extension acquisition metadata.
type MarketplaceExtensionDetailPayload struct {
	InstallSlug  string `json:"install_slug"`
	ArtifactURL  string `json:"artifact_url"`
	DigestSHA256 string `json:"digest_sha256"`
	Repository   string `json:"repository,omitempty"`
}

// MarketplaceSkillDetailPayload contains skill registry detail and acquisition metadata.
type MarketplaceSkillDetailPayload struct {
	InstallSlug string   `json:"install_slug"`
	DisplayName string   `json:"display_name,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Readme      string   `json:"readme,omitempty"`
	License     string   `json:"license,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	Versions    []string `json:"versions,omitempty"`
}

// MarketplaceEntryResponse is one exact detail resolved by entry_id.
type MarketplaceEntryResponse struct {
	Entry     MarketplaceListingPayload          `json:"entry"`
	MCP       *MarketplaceMCPDetailPayload       `json:"mcp,omitempty"`
	Extension *MarketplaceExtensionDetailPayload `json:"extension,omitempty"`
	Skill     *MarketplaceSkillDetailPayload     `json:"skill,omitempty"`
}

// MarketplaceRefreshKindPayload reports one feed-backed refresh outcome.
type MarketplaceRefreshKindPayload struct {
	Kind       string `json:"kind"`
	Outcome    string `json:"outcome"`
	EntryCount int    `json:"entry_count"`
	Stale      bool   `json:"stale"`
	ErrorClass string `json:"error_class,omitempty"`
}

// MarketplaceRefreshResponse reports deterministic per-kind refresh outcomes.
type MarketplaceRefreshResponse struct {
	Kinds []MarketplaceRefreshKindPayload `json:"kinds"`
}
