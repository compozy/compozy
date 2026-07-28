package registry

import "io"

// PackageType distinguishes skills from extensions in registry results.
type PackageType string

const (
	PackageTypeSkill     PackageType = "skill"
	PackageTypeExtension PackageType = "extension"
	PackageTypeAll       PackageType = ""
)

// Listing is one registry search result.
type Listing struct {
	Slug        string      `json:"slug"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Author      string      `json:"author"`
	Version     string      `json:"version"`
	Downloads   int         `json:"downloads"`
	Source      string      `json:"source"`
	Type        PackageType `json:"type"`
}

// Detail is the full metadata for one registry package.
type Detail struct {
	Listing
	Readme     string   `json:"readme"`
	MCPServers []string `json:"mcp_servers,omitempty"`
	Tags       []string `json:"tags"`
	License    string   `json:"license"`
	Repository string   `json:"repository"`
	Versions   []string `json:"versions"`
}

// DownloadOpts controls version and asset selection for a download.
type DownloadOpts struct {
	Version        string
	Asset          string
	MaxArchiveSize int64
	ExpectedSHA256 string
}

// DownloadResult is the structured download response returned by a source.
type DownloadResult struct {
	Reader         io.ReadCloser
	Slug           string
	Version        string
	ContentSize    int64
	Checksum       string
	ContentType    string
	expectedSHA256 string
}

// SourceCaps declares which operations one registry source supports.
type SourceCaps struct {
	Search bool
}

// SearchOpts controls search pagination and package filtering.
type SearchOpts struct {
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
	Type   PackageType `json:"type"`
}

// UpdateInfo is the result of a local-vs-remote update check.
type UpdateInfo struct {
	Slug           string `json:"slug"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	Source         string `json:"source"`
}

// InstallResult is the outcome of a registry-backed install.
type InstallResult struct {
	Slug                string `json:"slug"`
	Name                string `json:"name"`
	Version             string `json:"version"`
	Source              string `json:"source"`
	InstallPath         string `json:"install_path"`
	Checksum            string `json:"checksum"`
	ArchiveDigestSHA256 string `json:"archive_digest_sha256"`
}
