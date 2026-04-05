package agent

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestAgentRegistryEntries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		ide        string
		reasoning  string
		addDirs    []string
		accessMode string
		wantLaunch []string
		wantProbe  []string
	}{
		{
			name:       "claude",
			ide:        model.IDEClaude,
			reasoning:  "medium",
			addDirs:    []string{"../shared", "../docs"},
			accessMode: model.AccessModeFull,
			wantLaunch: []string{"claude-agent-acp"},
			wantProbe:  []string{"claude-agent-acp", "--help"},
		},
		{
			name:       "codex",
			ide:        model.IDECodex,
			reasoning:  "medium",
			addDirs:    []string{"../shared", "../docs"},
			accessMode: model.AccessModeFull,
			wantLaunch: []string{
				"codex-acp",
				"-c",
				`approval_policy="never"`,
				"-c",
				`sandbox_mode="danger-full-access"`,
				"-c",
				`web_search="live"`,
			},
			wantProbe: []string{"codex-acp", "--help"},
		},
		{
			name:       "droid",
			ide:        model.IDEDroid,
			reasoning:  "medium",
			accessMode: model.AccessModeFull,
			wantLaunch: []string{
				"droid",
				"exec",
				"--output-format",
				"acp",
				"--skip-permissions-unsafe",
				"--model",
				model.DefaultCodexModel,
				"--reasoning-effort",
				"medium",
			},
			wantProbe: []string{"droid", "exec", "--help"},
		},
		{
			name:       "cursor",
			ide:        model.IDECursor,
			reasoning:  "medium",
			accessMode: model.AccessModeFull,
			wantLaunch: []string{"cursor-agent", "acp"},
			wantProbe:  []string{"cursor-agent", "acp", "--help"},
		},
		{
			name:       "opencode",
			ide:        model.IDEOpenCode,
			reasoning:  "medium",
			accessMode: model.AccessModeFull,
			wantLaunch: []string{"opencode", "acp"},
			wantProbe:  []string{"opencode", "acp", "--help"},
		},
		{
			name:       "pi",
			ide:        model.IDEPi,
			reasoning:  "medium",
			accessMode: model.AccessModeFull,
			wantLaunch: []string{"pi-acp"},
			wantProbe:  []string{"pi-acp", "--help"},
		},
		{
			name:       "gemini",
			ide:        model.IDEGemini,
			reasoning:  "medium",
			accessMode: model.AccessModeFull,
			wantLaunch: []string{"gemini", "--acp"},
			wantProbe:  []string{"gemini", "--acp", "--help"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec, err := lookupAgentSpec(tc.ide)
			if err != nil {
				t.Fatalf("lookup agent spec: %v", err)
			}

			gotLaunch := spec.launchCommand(resolveModel(spec, ""), tc.reasoning, tc.addDirs, tc.accessMode)
			if !slices.Equal(gotLaunch, tc.wantLaunch) {
				t.Fatalf("unexpected launch command for %s: got %v want %v", tc.ide, gotLaunch, tc.wantLaunch)
			}
			if gotProbe := spec.probeCommand(); !slices.Equal(gotProbe, tc.wantProbe) {
				t.Fatalf("unexpected probe command for %s: got %v want %v", tc.ide, gotProbe, tc.wantProbe)
			}
		})
	}
}

func TestBuildShellCommandStringUsesFallbackLauncherWhenPrimaryMissing(t *testing.T) {
	tmpDir := t.TempDir()
	npxPath := filepath.Join(tmpDir, "npx")
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(npxPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake npx: %v", err)
	}

	t.Setenv("PATH", tmpDir)
	registerTestSpec(t, Spec{
		ID:           "fallback-shell-test",
		DisplayName:  "Fallback Shell",
		DefaultModel: "test-model",
		Command:      "missing-acp",
		Fallbacks: []Launcher{
			{
				Command:   "npx",
				FixedArgs: []string{"--yes", "@scope/test-acp"},
			},
		},
	})

	got := BuildShellCommandString("fallback-shell-test", "", nil, "medium", model.AccessModeFull)
	if got != `npx --yes @scope/test-acp` {
		t.Fatalf("unexpected shell command: %s", got)
	}
}

func TestResolveLaunchCommandUsesFallbackCandidate(t *testing.T) {
	tmpDir := t.TempDir()
	npxPath := filepath.Join(tmpDir, "npx")
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(npxPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake npx: %v", err)
	}

	t.Setenv("PATH", tmpDir)
	registerTestSpec(t, Spec{
		ID:           "fallback-launch-test",
		DisplayName:  "Fallback Launch",
		DefaultModel: "test-model",
		Command:      "missing-acp",
		Fallbacks: []Launcher{
			{
				Command:   "npx",
				FixedArgs: []string{"--yes", "@scope/test-acp"},
			},
		},
	})

	spec, err := lookupAgentSpec("fallback-launch-test")
	if err != nil {
		t.Fatalf("lookup test spec: %v", err)
	}

	command, err := resolveLaunchCommand(spec, spec.DefaultModel, "medium", nil, model.AccessModeDefault, true)
	if err != nil {
		t.Fatalf("resolve launch command: %v", err)
	}
	if want := []string{"npx", "--yes", "@scope/test-acp"}; !slices.Equal(command, want) {
		t.Fatalf("unexpected fallback command: got %v want %v", command, want)
	}
}

func TestLookupAgentSpecUnknownIDE(t *testing.T) {
	t.Parallel()

	if _, err := lookupAgentSpec("unknown-ide"); err == nil {
		t.Fatal("expected lookup error for unknown ide")
	}
}

func TestValidateRuntimeConfigAcceptsSupportedIDEs(t *testing.T) {
	t.Parallel()

	validIDEs := []string{
		model.IDEClaude,
		model.IDECodex,
		model.IDEDroid,
		model.IDECursor,
		model.IDEOpenCode,
		model.IDEPi,
		model.IDEGemini,
	}

	for _, ide := range validIDEs {
		ide := ide
		t.Run(ide, func(t *testing.T) {
			t.Parallel()

			cfg := &model.RuntimeConfig{
				Mode:                   model.ExecutionModePRReview,
				IDE:                    ide,
				OutputFormat:           model.OutputFormatText,
				BatchSize:              1,
				MaxRetries:             1,
				RetryBackoffMultiplier: 1.5,
			}
			if err := ValidateRuntimeConfig(cfg); err != nil {
				t.Fatalf("validate runtime config: %v", err)
			}
		})
	}
}

func TestValidateRuntimeConfigRejectsPRDTaskBatching(t *testing.T) {
	t.Parallel()

	cfg := &model.RuntimeConfig{
		Mode:         model.ExecutionModePRDTasks,
		IDE:          model.IDECodex,
		OutputFormat: model.OutputFormatText,
		BatchSize:    2,
	}

	err := ValidateRuntimeConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRuntimeConfigRejectsInvalidRetryConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  *model.RuntimeConfig
	}{
		{
			name: "negative retries",
			cfg: &model.RuntimeConfig{
				Mode:                   model.ExecutionModePRReview,
				IDE:                    model.IDECodex,
				OutputFormat:           model.OutputFormatText,
				BatchSize:              1,
				MaxRetries:             -1,
				RetryBackoffMultiplier: 1.5,
			},
		},
		{
			name: "non positive multiplier",
			cfg: &model.RuntimeConfig{
				Mode:                   model.ExecutionModePRReview,
				IDE:                    model.IDECodex,
				OutputFormat:           model.OutputFormatText,
				BatchSize:              1,
				MaxRetries:             1,
				RetryBackoffMultiplier: 0,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateRuntimeConfig(tc.cfg); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateRuntimeConfigRejectsInvalidAccessMode(t *testing.T) {
	t.Parallel()

	cfg := &model.RuntimeConfig{
		Mode:                   model.ExecutionModePRReview,
		IDE:                    model.IDECodex,
		OutputFormat:           model.OutputFormatText,
		BatchSize:              1,
		AccessMode:             "invalid",
		MaxRetries:             0,
		RetryBackoffMultiplier: 1.5,
	}

	err := ValidateRuntimeConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "--access-mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRuntimeConfigAcceptsExecModeWithSinglePromptSource(t *testing.T) {
	t.Parallel()

	cfg := &model.RuntimeConfig{
		Mode:                   model.ExecutionModeExec,
		IDE:                    model.IDECodex,
		OutputFormat:           model.OutputFormatJSON,
		PromptFile:             "prompt.md",
		BatchSize:              1,
		MaxRetries:             1,
		RetryBackoffMultiplier: 1.5,
	}

	if err := ValidateRuntimeConfig(cfg); err != nil {
		t.Fatalf("validate exec runtime config: %v", err)
	}
}

func TestValidateRuntimeConfigRejectsInvalidExecCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *model.RuntimeConfig
		wantErr string
	}{
		{
			name: "missing prompt source",
			cfg: &model.RuntimeConfig{
				Mode:                   model.ExecutionModeExec,
				IDE:                    model.IDECodex,
				OutputFormat:           model.OutputFormatText,
				BatchSize:              1,
				MaxRetries:             1,
				RetryBackoffMultiplier: 1.5,
			},
			wantErr: "requires exactly one prompt source",
		},
		{
			name: "multiple prompt sources",
			cfg: &model.RuntimeConfig{
				Mode:                   model.ExecutionModeExec,
				IDE:                    model.IDECodex,
				OutputFormat:           model.OutputFormatText,
				PromptText:             "hello",
				PromptFile:             "prompt.md",
				BatchSize:              1,
				MaxRetries:             1,
				RetryBackoffMultiplier: 1.5,
			},
			wantErr: "accepts only one prompt source",
		},
		{
			name: "unsupported output format",
			cfg: &model.RuntimeConfig{
				Mode:                   model.ExecutionModeExec,
				IDE:                    model.IDECodex,
				OutputFormat:           model.OutputFormat("yaml"),
				PromptText:             "hello",
				BatchSize:              1,
				MaxRetries:             1,
				RetryBackoffMultiplier: 1.5,
			},
			wantErr: "invalid output format",
		},
		{
			name: "prompt source outside exec mode",
			cfg: &model.RuntimeConfig{
				Mode:                   model.ExecutionModePRReview,
				IDE:                    model.IDECodex,
				OutputFormat:           model.OutputFormatText,
				PromptText:             "hello",
				BatchSize:              1,
				MaxRetries:             1,
				RetryBackoffMultiplier: 1.5,
			},
			wantErr: "prompt source fields are only supported for exec mode",
		},
		{
			name: "json format outside exec mode",
			cfg: &model.RuntimeConfig{
				Mode:                   model.ExecutionModePRReview,
				IDE:                    model.IDECodex,
				OutputFormat:           model.OutputFormatJSON,
				BatchSize:              1,
				MaxRetries:             1,
				RetryBackoffMultiplier: 1.5,
			},
			wantErr: "only supported for exec mode",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRuntimeConfig(tt.cfg)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("unexpected error\nwant substring: %q\ngot: %v", tt.wantErr, err)
			}
		})
	}
}

func TestEnsureAvailableReturnsTypedErrorWhenCommandMissing(t *testing.T) {
	testSpec := Spec{
		ID:           "missing-binary-test",
		DisplayName:  "Missing",
		DefaultModel: "test-model",
		Command:      "definitely-not-installed-binary",
		DocsURL:      "https://example.com/docs",
		InstallHint:  "Install the missing ACP adapter.",
	}
	registerTestSpec(t, testSpec)

	err := EnsureAvailable(&model.RuntimeConfig{IDE: testSpec.ID})
	if err == nil {
		t.Fatal("expected EnsureAvailable error")
	}

	var availabilityErr *AvailabilityError
	if !errors.As(err, &availabilityErr) {
		t.Fatalf("expected AvailabilityError, got %T", err)
	}
	if !strings.Contains(err.Error(), `tried definitely-not-installed-binary`) {
		t.Fatalf("expected attempted command in error, got %q", err)
	}
	if !strings.Contains(err.Error(), testSpec.InstallHint) {
		t.Fatalf("expected install hint in error, got %q", err)
	}

	if err := EnsureAvailable(&model.RuntimeConfig{IDE: testSpec.ID, DryRun: true}); err != nil {
		t.Fatalf("expected dry-run EnsureAvailable to bypass checks: %v", err)
	}
}

func TestEnsureAvailableReturnsProbeOutputWhenCommandIsBroken(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "broken-acp")
	script := "#!/bin/sh\nprintf 'adapter exploded' >&2\nexit 7\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write helper script: %v", err)
	}

	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	registerTestSpec(t, Spec{
		ID:           "broken-probe-test",
		DisplayName:  "Broken ACP",
		DefaultModel: "test-model",
		Command:      "broken-acp",
		ProbeArgs:    []string{"probe"},
		InstallHint:  "Reinstall the broken ACP adapter.",
	})

	err := EnsureAvailable(&model.RuntimeConfig{IDE: "broken-probe-test"})
	if err == nil {
		t.Fatal("expected EnsureAvailable error")
	}

	var availabilityErr *AvailabilityError
	if !errors.As(err, &availabilityErr) {
		t.Fatalf("expected AvailabilityError, got %T", err)
	}
	if got := strings.TrimSpace(availabilityErr.Output); got != "adapter exploded" {
		t.Fatalf("unexpected probe output: %q", got)
	}
	if !strings.Contains(err.Error(), "adapter exploded") {
		t.Fatalf("expected probe output in error, got %q", err)
	}
}

func TestDisplayNameReturnsCorrectDisplayNames(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		model.IDEClaude:   "Claude",
		model.IDECodex:    "Codex",
		model.IDEDroid:    "Droid",
		model.IDECursor:   "Cursor",
		model.IDEOpenCode: "OpenCode",
		model.IDEPi:       "Pi",
		model.IDEGemini:   "Gemini",
	}

	for ide, want := range cases {
		if got := DisplayName(ide); got != want {
			t.Fatalf("unexpected display name for %s: got %q want %q", ide, got, want)
		}
	}
}

func TestDriverCatalogExposesCanonicalCommandsAndFallbacks(t *testing.T) {
	t.Parallel()

	entries := DriverCatalog()
	if len(entries) != len(supportedRegistryIDEOrder) {
		t.Fatalf("expected %d driver catalog entries, got %d", len(supportedRegistryIDEOrder), len(entries))
	}

	byIDE := make(map[string]DriverCatalogEntry, len(entries))
	for _, entry := range entries {
		byIDE[entry.IDE] = entry
	}

	cases := []struct {
		ide               string
		wantCommand       []string
		wantProbe         []string
		wantFallbackCount int
	}{
		{
			ide:               model.IDEClaude,
			wantCommand:       []string{"claude-agent-acp"},
			wantProbe:         []string{"claude-agent-acp", "--help"},
			wantFallbackCount: 1,
		},
		{
			ide:               model.IDECodex,
			wantCommand:       []string{"codex-acp"},
			wantProbe:         []string{"codex-acp", "--help"},
			wantFallbackCount: 1,
		},
		{
			ide:               model.IDEDroid,
			wantCommand:       []string{"droid", "exec", "--output-format", "acp"},
			wantProbe:         []string{"droid", "exec", "--help"},
			wantFallbackCount: 1,
		},
		{
			ide:               model.IDECursor,
			wantCommand:       []string{"cursor-agent", "acp"},
			wantProbe:         []string{"cursor-agent", "acp", "--help"},
			wantFallbackCount: 0,
		},
		{
			ide:               model.IDEOpenCode,
			wantCommand:       []string{"opencode", "acp"},
			wantProbe:         []string{"opencode", "acp", "--help"},
			wantFallbackCount: 0,
		},
		{
			ide:               model.IDEPi,
			wantCommand:       []string{"pi-acp"},
			wantProbe:         []string{"pi-acp", "--help"},
			wantFallbackCount: 1,
		},
		{
			ide:               model.IDEGemini,
			wantCommand:       []string{"gemini", "--acp"},
			wantProbe:         []string{"gemini", "--acp", "--help"},
			wantFallbackCount: 1,
		},
	}

	for _, tc := range cases {
		entry, ok := byIDE[tc.ide]
		if !ok {
			t.Fatalf("missing driver catalog entry for %s", tc.ide)
		}
		if !slices.Equal(entry.CanonicalCommand, tc.wantCommand) {
			t.Fatalf(
				"unexpected canonical command for %s: got %v want %v",
				tc.ide,
				entry.CanonicalCommand,
				tc.wantCommand,
			)
		}
		if !slices.Equal(entry.CanonicalProbe, tc.wantProbe) {
			t.Fatalf("unexpected canonical probe for %s: got %v want %v", tc.ide, entry.CanonicalProbe, tc.wantProbe)
		}
		if len(entry.FallbackLaunchers) != tc.wantFallbackCount {
			t.Fatalf(
				"unexpected fallback count for %s: got %d want %d",
				tc.ide,
				len(entry.FallbackLaunchers),
				tc.wantFallbackCount,
			)
		}
	}
}

func TestDriverCatalogCanonicalCommandExcludesDynamicBootstrapArgs(t *testing.T) {
	t.Parallel()

	entry, err := DriverCatalogEntryForIDE(model.IDEDroid)
	if err != nil {
		t.Fatalf("driver catalog entry for droid: %v", err)
	}

	if slices.Contains(entry.CanonicalCommand, "--model") ||
		slices.Contains(entry.CanonicalCommand, "--reasoning-effort") {
		t.Fatalf("expected canonical command to exclude dynamic bootstrap args, got %v", entry.CanonicalCommand)
	}
	if !entry.UsesBootstrapModel {
		t.Fatalf("expected droid catalog entry to report bootstrap-model support, got %#v", entry)
	}
}

func registerTestSpec(t *testing.T, spec Spec) {
	t.Helper()

	registryMu.Lock()
	previous, hadPrevious := registry[spec.ID]
	registry[spec.ID] = spec
	registryMu.Unlock()

	t.Cleanup(func() {
		registryMu.Lock()
		defer registryMu.Unlock()
		if hadPrevious {
			registry[spec.ID] = previous
			return
		}
		delete(registry, spec.ID)
	})
}
