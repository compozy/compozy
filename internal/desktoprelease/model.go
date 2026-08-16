package desktoprelease

import "time"

const (
	ChannelBeta   = "beta"
	ChannelStable = "stable"

	ChannelBranchPrefix = "channel-"
	ChannelDirectory    = "desktop"

	ManifestMac       = "latest-mac.yml"
	ManifestLinux     = "latest-linux.yml"
	GenerationFile    = "generation.json"
	CompatibilityFile = "compat.json"
	ChecksumsFile     = "checksums.txt"

	operationPublish = "publish"
	operationRepair  = "repair"
)

type Compatibility struct {
	RuntimeVersion string `json:"runtime_version"`
	MinAppVersion  string `json:"min_app_version"`
}

type Artifact struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Generation struct {
	OperationID   string    `json:"operation_id"`
	Operation     string    `json:"operation"`
	Version       string    `json:"version"`
	MinAppVersion string    `json:"min_app_version"`
	PublishedAt   time.Time `json:"published_at"`
	SourceCommit  string    `json:"source_commit,omitempty"`
}

type OperatorResult struct {
	Operation         string     `json:"operation"`
	ChannelRefBefore  string     `json:"channel_ref_before"`
	ChannelRefAfter   string     `json:"channel_ref_after"`
	VerifiedInventory []Artifact `json:"verified_inventory"`
	AuditCommit       string     `json:"audit_commit"`
	Outcome           string     `json:"outcome"`
}
