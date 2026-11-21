package runtimemcp

import "time"

const (
	fieldHTTPHeaders = "http_headers"
	fieldType        = "type"
	fieldAuth        = "auth"

	fieldHTTPURL     = "httpUrl"
	fieldEnvironment = "environment"
	fieldCommand     = "command"
	fieldURL         = "url"
	runtimeQwenCode  = "qwen-code"

	fieldEnv              = "env"
	transportStdio        = "stdio"
	scopeManaged          = "managed"
	transportHTTP         = "http"
	fieldArgs             = "args"
	fieldHeader           = "header"
	fieldHeaders          = "headers"
	runtimeCodex          = "codex"
	runtimeOpenCode       = "opencode"
	scopeLocal            = "local"
	runtimeGemini         = "gemini"
	scopeUser             = "user"
	scopeProject          = "project"
	runtimeClaude         = "claude"
	runtimeQwen           = "qwen"
	runtimeCopilot        = "copilot"
	runtimeCursor         = "cursor"
	runtimeAntigravity    = "agy"
	scopePlugin           = "plugin"
	authOAuth             = "oauth"
	toolFiltersCopyReason = "uses tool filters, which CompozyOS servers can't keep"
)

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
	Target            string   `json:"target"`
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
		{runtimeClaude, "Claude Code", []string{runtimeClaude}, true, false},
		{runtimeCodex, "Codex", []string{runtimeCodex}, false, false},
		{runtimeGemini, "Gemini CLI", []string{runtimeGemini}, false, false},
		{runtimeOpenCode, "OpenCode", []string{runtimeOpenCode}, false, true},
		{"kimi-cli", "Kimi CLI", []string{"kimi"}, false, false},
		{runtimeQwenCode, "Qwen Code", []string{runtimeQwen}, true, false},
		{runtimeCopilot, "GitHub Copilot CLI", []string{runtimeCopilot}, true, true},
		{runtimeCursor, "Cursor Agent", []string{"cursor-agent", "agent"}, false, false},
		{runtimeAntigravity, "Antigravity", []string{runtimeAntigravity}, false, false},
		{"pi", "Pi", []string{"pi"}, false, false},
	}
}
