package e2e

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
	"github.com/goccy/go-yaml"
)

var socketPathCounter atomic.Uint64

// AgentSeed defines one persisted AGENT.md fixture.
type AgentSeed struct {
	Name         string
	Provider     string
	Command      string
	Model        string
	Permissions  string
	Tools        []string
	Toolsets     []string
	DenyTools    []string
	CategoryPath []string
	MCPServers   []aghconfig.MCPServer
	Prompt       string
}

// ConfigSeedOptions configures the seeded daemon runtime config.
type ConfigSeedOptions struct {
	Host            string
	HTTPPort        int
	SocketPath      string
	DefaultAgent    string
	DefaultProvider string
	DefaultSandbox  string
	PermissionMode  aghconfig.PermissionMode
	Providers       map[string]aghconfig.ProviderConfig
	Sandboxes       map[string]aghconfig.SandboxProfile
	AgentDefs       []AgentSeed
	Mutate          func(*aghconfig.Config)
}

// WorkspaceSeedOptions configures the seeded workspace root.
type WorkspaceSeedOptions struct {
	Root  string
	Files map[string]string
}

type configSeedFile struct {
	Daemon      *configSeedDaemonSection            `toml:"daemon,omitempty"`
	HTTP        *configSeedHTTPSection              `toml:"http,omitempty"`
	Defaults    *configSeedDefaultsSection          `toml:"defaults,omitempty"`
	Permissions *configSeedPermissionsSection       `toml:"permissions,omitempty"`
	Session     *aghconfig.SessionConfig            `toml:"session,omitempty"`
	Roles       *aghconfig.RolesConfig              `toml:"roles,omitempty"`
	Memory      *aghconfig.MemoryConfig             `toml:"memory,omitempty"`
	Network     *aghconfig.NetworkConfig            `toml:"network,omitempty"`
	Marketplace *aghconfig.MarketplaceRuntimeConfig `toml:"marketplace,omitempty"`
	Extensions  *configSeedExtensionsSection        `toml:"extensions,omitempty"`
	Providers   map[string]aghconfig.ProviderConfig `toml:"providers,omitempty"`
	Sandboxes   map[string]aghconfig.SandboxProfile `toml:"sandboxes,omitempty"`
}

type configSeedExtensionsSection struct {
	Marketplace *aghconfig.ExtensionsMarketplaceConfig `toml:"marketplace,omitempty"`
}

type configSeedDaemonSection struct {
	Socket string `toml:"socket"`
}

type configSeedHTTPSection struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type configSeedDefaultsSection struct {
	Agent    string `toml:"agent,omitempty"`
	Provider string `toml:"provider,omitempty"`
	Sandbox  string `toml:"sandbox,omitempty"`
}

type configSeedPermissionsSection struct {
	Mode aghconfig.PermissionMode `toml:"mode,omitempty"`
}

// NewHomePaths creates an isolated AGH home layout for one test run.
func NewHomePaths(t testing.TB) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

// SeedConfig writes a minimal config overlay and any requested agent definitions.
func SeedConfig(t testing.TB, homePaths aghconfig.HomePaths, opts ConfigSeedOptions) aghconfig.Config {
	t.Helper()

	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = defaultString(opts.Host, "127.0.0.1")
	if opts.HTTPPort > 0 {
		cfg.HTTP.Port = opts.HTTPPort
	} else {
		cfg.HTTP.Port = testutil.FreeTCPPort(t)
	}
	cfg.Daemon.Socket = defaultString(opts.SocketPath, shortSocketPath(t))
	if trimmed := strings.TrimSpace(opts.DefaultAgent); trimmed != "" {
		cfg.Defaults.Agent = trimmed
	}
	if trimmed := strings.TrimSpace(opts.DefaultProvider); trimmed != "" {
		cfg.Defaults.Provider = trimmed
	}
	if trimmed := strings.TrimSpace(opts.DefaultSandbox); trimmed != "" {
		cfg.Defaults.Sandbox = trimmed
	}
	if opts.PermissionMode != "" {
		cfg.Permissions.Mode = opts.PermissionMode
	}
	if len(opts.Providers) > 0 {
		cfg.Providers = cloneProviders(opts.Providers)
	}
	if len(opts.Sandboxes) > 0 {
		cfg.Sandboxes = cloneSandboxProfiles(opts.Sandboxes)
	}
	if opts.Mutate != nil {
		opts.Mutate(&cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("seed config validate error = %v", err)
	}
	if err := writeSeedConfigFile(homePaths, &cfg); err != nil {
		t.Fatalf("write seed config %q error = %v", homePaths.ConfigFile, err)
	}

	for _, agent := range opts.AgentDefs {
		WriteAgentDef(t, homePaths, agent)
	}

	return cfg
}

func writeSeedConfigFile(homePaths aghconfig.HomePaths, cfg *aghconfig.Config) error {
	if cfg == nil {
		return errors.New("seed config is required")
	}

	overlay := configSeedFile{
		Daemon: &configSeedDaemonSection{
			Socket: cfg.Daemon.Socket,
		},
		HTTP: &configSeedHTTPSection{
			Host: cfg.HTTP.Host,
			Port: cfg.HTTP.Port,
		},
		Defaults: &configSeedDefaultsSection{
			Agent:    cfg.Defaults.Agent,
			Provider: cfg.Defaults.Provider,
			Sandbox:  cfg.Defaults.Sandbox,
		},
		Session: cloneSessionConfig(cfg.Session),
		Roles:   cloneRolesConfig(&cfg.Roles),
		Memory:  cloneMemoryConfig(&cfg.Memory),
		Network: &cfg.Network,
		Marketplace: &aghconfig.MarketplaceRuntimeConfig{
			Catalog: cfg.Marketplace.Catalog,
		},
		Extensions: &configSeedExtensionsSection{
			Marketplace: &cfg.Extensions.Marketplace,
		},
		Providers: cloneProviders(cfg.Providers),
		Sandboxes: cloneSandboxProfiles(cfg.Sandboxes),
	}
	if cfg.Permissions.Mode != "" {
		overlay.Permissions = &configSeedPermissionsSection{Mode: cfg.Permissions.Mode}
	}

	file, err := os.Create(homePaths.ConfigFile)
	if err != nil {
		return fmt.Errorf("os.Create(%q): %w", homePaths.ConfigFile, err)
	}
	if err := toml.NewEncoder(file).Encode(overlay); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf(
				"toml encode config %q: %w (close error = %v)",
				homePaths.ConfigFile,
				err,
				closeErr,
			)
		}
		return fmt.Errorf("toml encode config %q: %w", homePaths.ConfigFile, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config %q: %w", homePaths.ConfigFile, err)
	}
	return nil
}

func cloneSessionConfig(cfg aghconfig.SessionConfig) *aghconfig.SessionConfig {
	cloned := cfg
	return &cloned
}

func cloneRolesConfig(cfg *aghconfig.RolesConfig) *aghconfig.RolesConfig {
	cloned := aghconfig.CloneRolesConfig(cfg)
	return &cloned
}

func cloneMemoryConfig(cfg *aghconfig.MemoryConfig) *aghconfig.MemoryConfig {
	cloned := aghconfig.CloneMemoryConfig(cfg)
	return &cloned
}

// SeedWorkspace creates an isolated workspace root and any requested files.
func SeedWorkspace(t testing.TB, opts WorkspaceSeedOptions) string {
	t.Helper()

	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = t.TempDir()
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", root, err)
	}

	for relativePath, contents := range opts.Files {
		targetPath, err := seedWorkspaceTargetPath(root, relativePath)
		if err != nil {
			t.Fatalf("seed workspace path %q error = %v", relativePath, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, []byte(contents), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", targetPath, err)
		}
	}

	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err == nil {
		return canonicalRoot
	}
	return root
}

func seedWorkspaceTargetPath(root string, relativePath string) (string, error) {
	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		return "", fmt.Errorf("workspace root is required")
	}
	cleanedRoot := filepath.Clean(trimmedRoot)

	targetPath := filepath.Clean(filepath.Join(cleanedRoot, relativePath))
	relativeTarget, err := filepath.Rel(cleanedRoot, targetPath)
	if err != nil {
		return "", fmt.Errorf("rel workspace target %q: %w", targetPath, err)
	}
	if relativeTarget == "." {
		return "", fmt.Errorf("workspace path %q must reference a file", relativePath)
	}
	if relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace path %q escapes root %q", relativePath, cleanedRoot)
	}

	return targetPath, nil
}

func cloneSandboxProfiles(
	profiles map[string]aghconfig.SandboxProfile,
) map[string]aghconfig.SandboxProfile {
	if len(profiles) == 0 {
		return nil
	}
	cloned := make(map[string]aghconfig.SandboxProfile, len(profiles))
	for name, profile := range profiles {
		next := profile
		next.Env = maps.Clone(profile.Env)
		next.Network.AllowList = append([]string(nil), profile.Network.AllowList...)
		next.Network.DenyList = append([]string(nil), profile.Network.DenyList...)
		cloned[name] = next
	}
	return cloned
}

// WriteAgentDef persists one AGENT.md fixture under the supplied home.
func WriteAgentDef(t testing.TB, homePaths aghconfig.HomePaths, seed AgentSeed) {
	t.Helper()

	name := strings.TrimSpace(seed.Name)
	if name == "" {
		t.Fatal("agent seed name is required")
	}

	path := filepath.Join(homePaths.AgentsDir, name, "AGENT.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}

	content, err := renderSeedAgentDef(seed)
	if err != nil {
		t.Fatalf("render seed agent def %q error = %v", name, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

type seedAgentDefFrontmatter struct {
	Name         string                 `yaml:"name"`
	Provider     string                 `yaml:"provider,omitempty"`
	Command      string                 `yaml:"command,omitempty"`
	Model        string                 `yaml:"model,omitempty"`
	Permissions  string                 `yaml:"permissions,omitempty"`
	Tools        []string               `yaml:"tools,omitempty"`
	Toolsets     []string               `yaml:"toolsets,omitempty"`
	DenyTools    []string               `yaml:"deny_tools,omitempty"`
	CategoryPath []string               `yaml:"category_path,omitempty"`
	MCPServers   []seedAgentMCPServerFM `yaml:"mcp_servers,omitempty"`
}

type seedAgentMCPServerFM struct {
	Name      string            `yaml:"name"`
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	SecretEnv map[string]string `yaml:"secret_env,omitempty"`
}

func renderSeedAgentDef(seed AgentSeed) (string, error) {
	name := strings.TrimSpace(seed.Name)
	if name == "" {
		return "", fmt.Errorf("agent seed name is required")
	}

	metadata := seedAgentDefFrontmatter{
		Name:         name,
		Provider:     strings.TrimSpace(seed.Provider),
		Command:      strings.TrimSpace(seed.Command),
		Model:        strings.TrimSpace(seed.Model),
		Permissions:  strings.TrimSpace(seed.Permissions),
		Tools:        trimSeedValues(seed.Tools),
		Toolsets:     trimSeedValues(seed.Toolsets),
		DenyTools:    trimSeedValues(seed.DenyTools),
		CategoryPath: trimSeedValues(seed.CategoryPath),
		MCPServers:   make([]seedAgentMCPServerFM, 0, len(seed.MCPServers)),
	}
	for _, server := range seed.MCPServers {
		metadata.MCPServers = append(metadata.MCPServers, seedAgentMCPServerFM{
			Name:      strings.TrimSpace(server.Name),
			Command:   strings.TrimSpace(server.Command),
			Args:      append([]string(nil), server.Args...),
			Env:       maps.Clone(server.Env),
			SecretEnv: maps.Clone(server.SecretEnv),
		})
	}

	frontmatterBytes, err := yaml.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal agent frontmatter: %w", err)
	}

	var builder strings.Builder
	builder.WriteString("---\n")
	builder.Write(frontmatterBytes)
	builder.WriteString("---\n\n")
	builder.WriteString(defaultString(seed.Prompt, "You are "+name+"."))
	builder.WriteString("\n")
	return builder.String(), nil
}

func trimSeedValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		trimmed = append(trimmed, strings.TrimSpace(value))
	}
	return trimmed
}

func shortSocketPath(t testing.TB) string {
	t.Helper()

	path := filepath.Join(
		os.TempDir(),
		fmt.Sprintf(
			"agh-e2e-%d-%d-%d.sock",
			os.Getpid(),
			time.Now().UTC().UnixNano(),
			socketPathCounter.Add(1),
		),
	)
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Logf("remove socket %q error = %v", path, err)
		}
	})
	return path
}

func cloneProviders(in map[string]aghconfig.ProviderConfig) map[string]aghconfig.ProviderConfig {
	if len(in) == 0 {
		return map[string]aghconfig.ProviderConfig{}
	}
	out := make(map[string]aghconfig.ProviderConfig, len(in))
	maps.Copy(out, in)
	return out
}

func defaultString(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func sortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
