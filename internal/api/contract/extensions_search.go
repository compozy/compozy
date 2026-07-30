package contract

// ExtensionSearchRequest selects global extension discovery sources.
type ExtensionSearchRequest struct {
	Query   string
	Sources []string
	Limit   int
	Cursor  string
}

// ExtensionSearchItem is one source-tagged extension discovery result.
type ExtensionSearchItem struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Author           string `json:"author,omitempty"`
	Version          string `json:"version,omitempty"`
	Downloads        int    `json:"downloads,omitempty"`
	Source           string `json:"source"`
	Tier             string `json:"tier"`
	Integrity        string `json:"integrity"`
	DigestMatched    bool   `json:"digest_matched"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version,omitempty"`
	UpdateAvailable  bool   `json:"update_available"`
}

// ExtensionSearchResponse is a stable page from a cached source-union snapshot.
type ExtensionSearchResponse struct {
	Items           []ExtensionSearchItem `json:"items"`
	NextCursor      string                `json:"next_cursor,omitempty"`
	SourcesDegraded []string              `json:"sources_degraded,omitempty"`
}
