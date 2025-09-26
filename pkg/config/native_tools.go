package config

import "time"

// NativeToolsConfig controls cp__ builtin enablement and sandbox settings.
type NativeToolsConfig struct {
	Enabled bool              `koanf:"enabled"  json:"enabled"  yaml:"enabled"  mapstructure:"enabled"`
	RootDir string            `koanf:"root_dir" json:"root_dir" yaml:"root_dir" mapstructure:"root_dir"`
	Exec    NativeExecConfig  `koanf:"exec"     json:"exec"     yaml:"exec"     mapstructure:"exec"`
	Fetch   NativeFetchConfig `koanf:"fetch"    json:"fetch"    yaml:"fetch"    mapstructure:"fetch"`
}

// NativeExecConfig holds cp__exec configuration knobs.
type NativeExecConfig struct {
	Timeout        time.Duration             `koanf:"timeout"          json:"timeout"          yaml:"timeout"          mapstructure:"timeout"`
	MaxStdoutBytes int64                     `koanf:"max_stdout_bytes" json:"max_stdout_bytes" yaml:"max_stdout_bytes" mapstructure:"max_stdout_bytes"`
	MaxStderrBytes int64                     `koanf:"max_stderr_bytes" json:"max_stderr_bytes" yaml:"max_stderr_bytes" mapstructure:"max_stderr_bytes"`
	Allowlist      []NativeExecCommandConfig `koanf:"allowlist"        json:"allowlist"        yaml:"allowlist"        mapstructure:"allowlist"`
}

// NativeExecCommandConfig defines per-command execution policies.
type NativeExecCommandConfig struct {
	Path            string                     `koanf:"path"             json:"path"             yaml:"path"             mapstructure:"path"`
	Description     string                     `koanf:"description"      json:"description"      yaml:"description"      mapstructure:"description"`
	Timeout         time.Duration              `koanf:"timeout"          json:"timeout"          yaml:"timeout"          mapstructure:"timeout"`
	MaxArgs         int                        `koanf:"max_args"         json:"max_args"         yaml:"max_args"         mapstructure:"max_args"`
	AllowAdditional bool                       `koanf:"allow_additional" json:"allow_additional" yaml:"allow_additional" mapstructure:"allow_additional"`
	Arguments       []NativeExecArgumentConfig `koanf:"arguments"        json:"arguments"        yaml:"arguments"        mapstructure:"arguments"`
}

// NativeExecArgumentConfig enforces validation for a single argument position.
type NativeExecArgumentConfig struct {
	Index    int      `koanf:"index"    json:"index"    yaml:"index"    mapstructure:"index"`
	Pattern  string   `koanf:"pattern"  json:"pattern"  yaml:"pattern"  mapstructure:"pattern"`
	Enum     []string `koanf:"enum"     json:"enum"     yaml:"enum"     mapstructure:"enum"`
	Optional bool     `koanf:"optional" json:"optional" yaml:"optional" mapstructure:"optional"`
}

// NativeFetchConfig holds cp__fetch configuration knobs.
type NativeFetchConfig struct {
	Timeout        time.Duration `koanf:"timeout"         json:"timeout"         yaml:"timeout"         mapstructure:"timeout"`
	MaxBodyBytes   int64         `koanf:"max_body_bytes"  json:"max_body_bytes"  yaml:"max_body_bytes"  mapstructure:"max_body_bytes"`
	MaxRedirects   int           `koanf:"max_redirects"   json:"max_redirects"   yaml:"max_redirects"   mapstructure:"max_redirects"`
	AllowedMethods []string      `koanf:"allowed_methods" json:"allowed_methods" yaml:"allowed_methods" mapstructure:"allowed_methods"`
}

// DefaultNativeToolsConfig returns safe defaults for native tool execution.
func DefaultNativeToolsConfig() NativeToolsConfig {
	return NativeToolsConfig{
		Enabled: true,
		RootDir: ".",
		Exec: NativeExecConfig{
			Timeout:        30 * time.Second,
			MaxStdoutBytes: 2 << 20, // 2 MiB
			MaxStderrBytes: 1 << 10, // 1 KiB
			Allowlist:      nil,
		},
		Fetch: NativeFetchConfig{
			Timeout:        5 * time.Second,
			MaxBodyBytes:   2 << 20, // 2 MiB
			MaxRedirects:   5,
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		},
	}
}
