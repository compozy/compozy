package providerenv

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestApplyHomePolicyRejectsSymlinkedProviderDirsContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		linkRel string
		envKey  string
	}{
		{name: "Should reject symlinked provider home", linkRel: "", envKey: "HOME"},
		{name: "Should reject symlinked cache directory", linkRel: ".cache", envKey: "XDG_CACHE_HOME"},
		{name: "Should reject symlinked config directory", linkRel: ".config", envKey: "XDG_CONFIG_HOME"},
		{name: "Should reject symlinked local directory", linkRel: ".local", envKey: "XDG_DATA_HOME"},
		{name: "Should reject symlinked known provider directory", linkRel: "codex", envKey: "CODEX_HOME"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			home := t.TempDir()
			homePaths := aghconfig.HomePaths{HomeDir: home}
			providerHome := filepath.Join(home, "providers", "codex")
			if err := os.MkdirAll(filepath.Dir(filepath.Join(providerHome, tt.linkRel)), 0o700); err != nil {
				t.Fatalf("os.MkdirAll(provider parent) error = %v", err)
			}
			target := filepath.Join(t.TempDir(), "outside")
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Fatalf("os.Mkdir(target) error = %v", err)
			}
			before := statMode(t, target)
			createSymlinkOrSkip(t, target, filepath.Join(providerHome, tt.linkRel))

			env, err := ApplyHomePolicy(homePaths, "codex", aghconfig.ProviderHomePolicyIsolated, nil)
			if err == nil {
				t.Fatal("ApplyHomePolicy() error = nil, want symlink rejection")
			}
			if !errors.Is(err, errProviderDirSymlink) {
				t.Fatalf("ApplyHomePolicy() error = %v, want symlink rejection", err)
			}
			if got := providerEnvValue(env, tt.envKey); got != "" {
				t.Fatalf("%s = %q, want unset after rejection", tt.envKey, got)
			}
			if got := statMode(t, target); got != before {
				t.Fatalf("target mode = %v, want unchanged %v", got, before)
			}
		})
	}
}

func TestApplyPiAgentDirPolicyRejectsSymlinkedAgentDirContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject symlinked Pi agent directory", func(t *testing.T) {
		t.Parallel()

		home := t.TempDir()
		homePaths := aghconfig.HomePaths{HomeDir: home}
		providerHome := filepath.Join(home, "providers", "codex")
		if err := os.MkdirAll(filepath.Join(providerHome, ".pi"), 0o700); err != nil {
			t.Fatalf("os.MkdirAll(.pi) error = %v", err)
		}
		target := filepath.Join(t.TempDir(), "outside")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatalf("os.Mkdir(target) error = %v", err)
		}
		before := statMode(t, target)
		createSymlinkOrSkip(t, target, filepath.Join(providerHome, ".pi", "agent"))

		env, err := ApplyPiAgentDirPolicy(homePaths, "codex", aghconfig.ProviderHomePolicyIsolated, nil)
		if err == nil {
			t.Fatal("ApplyPiAgentDirPolicy() error = nil, want symlink rejection")
		}
		if !errors.Is(err, errProviderDirSymlink) {
			t.Fatalf("ApplyPiAgentDirPolicy() error = %v, want symlink rejection", err)
		}
		if got := providerEnvValue(env, "PI_CODING_AGENT_DIR"); got != "" {
			t.Fatalf("PI_CODING_AGENT_DIR = %q, want unset after rejection", got)
		}
		if got := statMode(t, target); got != before {
			t.Fatalf("target mode = %v, want unchanged %v", got, before)
		}
	})
}

func createSymlinkOrSkip(t *testing.T, target string, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
}

func statMode(t *testing.T, path string) os.FileMode {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}
	return info.Mode().Perm()
}

func providerEnvValue(env []string, key string) string {
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name == key {
			return value
		}
	}
	return ""
}
