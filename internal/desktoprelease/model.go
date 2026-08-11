package desktoprelease

const (
	ChannelBeta        = "beta"
	ChannelStable      = "stable"
	DistributionOrigin = "https://releases.compozy.com"

	platformDarwinAArch64 = "darwin-aarch64"
	platformDarwinX8664   = "darwin-x86_64"
	platformLinuxX8664    = "linux-x86_64"

	schemaStreamGlobal    = "global"
	schemaStreamMemory    = "memory"
	schemaStreamSession   = "session"
	schemaStreamWorkspace = "workspace"
)

// Windows is paused until Trusted Signing is restored; reintroduce its
// platform key together with the release.yml matrix entry.
var platformKeys = [...]string{
	platformDarwinAArch64,
	platformDarwinX8664,
	platformLinuxX8664,
}

type UpdaterPlatform struct {
	Signature string `json:"signature"`
	URL       string `json:"url"`
}

type LatestManifest struct {
	Version   string                     `json:"version"`
	Notes     string                     `json:"notes"`
	PubDate   string                     `json:"pub_date"`
	Platforms map[string]UpdaterPlatform `json:"platforms"`
}

type RuntimePlatform struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type RuntimeManifest struct {
	SchemaVersion int                        `json:"schema_version"`
	Version       string                     `json:"version"`
	Platforms     map[string]RuntimePlatform `json:"platforms"`
	SchemaHeads   map[string]uint64          `json:"schema_heads"`
}
