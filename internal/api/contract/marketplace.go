package contract

import "time"

const (
	MarketplaceKindMCP       = "mcp"
	MarketplaceKindExtension = "extension"
	MarketplaceKindSkill     = "skill"
	MarketplaceKindBundle    = "bundle"

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

// MarketplaceMCPEnvFieldPayload describes one guided MCP environment input.
type MarketplaceMCPEnvFieldPayload struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt,omitempty"`
	Required bool   `json:"required"`
	Secret   bool   `json:"secret"`
	Default  string `json:"default,omitempty"`
}

// MarketplaceMCPOAuthPayload contains public OAuth client metadata from the catalog.
type MarketplaceMCPOAuthPayload struct {
	IssuerURL        string   `json:"issuer_url,omitempty"`
	AuthorizationURL string   `json:"authorization_url,omitempty"`
	TokenURL         string   `json:"token_url,omitempty"`
	ClientID         string   `json:"client_id"`
	Scopes           []string `json:"scopes,omitempty"`
}

// MarketplaceMCPDetailPayload contains the feed-locked MCP install template.
type MarketplaceMCPDetailPayload struct {
	Transport    string                          `json:"transport"`
	Command      string                          `json:"command,omitempty"`
	Args         []string                        `json:"args,omitempty"`
	URL          string                          `json:"url,omitempty"`
	OAuth        *MarketplaceMCPOAuthPayload     `json:"oauth,omitempty"`
	Env          []MarketplaceMCPEnvFieldPayload `json:"env,omitempty"`
	DefaultScope string                          `json:"default_scope,omitempty"`
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

// MarketplaceBundleProfilePayload summarizes one activatable bundle profile.
type MarketplaceBundleProfilePayload struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Layouts     int    `json:"layouts"`
	Agents      int    `json:"agents"`
	Jobs        int    `json:"jobs"`
	Triggers    int    `json:"triggers"`
	Bridges     int    `json:"bridges"`
	Channels    int    `json:"channels"`
}

// MarketplaceBundleDetailPayload contains the derived bundle catalog detail.
type MarketplaceBundleDetailPayload struct {
	ExtensionName string                            `json:"extension_name"`
	Profiles      []MarketplaceBundleProfilePayload `json:"profiles"`
}

// MarketplaceEntryResponse is one exact detail resolved by entry_id.
type MarketplaceEntryResponse struct {
	Entry     MarketplaceListingPayload          `json:"entry"`
	MCP       *MarketplaceMCPDetailPayload       `json:"mcp,omitempty"`
	Extension *MarketplaceExtensionDetailPayload `json:"extension,omitempty"`
	Skill     *MarketplaceSkillDetailPayload     `json:"skill,omitempty"`
	Bundle    *MarketplaceBundleDetailPayload    `json:"bundle,omitempty"`
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
