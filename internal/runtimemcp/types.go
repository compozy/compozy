package runtimemcp

import "time"

type ReadContext struct {
	SystemRoot     string
	WorktreeRoot   string
	Home           string
	CWD            string
	GitRoot        string
	WorkspaceRoot  string
	Profile        string
	RuntimeProfile string
	Env            map[string]string
}

type Request struct {
	Context      ReadContext
	Runtime      string
	Modes        map[string]string
	CompozyNames []string
}

type RuntimeMode struct {
	Mode       string `json:"mode"`
	CanDisable bool   `json:"can_disable"`
}

type Server struct {
	ActivationDigest  string   `json:"-"`
	Runtime           string   `json:"runtime"`
	Name              string   `json:"name"`
	Scope             string   `json:"scope"`
	Transport         string   `json:"transport"`
	SourcePath        string   `json:"source_path"`
	SourceLabel       string   `json:"source_label"`
	EnabledInRuntime  bool     `json:"enabled_in_runtime"`
	Approval          string   `json:"approval"`
	Auth              string   `json:"auth"`
	EnvKeys           []string `json:"env_keys"`
	HeaderKeys        []string `json:"header_keys"`
	AlsoInCompozy     bool     `json:"also_in_compozy"`
	Copyable          bool     `json:"copyable"`
	CopyBlockedReason string   `json:"copy_blocked_reason"`
	DefinitionDigest  string   `json:"definition_digest"`
}

type Snapshot struct {
	Runtime          string      `json:"runtime"`
	DisplayName      string      `json:"display_name"`
	Installed        bool        `json:"installed"`
	RuntimeMCP       RuntimeMode `json:"runtime_mcp"`
	ReadState        string      `json:"read_state"`
	ReadError        string      `json:"read_error,omitempty"`
	ConfigFiles      []string    `json:"config_files"`
	Notes            []string    `json:"notes,omitempty"`
	Servers          []Server    `json:"servers"`
	ActivationDigest string      `json:"-"`
}

type Inventory struct {
	Runtimes    []Snapshot `json:"runtimes"`
	GeneratedAt time.Time  `json:"generated_at"`
}

type Descriptor struct {
	Runtime              string
	DisplayName          string
	Commands             []string
	CanDisableRuntimeMCP bool
	NativeHotReload      bool
}

func Descriptors() []Descriptor {
	return []Descriptor{
		{"claude", "Claude Code", []string{"claude"}, true, false},
		{"codex", "Codex", []string{"codex"}, false, false},
		{"gemini", "Gemini CLI", []string{"gemini"}, false, false},
		{"opencode", "OpenCode", []string{"opencode"}, false, true},
		{"kimi-cli", "Kimi CLI", []string{"kimi"}, false, false},
		{"qwen-code", "Qwen Code", []string{"qwen"}, true, false},
		{"copilot", "GitHub Copilot CLI", []string{"copilot"}, true, true},
		{"cursor", "Cursor Agent", []string{"cursor-agent", "agent"}, false, false},
		{"agy", "Antigravity", []string{"agy", "agy_acp_server.par"}, false, false},
		{"pi", "Pi", []string{"pi"}, false, false},
	}
}
