package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/procutil"
)

const (
	hermeticTimezone = "UTC"
	hermeticLocale   = "C.UTF-8"
)

// HermeticEnv captures deterministic environment paths applied by ApplyHermeticEnv.
type HermeticEnv struct {
	HomeDir          string
	ConfigHomeDir    string
	ProviderHomeDir  string
	ProviderCodexDir string
	ClaudeConfigDir  string
	CodexHomeDir     string
	OpenCodeHomeDir  string
}

// ApplyHermeticEnv scrubs ambient credentials and pins deterministic env values
// for tests that intentionally exercise process environment, AGH_HOME, provider
// home, or release shell behavior. It mutates process environment and therefore
// must not be used by tests that call t.Parallel.
func ApplyHermeticEnv(t testing.TB) HermeticEnv {
	t.Helper()

	restorer := newEnvRestorer(t)
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if shouldClearHermeticEnvName(name) {
			restorer.unset(t, name)
		}
	}

	state := HermeticEnv{
		HomeDir:          filepath.Join(t.TempDir(), "agh-home"),
		ConfigHomeDir:    filepath.Join(t.TempDir(), "agh-config-home"),
		ProviderHomeDir:  filepath.Join(t.TempDir(), "provider-home"),
		ProviderCodexDir: filepath.Join(t.TempDir(), "provider-codex-home"),
		ClaudeConfigDir:  filepath.Join(t.TempDir(), "claude-config"),
		CodexHomeDir:     filepath.Join(t.TempDir(), "codex-home"),
		OpenCodeHomeDir:  filepath.Join(t.TempDir(), "opencode-config"),
	}

	restorer.set(t, "AGH_HOME", state.HomeDir)
	restorer.set(t, "AGH_CONFIG_HOME", state.ConfigHomeDir)
	restorer.set(t, "PROVIDER_HOME", state.ProviderHomeDir)
	restorer.set(t, "PROVIDER_CODEX_HOME", state.ProviderCodexDir)
	restorer.set(t, "CLAUDE_CONFIG_DIR", state.ClaudeConfigDir)
	restorer.set(t, "CODEX_HOME", state.CodexHomeDir)
	restorer.set(t, "OPENCODE_CONFIG_DIR", state.OpenCodeHomeDir)
	restorer.set(t, "TZ", hermeticTimezone)
	restorer.set(t, "LANG", hermeticLocale)
	restorer.set(t, "LC_ALL", hermeticLocale)
	restorer.set(t, "LC_CTYPE", hermeticLocale)
	return state
}

// HermeticProcessEnv returns a child-process environment with credential-shaped
// and AGH/provider-local state removed, plus deterministic timezone and locale
// pins. It intentionally leaves HOME untouched; tests that need isolated AGH
// state should set AGH_HOME explicitly after calling this helper.
func HermeticProcessEnv(base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	env := make([]string, 0, len(base)+4)
	for _, entry := range base {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || shouldClearHermeticEnvName(name) {
			continue
		}
		env = append(env, entry)
	}
	env = setEnvEntry(env, "TZ", hermeticTimezone)
	env = setEnvEntry(env, "LANG", hermeticLocale)
	env = setEnvEntry(env, "LC_ALL", hermeticLocale)
	env = setEnvEntry(env, "LC_CTYPE", hermeticLocale)
	return env
}

type envRestorer struct {
	values map[string]envValue
}

type envValue struct {
	value string
	ok    bool
}

func newEnvRestorer(t testing.TB) *envRestorer {
	t.Helper()

	restorer := &envRestorer{values: make(map[string]envValue)}
	t.Cleanup(func() {
		restorer.restore(t)
	})
	return restorer
}

func (r *envRestorer) set(t testing.TB, key string, value string) {
	t.Helper()

	r.remember(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Setenv(%q) error = %v", key, err)
	}
}

func (r *envRestorer) unset(t testing.TB, key string) {
	t.Helper()

	r.remember(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%q) error = %v", key, err)
	}
}

func (r *envRestorer) remember(key string) {
	if _, ok := r.values[key]; ok {
		return
	}
	value, ok := os.LookupEnv(key)
	r.values[key] = envValue{value: value, ok: ok}
}

func (r *envRestorer) restore(t testing.TB) {
	t.Helper()

	for key, value := range r.values {
		var err error
		if value.ok {
			err = os.Setenv(key, value.value)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restore env %q error = %v", key, err)
		}
	}
}

func shouldClearHermeticEnvName(name string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(name))
	if normalized == "" {
		return false
	}
	if strings.HasPrefix(normalized, "AGH_TEST_") {
		return false
	}
	if procutil.SensitiveEnvName(normalized) {
		return true
	}
	if strings.HasPrefix(normalized, "AGH_") {
		return true
	}
	switch normalized {
	case "PROVIDER_HOME",
		"PROVIDER_CODEX_HOME",
		"CLAUDE_CONFIG_DIR",
		"CODEX_HOME",
		"OPENCODE_CONFIG_DIR",
		"PI_CODING_AGENT_DIR",
		"AWS_PROFILE",
		"AWS_CONFIG_FILE",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"KUBECONFIG",
		"NETRC",
		"DOCKER_CONFIG",
		"NPM_CONFIG_USERCONFIG":
		return true
	default:
		return hasHermeticProviderPrefix(normalized)
	}
}

func hasHermeticProviderPrefix(name string) bool {
	for _, prefix := range []string{
		"ANTHROPIC_",
		"AWS_",
		"AZURE_",
		"BRAVE_",
		"CLAUDE_",
		"CODEX_",
		"DISCORD_",
		"EXA_",
		"GCP_",
		"GEMINI_",
		"GH_",
		"GITHUB_",
		"GOOGLE_",
		"GROQ_",
		"KIMI_",
		"LINEAR_",
		"MCP_",
		"MINIMAX_",
		"MISTRAL_",
		"MOONSHOT_",
		"NOTION_",
		"NPM_",
		"OPENCODE_",
		"OPENAI_",
		"OPENROUTER_",
		"SERPAPI_",
		"SLACK_",
		"TAVILY_",
		"VERCEL_",
		"VERTEX_",
		"XAI_",
		"ZAI_",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func setEnvEntry(env []string, key string, value string) []string {
	prefix := key + "="
	filtered := env[:0]
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, prefix+value)
}
