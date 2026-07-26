package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestApplyMCPServerOverlaysNormalizesNameCollisions(t *testing.T) {
	t.Parallel()

	t.Run("Should merge trimmed overlay name into trimmed base name", func(t *testing.T) {
		t.Parallel()

		base := []MCPServer{{
			Name:    "  github  ",
			Command: "npx",
			Env: map[string]string{
				"BASE": "1",
			},
		}}
		name := "github"
		args := []string{"-y"}
		env := map[string]string{"WORKSPACE": "1"}

		merged := applyMCPServerOverlays(base, []mcpServerOverlay{{
			Name: &name,
			Args: &args,
			Env:  &env,
		}})

		if len(merged) != 1 {
			t.Fatalf("applyMCPServerOverlays() len = %d, want 1", len(merged))
		}
		if got, want := merged[0].Name, "github"; got != want {
			t.Fatalf("merged[0].Name = %q, want %q", got, want)
		}
		if got, want := merged[0].Command, "npx"; got != want {
			t.Fatalf("merged[0].Command = %q, want %q", got, want)
		}
		if got, want := strings.Join(merged[0].Args, ","), "-y"; got != want {
			t.Fatalf("merged[0].Args = %#v, want %q", merged[0].Args, want)
		}
		if got, want := merged[0].Env["BASE"], "1"; got != want {
			t.Fatalf("merged[0].Env[BASE] = %q, want %q", got, want)
		}
		if got, want := merged[0].Env["WORKSPACE"], "1"; got != want {
			t.Fatalf("merged[0].Env[WORKSPACE] = %q, want %q", got, want)
		}

		merged[0].Args[0] = "mutated"
		merged[0].Env["BASE"] = "mutated"
		if got, want := args[0], "-y"; got != want {
			t.Fatalf("overlay args alias changed = %q, want %q", got, want)
		}
		if got, want := base[0].Env["BASE"], "1"; got != want {
			t.Fatalf("base env alias changed = %q, want %q", got, want)
		}
	})
}

func TestMergeMCPServerLayersMergesCollisionsWithoutAliasingInputs(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve input isolation after collision merge", func(t *testing.T) {
		t.Parallel()

		base := []MCPServer{{
			Name:    "github",
			Command: "npx",
			Args:    []string{"base"},
			Env: map[string]string{
				"BASE": "1",
			},
			SecretEnv: map[string]string{
				"TOKEN": "env:BASE_TOKEN",
			},
			Auth: MCPAuthConfig{
				Scopes: []string{"base-scope"},
			},
		}}
		overlay := []MCPServer{{
			Name: "github",
			Args: []string{"overlay"},
			Env: map[string]string{
				"OVERLAY": "1",
			},
			SecretEnv: map[string]string{
				"TOKEN": "env:OVERLAY_TOKEN",
			},
			Auth: MCPAuthConfig{
				Type:   MCPAuthTypeOAuth2PKCE,
				Scopes: []string{"overlay-scope"},
			},
		}}

		merged := mergeMCPServerLayers(base, overlay)
		if len(merged) != 1 {
			t.Fatalf("mergeMCPServerLayers() len = %d, want 1", len(merged))
		}
		if got, want := merged[0].Command, "npx"; got != want {
			t.Fatalf("merged[0].Command = %q, want %q", got, want)
		}
		if got, want := strings.Join(merged[0].Args, ","), "overlay"; got != want {
			t.Fatalf("merged[0].Args = %#v, want %q", merged[0].Args, want)
		}
		if got, want := merged[0].Env["BASE"], "1"; got != want {
			t.Fatalf("merged[0].Env[BASE] = %q, want %q", got, want)
		}
		if got, want := merged[0].Env["OVERLAY"], "1"; got != want {
			t.Fatalf("merged[0].Env[OVERLAY] = %q, want %q", got, want)
		}
		if got, want := merged[0].SecretEnv["TOKEN"], "env:OVERLAY_TOKEN"; got != want {
			t.Fatalf("merged[0].SecretEnv[TOKEN] = %q, want %q", got, want)
		}
		if got, want := merged[0].Auth.Type, MCPAuthTypeOAuth2PKCE; got != want {
			t.Fatalf("merged[0].Auth.Type = %q, want %q", got, want)
		}
		if got, want := strings.Join(merged[0].Auth.Scopes, ","), "overlay-scope"; got != want {
			t.Fatalf("merged[0].Auth.Scopes = %#v, want %q", merged[0].Auth.Scopes, want)
		}

		merged[0].Args[0] = "mutated"
		merged[0].Env["BASE"] = "mutated"
		merged[0].SecretEnv["TOKEN"] = "mutated"
		merged[0].Auth.Scopes[0] = "mutated"
		if got, want := base[0].Args[0], "base"; got != want {
			t.Fatalf("base args alias changed = %q, want %q", got, want)
		}
		if got, want := base[0].Env["BASE"], "1"; got != want {
			t.Fatalf("base env alias changed = %q, want %q", got, want)
		}
		if got, want := base[0].SecretEnv["TOKEN"], "env:BASE_TOKEN"; got != want {
			t.Fatalf("base secret env alias changed = %q, want %q", got, want)
		}
		if got, want := base[0].Auth.Scopes[0], "base-scope"; got != want {
			t.Fatalf("base auth scopes alias changed = %q, want %q", got, want)
		}
		if got, want := overlay[0].Args[0], "overlay"; got != want {
			t.Fatalf("overlay args alias changed = %q, want %q", got, want)
		}
		if got, want := overlay[0].Auth.Scopes[0], "overlay-scope"; got != want {
			t.Fatalf("overlay auth scopes alias changed = %q, want %q", got, want)
		}
	})
}

func TestHookDeclarationsNormalizesWithoutAliasingInputs(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve order and input isolation", func(t *testing.T) {
		t.Parallel()

		enabled := true
		disabled := false
		toolReadOnly := true
		hooksCfg := HooksConfig{Declarations: []hookspkg.HookDecl{
			{
				Name:    "disabled",
				Event:   hookspkg.HookToolPreCall,
				Source:  hookspkg.HookSourceConfig,
				Enabled: &disabled,
				Command: "/bin/echo",
			},
			{
				Name:    "config-ready",
				Event:   hookspkg.HookToolPreCall,
				Source:  hookspkg.HookSourceConfig,
				Enabled: &enabled,
				Command: "/bin/echo",
				Args:    []string{"config"},
				Env:     map[string]string{"CONFIG": "1"},
				Matcher: hookspkg.HookMatcher{ToolReadOnly: &toolReadOnly},
			},
		}}
		agents := []AgentDef{{
			Name: "coder",
			Hooks: []hookspkg.HookDecl{{
				Name:    "agent-ready",
				Event:   hookspkg.HookToolPreCall,
				Source:  hookspkg.HookSourceAgentDefinition,
				Enabled: &enabled,
				Command: "/bin/echo",
				Args:    []string{"agent"},
				Env:     map[string]string{"AGENT": "1"},
			}},
		}}

		decls, err := HookDeclarations(hooksCfg, agents)
		if err != nil {
			t.Fatalf("HookDeclarations() error = %v", err)
		}
		if len(decls) != 2 {
			t.Fatalf("HookDeclarations() len = %d, want 2", len(decls))
		}
		if got, want := decls[0].Name, "config-ready"; got != want {
			t.Fatalf("decls[0].Name = %q, want %q", got, want)
		}
		if got, want := decls[1].Name, "agent-ready"; got != want {
			t.Fatalf("decls[1].Name = %q, want %q", got, want)
		}

		decls[0].Args[0] = "mutated"
		decls[0].Env["CONFIG"] = "mutated"
		*decls[0].Matcher.ToolReadOnly = false
		decls[1].Args[0] = "mutated"
		decls[1].Env["AGENT"] = "mutated"
		if got, want := hooksCfg.Declarations[1].Args[0], "config"; got != want {
			t.Fatalf("source config args alias changed = %q, want %q", got, want)
		}
		if got, want := hooksCfg.Declarations[1].Env["CONFIG"], "1"; got != want {
			t.Fatalf("source config env alias changed = %q, want %q", got, want)
		}
		if got, want := *hooksCfg.Declarations[1].Matcher.ToolReadOnly, true; got != want {
			t.Fatalf("source config matcher alias changed = %v, want %v", got, want)
		}
		if got, want := agents[0].Hooks[0].Args[0], "agent"; got != want {
			t.Fatalf("source agent args alias changed = %q, want %q", got, want)
		}
		if got, want := agents[0].Hooks[0].Env["AGENT"], "1"; got != want {
			t.Fatalf("source agent env alias changed = %q, want %q", got, want)
		}
	})
}

func TestEditAgentDefFileUsesPrivateAtomicWriter(t *testing.T) {
	t.Parallel()

	t.Run("Should rewrite agent definition with private mode", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "AGENT.md")
		writeFile(t, path, `---
name: coder
provider: claude
model: sonnet
---

Original prompt.
`)
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatalf("os.Chmod(agent file) error = %v", err)
		}

		agent, err := EditAgentDefFile(path, func(agent *AgentDef) error {
			agent.Model = "opus"
			agent.Prompt = "Updated prompt."
			return nil
		})
		if err != nil {
			t.Fatalf("EditAgentDefFile() error = %v", err)
		}
		if got, want := agent.Model, "opus"; got != want {
			t.Fatalf("EditAgentDefFile() Model = %q, want %q", got, want)
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(agent file) error = %v", err)
		}
		text := string(contents)
		if !strings.Contains(text, "model: opus") {
			t.Fatalf("agent file missing updated model:\n%s", text)
		}
		if !strings.Contains(text, "Updated prompt.") {
			t.Fatalf("agent file missing updated prompt:\n%s", text)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat(agent file) error = %v", err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
			t.Fatalf("agent file mode = %s, want %s", got, want)
		}
	})
}

func TestConfigSidecarReadsRejectSymlinks(t *testing.T) {
	t.Parallel()

	t.Run("Should reject dot env symlink without reading target", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink permissions vary on Windows")
		}

		dir := t.TempDir()
		targetPath := filepath.Join(dir, "actual.env")
		if err := os.WriteFile(targetPath, []byte("LEAKED_DOTENV_VALUE=secret\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(target .env) error = %v", err)
		}
		if err := os.Symlink(targetPath, filepath.Join(dir, ".env")); err != nil {
			t.Fatalf("os.Symlink(.env) error = %v", err)
		}

		_, err := loadDotEnvLookup(dir)
		if err == nil {
			t.Fatal("loadDotEnvLookup() error = nil, want symlink rejection")
		}
		if !errors.Is(err, ErrDotEnvUnsupported) {
			t.Fatalf("loadDotEnvLookup() error = %v, want ErrDotEnvUnsupported", err)
		}
		if strings.Contains(err.Error(), "LEAKED_DOTENV_VALUE") {
			t.Fatalf("loadDotEnvLookup() error leaked target content: %v", err)
		}
	})

	t.Run("Should reject MCP JSON symlink without reading target", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink permissions vary on Windows")
		}

		dir := t.TempDir()
		targetPath := filepath.Join(dir, "actual-mcp.json")
		payload := []byte(`{"mcpServers":{"leaked":{"command":"secret-command"}}}`)
		if err := os.WriteFile(targetPath, payload, 0o600); err != nil {
			t.Fatalf("os.WriteFile(target mcp.json) error = %v", err)
		}
		linkPath := filepath.Join(dir, MCPJSONName)
		if err := os.Symlink(targetPath, linkPath); err != nil {
			t.Fatalf("os.Symlink(mcp.json) error = %v", err)
		}

		_, err := LoadMCPServersJSONFile(linkPath)
		if err == nil {
			t.Fatal("LoadMCPServersJSONFile() error = nil, want symlink rejection")
		}
		if !strings.Contains(err.Error(), "not a symlink") {
			t.Fatalf("LoadMCPServersJSONFile() error = %v, want symlink context", err)
		}
		if strings.Contains(err.Error(), "secret-command") {
			t.Fatalf("LoadMCPServersJSONFile() error leaked target content: %v", err)
		}
	})

	t.Run("Should reject capability catalog symlink without reading target", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("symlink permissions vary on Windows")
		}

		agentDir := t.TempDir()
		targetPath := filepath.Join(agentDir, "actual-capabilities.toml")
		writeFile(t, targetPath, `
[[capabilities]]
id = "leaked-capability"
summary = "secret summary"
outcome = "secret outcome"
`)
		linkPath := filepath.Join(agentDir, capabilityCatalogTOMLName)
		if err := os.Symlink(targetPath, linkPath); err != nil {
			t.Fatalf("os.Symlink(capabilities.toml) error = %v", err)
		}

		_, err := LoadAgentCapabilities(agentDir)
		if err == nil {
			t.Fatal("LoadAgentCapabilities() error = nil, want symlink rejection")
		}
		if !strings.Contains(err.Error(), "not a symlink") {
			t.Fatalf("LoadAgentCapabilities() error = %v, want symlink context", err)
		}
		if strings.Contains(err.Error(), "secret summary") {
			t.Fatalf("LoadAgentCapabilities() error leaked target content: %v", err)
		}
	})
}
