//go:build integration

package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestEditConfigOverlayConcurrentUserAndProfileWritesIT044(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	userTarget, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget(user) error = %v", err)
	}
	profileTarget, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeProfile, "marketing")
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget(profile) error = %v", err)
	}

	start := make(chan struct{})
	errorsByScope := make(chan error, 2)
	var writers sync.WaitGroup
	for _, writer := range []struct {
		target WriteTarget
		value  string
	}{
		{target: userTarget, value: "claude"},
		{target: profileTarget, value: "codex"},
	} {
		writers.Go(func() {
			<-start
			_, writeErr := EditConfigOverlay(homePaths, "", writer.target, func(editor *OverlayEditor) error {
				return editor.SetValue([]string{"defaults", "provider"}, writer.value)
			})
			errorsByScope <- writeErr
		})
	}
	close(start)
	writers.Wait()
	close(errorsByScope)
	for writeErr := range errorsByScope {
		if writeErr != nil {
			t.Fatalf("EditConfigOverlay(concurrent layer) error = %v", writeErr)
		}
	}

	userPayload, err := os.ReadFile(userTarget.Path())
	if err != nil {
		t.Fatalf("ReadFile(user config) error = %v", err)
	}
	profilePayload, err := os.ReadFile(profileTarget.Path())
	if err != nil {
		t.Fatalf("ReadFile(profile config) error = %v", err)
	}
	if !strings.Contains(string(userPayload), `provider = "claude"`) {
		t.Fatalf("user config = %s, want claude", userPayload)
	}
	if !strings.Contains(string(profilePayload), `provider = "codex"`) {
		t.Fatalf("profile config = %s, want codex", profilePayload)
	}
	effective, err := LoadForHome(homePaths, WithProfile("marketing"))
	if err != nil {
		t.Fatalf("LoadForHome(marketing) error = %v", err)
	}
	if effective.Defaults.Provider != "codex" {
		t.Fatalf("effective defaults.provider = %q, want codex", effective.Defaults.Provider)
	}
}

func TestEditConfigOverlayGlobalWriteFromOperatorHomeWorkspace(t *testing.T) {
	t.Run("Should load global-only settings without treating them as a workspace overlay", func(t *testing.T) {
		t.Parallel()

		operatorHome := t.TempDir()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(operatorHome, DirName))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		target, err := ResolveConfigWriteTarget(homePaths, operatorHome, WriteScopeUser, "")
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
		}

		cfg, err := EditConfigOverlay(homePaths, operatorHome, target, func(editor *OverlayEditor) error {
			return editor.SetValue([]string{"gateway", "enabled"}, true)
		})
		if err != nil {
			t.Fatalf("EditConfigOverlay() error = %v", err)
		}
		if !cfg.Gateway.Enabled {
			t.Fatal("EditConfigOverlay() Gateway.Enabled = false, want true")
		}
	})
}

func TestEditConfigOverlayWorkspaceWriteLeavesGlobalConfigUntouched(t *testing.T) {
	t.Run("Should leave global config untouched during workspace writes", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		target, err := ResolveConfigWriteTarget(homePaths, workspaceRoot, WriteScopeWorkspace, "")
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
		}

		writeFile(t, homePaths.ConfigFile, `
[defaults]
agent = "global"
`)

		cfg, err := EditConfigOverlay(homePaths, workspaceRoot, target, func(editor *OverlayEditor) error {
			return editor.SetValue([]string{"defaults", "agent"}, "workspace")
		})
		if err != nil {
			t.Fatalf("EditConfigOverlay() error = %v", err)
		}
		if got, want := cfg.Defaults.Agent, "workspace"; got != want {
			t.Fatalf("Load merged Defaults.Agent = %q, want %q", got, want)
		}

		globalPayload, err := os.ReadFile(homePaths.ConfigFile)
		if err != nil {
			t.Fatalf("ReadFile(global config) error = %v", err)
		}
		if !strings.Contains(string(globalPayload), `agent = "global"`) {
			t.Fatalf("global config was modified unexpectedly\n%s", globalPayload)
		}

		workspaceConfig := filepath.Join(workspaceRoot, DirName, ConfigName)
		workspacePayload, err := os.ReadFile(workspaceConfig)
		if err != nil {
			t.Fatalf("ReadFile(workspace config) error = %v", err)
		}
		if !strings.Contains(string(workspacePayload), `agent = "workspace"`) {
			t.Fatalf("workspace config missing updated agent\n%s", workspacePayload)
		}

		globalOnly, err := LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome(global) error = %v", err)
		}
		if got, want := globalOnly.Defaults.Agent, "global"; got != want {
			t.Fatalf("global-only Defaults.Agent = %q, want %q", got, want)
		}

		assertPrivatePathMode(t, filepath.Dir(workspaceConfig), 0o700)
		assertPrivatePathMode(t, workspaceConfig, 0o600)
	})
}

func TestPutMCPSidecarServerRejectsDuplicateNamesAcrossTopLevelKeys(t *testing.T) {
	t.Run("Should reject duplicate MCP names across camel and snake collections", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		target, err := ResolveMCPSidecarWriteTarget(homePaths, "", WriteScopeUser, "")
		if err != nil {
			t.Fatalf("ResolveMCPSidecarWriteTarget() error = %v", err)
		}

		writeFile(t, target.path, `{
  "mcpServers": {
    "alpha": { "command": "camel" }
  },
  "mcp_servers": {
    " alpha ": { "command": "snake" }
  }
}`)

		_, err = PutMCPSidecarServer(homePaths, "", target, MCPServer{
			Name:    "beta",
			Command: "beta",
		})
		if err == nil ||
			!strings.Contains(err.Error(), `duplicate MCP server name "alpha" across top-level collections`) {
			t.Fatalf("PutMCPSidecarServer() error = %v, want cross-collection duplicate failure", err)
		}
	})
}
