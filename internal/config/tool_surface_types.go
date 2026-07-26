package config

// ValueKind identifies the TOML scalar shape supported by tool writes.
type ValueKind uint8

const (
	ConfigValueString ValueKind = iota
	ConfigValueBool
	ConfigValueInt
	ConfigValueInt64
	ConfigValueFloat
	ConfigValueDuration
	ConfigValueStringSlice
	ConfigValueTable
)

// PathDenial is the config package's path-policy decision.
type PathDenial string

const (
	ConfigPathAllowed         PathDenial = ""
	ConfigPathForbidden       PathDenial = "path_forbidden"
	ConfigPathSecretForbidden PathDenial = "secret_path_forbidden"
	ConfigPathTrustForbidden  PathDenial = "trust_root_forbidden"
)

// PathPolicy captures the deterministic decision for an agent-facing config path.
type PathPolicy struct {
	Segments []string
	Kind     ValueKind
	Redacted bool
	Denial   PathDenial
}

// Entry is one flattened, redacted effective config value.
type Entry struct {
	Path     string `json:"path"`
	Value    any    `json:"value"`
	Redacted bool   `json:"redacted"`
}

// DiffEntry describes one redacted effective config difference.
type DiffEntry struct {
	Path           string `json:"path"`
	Before         any    `json:"before,omitempty"`
	After          any    `json:"after,omitempty"`
	BeforeRedacted bool   `json:"before_redacted,omitempty"`
	AfterRedacted  bool   `json:"after_redacted,omitempty"`
}
