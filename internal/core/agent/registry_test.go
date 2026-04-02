package agent

import (
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestAgentRegistryEntries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		ide        string
		wantBinary string
		wantArgs   []string
		modelName  string
		reasoning  string
		addDirs    []string
	}{
		{
			name:       "claude",
			ide:        model.IDEClaude,
			wantBinary: "claude-agent-acp",
			wantArgs:   []string{"--model", model.DefaultClaudeModel, "--add-dir", "../shared", "--add-dir", "../docs"},
			reasoning:  "medium",
			addDirs:    []string{"../shared", "../docs"},
		},
		{
			name:       "codex",
			ide:        model.IDECodex,
			wantBinary: "codex-acp",
			wantArgs: []string{
				"--model",
				model.DefaultCodexModel,
				"--reasoning-effort",
				"medium",
				"--add-dir",
				"../shared",
				"--add-dir",
				"../docs",
			},
			reasoning: "medium",
			addDirs:   []string{"../shared", "../docs"},
		},
		{
			name:       "droid",
			ide:        model.IDEDroid,
			wantBinary: "droid",
			wantArgs:   []string{"--model", model.DefaultCodexModel, "--reasoning-effort", "medium"},
			reasoning:  "medium",
		},
		{
			name:       "cursor",
			ide:        model.IDECursor,
			wantBinary: "cursor-acp",
			wantArgs:   []string{"--model", model.DefaultCursorModel},
			reasoning:  "medium",
		},
		{
			name:       "opencode",
			ide:        model.IDEOpenCode,
			wantBinary: "opencode",
			wantArgs:   []string{"--model", model.DefaultOpenCodeModel, "--thinking", "medium"},
			reasoning:  "medium",
		},
		{
			name:       "pi",
			ide:        model.IDEPi,
			wantBinary: "pi",
			wantArgs:   []string{"--model", model.DefaultPiModel, "--thinking", "medium"},
			reasoning:  "medium",
		},
		{
			name:       "gemini",
			ide:        model.IDEGemini,
			wantBinary: "gemini",
			wantArgs:   []string{"--experimental-acp", "--model", model.DefaultGeminiModel},
			reasoning:  "medium",
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

			if spec.Binary != tc.wantBinary {
				t.Fatalf("unexpected binary for %s: got %q want %q", tc.ide, spec.Binary, tc.wantBinary)
			}

			gotArgs := spec.BootstrapArgs(resolveModel(spec, tc.modelName), tc.reasoning, tc.addDirs)
			if len(gotArgs) != len(tc.wantArgs) {
				t.Fatalf("unexpected arg count for %s: got %v want %v", tc.ide, gotArgs, tc.wantArgs)
			}
			for idx := range tc.wantArgs {
				if gotArgs[idx] != tc.wantArgs[idx] {
					t.Fatalf("unexpected args for %s: got %v want %v", tc.ide, gotArgs, tc.wantArgs)
				}
			}
		})
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
		Mode:      model.ExecutionModePRDTasks,
		IDE:       model.IDECodex,
		BatchSize: 2,
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

func TestEnsureAvailableReturnsErrorWhenBinaryMissing(t *testing.T) {
	testSpec := Spec{
		ID:           "missing-binary-test",
		DisplayName:  "Missing",
		DefaultModel: "test-model",
		Binary:       "definitely-not-installed-binary",
		BootstrapArgs: func(_, _ string, _ []string) []string {
			return nil
		},
	}
	registerTestSpec(t, testSpec)

	err := EnsureAvailable(&model.RuntimeConfig{IDE: testSpec.ID})
	if err == nil {
		t.Fatal("expected EnsureAvailable error")
	}

	if err := EnsureAvailable(&model.RuntimeConfig{IDE: testSpec.ID, DryRun: true}); err != nil {
		t.Fatalf("expected dry-run EnsureAvailable to bypass checks: %v", err)
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

func TestBuildShellCommandString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		ide     string
		addDirs []string
		want    string
	}{
		{
			name:    "claude",
			ide:     model.IDEClaude,
			addDirs: []string{"../shared", "../docs"},
			want:    "claude-agent-acp --model opus --add-dir ../shared --add-dir ../docs",
		},
		{
			name:    "codex",
			ide:     model.IDECodex,
			addDirs: []string{"../shared", "../docs"},
			want:    "codex-acp --model gpt-5.4 --reasoning-effort medium --add-dir ../shared --add-dir ../docs",
		},
		{
			name: "droid",
			ide:  model.IDEDroid,
			want: "droid --model gpt-5.4 --reasoning-effort medium",
		},
		{
			name: "cursor",
			ide:  model.IDECursor,
			want: "cursor-acp --model composer-1",
		},
		{
			name: "opencode",
			ide:  model.IDEOpenCode,
			want: "opencode --model anthropic/claude-opus-4-6 --thinking medium",
		},
		{
			name: "pi",
			ide:  model.IDEPi,
			want: "pi --model anthropic/claude-opus-4-6 --thinking medium",
		},
		{
			name: "gemini",
			ide:  model.IDEGemini,
			want: "gemini --experimental-acp --model gemini-2.5-pro",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := BuildShellCommandString(tc.ide, "", tc.addDirs, "medium")
			if got != tc.want {
				t.Fatalf("unexpected shell command\nwant: %s\ngot:  %s", tc.want, got)
			}
		})
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
