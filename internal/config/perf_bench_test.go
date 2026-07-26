package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func BenchmarkLoadForHomeWorkspaceOverlay(b *testing.B) {
	homePaths, workspaceRoot := benchmarkLoadConfigFixture(b)

	b.ReportAllocs()

	for b.Loop() {
		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			b.Fatalf("LoadForHome() error = %v", err)
		}
		if cfg.Defaults.Agent == "" {
			b.Fatal("LoadForHome() returned empty default agent")
		}
	}
}

func BenchmarkResolveAgentMergedMCPServers(b *testing.B) {
	cfg := &Config{
		Defaults: DefaultsConfig{Provider: "claude"},
		Permissions: PermissionsConfig{
			Mode: PermissionModeApproveAll,
		},
		MCPServers: benchmarkMCPServers("global", 24, 0),
		Providers: map[string]ProviderConfig{
			"claude": {
				Command:    "npx -y @agentclientprotocol/claude-agent-acp@latest",
				Models:     ProviderModelsConfig{Default: "claude-sonnet-4-6"},
				MCPServers: benchmarkMCPServers("provider", 24, 8),
			},
		},
	}
	agent := AgentDef{
		Name:       "coder",
		Prompt:     "Ship it",
		MCPServers: benchmarkMCPServers("agent", 24, 12),
	}

	b.ReportAllocs()

	for b.Loop() {
		resolved, err := cfg.ResolveAgent(agent)
		if err != nil {
			b.Fatalf("ResolveAgent() error = %v", err)
		}
		if len(resolved.MCPServers) == 0 {
			b.Fatal("ResolveAgent() returned no MCP servers")
		}
	}
}

func BenchmarkParseMCPServersJSONLarge(b *testing.B) {
	content := []byte(benchmarkMCPJSONDocument(48))

	b.ReportAllocs()

	for b.Loop() {
		servers, err := ParseMCPServersJSON(content, "bench.json")
		if err != nil {
			b.Fatalf("ParseMCPServersJSON() error = %v", err)
		}
		if len(servers) != 48 {
			b.Fatalf("len(ParseMCPServersJSON()) = %d, want 48", len(servers))
		}
	}
}

func BenchmarkHookDeclarationsNormalization(b *testing.B) {
	hooksCfg, agents := benchmarkHookDeclarationsFixture()

	b.ReportAllocs()

	for b.Loop() {
		decls, err := HookDeclarations(hooksCfg, agents)
		if err != nil {
			b.Fatalf("HookDeclarations() error = %v", err)
		}
		if len(decls) == 0 {
			b.Fatal("HookDeclarations() returned no declarations")
		}
	}
}

func benchmarkLoadConfigFixture(b *testing.B) (HomePaths, string) {
	b.Helper()

	homeRoot := filepath.Join(b.TempDir(), "home")
	homePaths, err := ResolveHomePathsFrom(homeRoot)
	if err != nil {
		b.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		b.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	writeBenchmarkFile(b, homePaths.ConfigFile, `
[defaults]
agent = "researcher"
provider = "claude"

[[mcp_servers]]
name = "global"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-global"]
`)
	writeBenchmarkFile(b, filepath.Join(homePaths.HomeDir, MCPJSONName), benchmarkMCPJSONDocument(8))

	workspaceRoot := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(workspaceRoot, DirName, ConfigName), `
[defaults]
agent = "workspace-agent"

[[mcp_servers]]
name = "workspace"
command = "npx"
args = ["-y", "@modelcontextprotocol/server-workspace"]
`)
	writeBenchmarkFile(b, filepath.Join(workspaceRoot, DirName, MCPJSONName), benchmarkMCPJSONDocument(8))

	return homePaths, workspaceRoot
}

func benchmarkMCPServers(prefix string, count int, overlap int) []MCPServer {
	servers := make([]MCPServer, 0, count)
	for i := range count {
		name := prefix + "-" + strconv.Itoa(i)
		if i < overlap {
			name = "shared-" + strconv.Itoa(i)
		}
		servers = append(servers, MCPServer{
			Name:    name,
			Command: "npx",
			Args:    []string{"-y", prefix + "-server", "--slot", strconv.Itoa(i)},
			Env: map[string]string{
				"PREFIX": prefix,
				"SLOT":   strconv.Itoa(i),
			},
		})
	}
	return servers
}

func benchmarkMCPJSONDocument(count int) string {
	var builder strings.Builder
	builder.WriteString("{\"mcpServers\":{")
	for i := range count {
		if i > 0 {
			builder.WriteByte(',')
		}
		fmt.Fprintf(
			&builder,
			"%q:{\"command\":\"npx\",\"args\":[\"-y\",\"server-%d\"],\"env\":{\"SLOT\":\"%d\"},\"secret_env\":{\"TOKEN\":\"env:BENCH_TOKEN_%d\"}}",
			"server-"+strconv.Itoa(i),
			i,
			i,
			i,
		)
	}
	builder.WriteString("}}")
	return builder.String()
}

func benchmarkHookDeclarationsFixture() (HooksConfig, []AgentDef) {
	configDecls := make([]hookspkg.HookDecl, 0, 24)
	agentDecls := make([]hookspkg.HookDecl, 0, 24)

	for i := range 24 {
		configDecls = append(configDecls, hookspkg.HookDecl{
			Name:         "config-hook-" + strconv.Itoa(i),
			Event:        hookspkg.HookToolPreCall,
			Source:       hookspkg.HookSourceConfig,
			Mode:         hookspkg.HookModeSync,
			Timeout:      5 * time.Second,
			ExecutorKind: hookspkg.HookExecutorSubprocess,
			Command:      "echo",
			Args:         []string{"config", strconv.Itoa(i)},
			Env:          map[string]string{"HOOK_KIND": "config", "IDX": strconv.Itoa(i)},
		})
		agentDecls = append(agentDecls, hookspkg.HookDecl{
			Name:         "agent-hook-" + strconv.Itoa(i),
			Event:        hookspkg.HookToolPostCall,
			Source:       hookspkg.HookSourceAgentDefinition,
			Mode:         hookspkg.HookModeSync,
			Timeout:      5 * time.Second,
			Matcher:      hookspkg.HookMatcher{AgentName: "coder"},
			ExecutorKind: hookspkg.HookExecutorSubprocess,
			Command:      "echo",
			Args:         []string{"agent", strconv.Itoa(i)},
			Env:          map[string]string{"HOOK_KIND": "agent", "IDX": strconv.Itoa(i)},
		})
	}

	return HooksConfig{Declarations: configDecls}, []AgentDef{{
		Name:   "coder",
		Prompt: "Ship it",
		Hooks:  agentDecls,
	}}
}

func writeBenchmarkFile(b *testing.B, path string, contents string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimLeft(contents, "\n")), 0o644); err != nil {
		b.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
