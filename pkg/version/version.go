package version

// Build variables to be set via ldflags during compilation
// These variables are injected by GoReleaser with consistent paths:
// -X 'github.com/compozy/compozy/pkg/version.Version=v1.0.0'
// -X 'github.com/compozy/compozy/pkg/version.CommitHash=abc123'
// -X 'github.com/compozy/compozy/pkg/version.BuildDate=2024-01-01T00:00:00Z'
var (
	// Version is the semantic version of the binary (e.g., "1.0.0")
	Version = "unknown"
	// CommitHash is the git commit hash used to build the binary
	CommitHash = "unknown"
	// BuildDate is the timestamp when the binary was built (RFC3339 format)
	BuildDate = "unknown"
)

// Info returns build information in a structured format
type Info struct {
	Version    string `json:"version"`
	CommitHash string `json:"commit_hash"`
	BuildDate  string `json:"build_date"`
}

// Get returns the current build information
func Get() Info {
	return Info{
		Version:    Version,
		CommitHash: CommitHash,
		BuildDate:  BuildDate,
	}
}

// GetVersion returns just the version string
func GetVersion() string {
	return Version
}

// GetCommitHash returns just the commit hash
func GetCommitHash() string {
	return CommitHash
}

// GetBuildDate returns just the build date
func GetBuildDate() string {
	return BuildDate
}
