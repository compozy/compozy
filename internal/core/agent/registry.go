package agent

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/model"
)

// Spec defines how to bootstrap an ACP-compatible agent process.
type Spec struct {
	ID              string
	DisplayName     string
	DefaultModel    string
	Binary          string
	SupportsAddDirs bool
	EnvVars         map[string]string
	BootstrapArgs   func(modelName, reasoningEffort string, addDirs []string) []string
}

var supportedRegistryIDEOrder = []string{
	model.IDEClaude,
	model.IDECodex,
	model.IDEDroid,
	model.IDECursor,
	model.IDEOpenCode,
	model.IDEPi,
	model.IDEGemini,
}

var (
	registryMu = sync.RWMutex{}
	registry   = map[string]Spec{
		model.IDEClaude: {
			ID:              model.IDEClaude,
			DisplayName:     "Claude",
			DefaultModel:    model.DefaultClaudeModel,
			Binary:          "claude-agent-acp",
			SupportsAddDirs: true,
			BootstrapArgs: func(modelName, _ string, addDirs []string) []string {
				args := []string{"--model", modelName}
				return appendACPAddDirs(args, addDirs)
			},
		},
		model.IDECodex: {
			ID:              model.IDECodex,
			DisplayName:     "Codex",
			DefaultModel:    model.DefaultCodexModel,
			Binary:          "codex-acp",
			SupportsAddDirs: true,
			BootstrapArgs: func(modelName, reasoningEffort string, addDirs []string) []string {
				args := []string{"--model", modelName, "--reasoning-effort", reasoningEffort}
				return appendACPAddDirs(args, addDirs)
			},
		},
		model.IDEDroid: {
			ID:           model.IDEDroid,
			DisplayName:  "Droid",
			DefaultModel: model.DefaultCodexModel,
			Binary:       "droid",
			BootstrapArgs: func(modelName, reasoningEffort string, _ []string) []string {
				return []string{"--model", modelName, "--reasoning-effort", reasoningEffort}
			},
		},
		model.IDECursor: {
			ID:           model.IDECursor,
			DisplayName:  "Cursor",
			DefaultModel: model.DefaultCursorModel,
			Binary:       "cursor-acp",
			BootstrapArgs: func(modelName, _ string, _ []string) []string {
				return []string{"--model", modelName}
			},
		},
		model.IDEOpenCode: {
			ID:           model.IDEOpenCode,
			DisplayName:  "OpenCode",
			DefaultModel: model.DefaultOpenCodeModel,
			Binary:       "opencode",
			BootstrapArgs: func(modelName, reasoningEffort string, _ []string) []string {
				return []string{"--model", modelName, "--thinking", reasoningEffort}
			},
		},
		model.IDEPi: {
			ID:           model.IDEPi,
			DisplayName:  "Pi",
			DefaultModel: model.DefaultPiModel,
			Binary:       "pi",
			BootstrapArgs: func(modelName, reasoningEffort string, _ []string) []string {
				return []string{"--model", modelName, "--thinking", reasoningEffort}
			},
		},
		model.IDEGemini: {
			ID:           model.IDEGemini,
			DisplayName:  "Gemini",
			DefaultModel: model.DefaultGeminiModel,
			Binary:       "gemini",
			BootstrapArgs: func(modelName, _ string, _ []string) []string {
				return []string{"--experimental-acp", "--model", modelName}
			},
		},
	}
)

// ValidateRuntimeConfig verifies that the runtime config references a supported agent runtime.
func ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	if cfg.Mode != model.ExecutionModePRReview && cfg.Mode != model.ExecutionModePRDTasks {
		return fmt.Errorf(
			"invalid --mode value %q: must be %q or %q",
			cfg.Mode,
			model.ModeCodeReview,
			model.ModePRDTasks,
		)
	}
	if _, err := lookupAgentSpec(cfg.IDE); err != nil {
		return fmt.Errorf("invalid --ide value %q: must be %s", cfg.IDE, quotedSupportedIDEs())
	}
	if cfg.Mode == model.ExecutionModePRDTasks && cfg.BatchSize != 1 {
		return fmt.Errorf("batch size must be 1 for prd-tasks mode (got %d)", cfg.BatchSize)
	}
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max-retries cannot be negative (got %d)", cfg.MaxRetries)
	}
	if cfg.RetryBackoffMultiplier <= 0 {
		return fmt.Errorf("retry-backoff-multiplier must be positive (got %.2f)", cfg.RetryBackoffMultiplier)
	}
	return nil
}

// EnsureAvailable verifies that the configured ACP agent binary is installed and executable.
func EnsureAvailable(cfg *model.RuntimeConfig) error {
	if cfg.DryRun {
		return nil
	}

	spec, err := lookupAgentSpec(cfg.IDE)
	if err != nil {
		return err
	}
	if err := assertBinaryExists(spec.Binary); err != nil {
		return err
	}
	if err := assertBinarySupported(spec.Binary); err != nil {
		return err
	}
	return nil
}

// DisplayName returns the human-readable display name for an agent runtime.
func DisplayName(ide string) string {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return ""
	}
	return spec.DisplayName
}

// BuildShellCommandString renders a shell preview for the configured ACP agent bootstrap command.
func BuildShellCommandString(ide string, modelName string, addDirs []string, reasoningEffort string) string {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return ""
	}

	resolvedModel := resolveModel(spec, modelName)
	resolvedDirs := addDirs
	if !spec.SupportsAddDirs {
		resolvedDirs = nil
	}
	args := append([]string{spec.Binary}, spec.BootstrapArgs(resolvedModel, reasoningEffort, resolvedDirs)...)

	parts := make([]string, 0, len(spec.EnvVars)+1)
	parts = append(parts, sortedEnvAssignments(spec.EnvVars)...)
	parts = append(parts, formatShellCommand(args))
	return strings.Join(parts, " ")
}

func appendACPAddDirs(args []string, addDirs []string) []string {
	for _, dir := range normalizeAddDirs(addDirs) {
		args = append(args, "--add-dir", dir)
	}
	return args
}

func assertBinaryExists(binary string) error {
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("%s CLI not found on PATH", binary)
	}
	return nil
}

func assertBinarySupported(binary string) error {
	cmd := exec.CommandContext(context.Background(), binary, "--help")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s CLI does not appear to be properly installed or configured", binary)
	}
	return nil
}

func lookupAgentSpec(ide string) (Spec, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	spec, ok := registry[ide]
	if !ok {
		return Spec{}, fmt.Errorf("unknown agent runtime %q", ide)
	}
	return cloneAgentSpec(spec), nil
}

func resolveModel(spec Spec, modelName string) string {
	if strings.TrimSpace(modelName) != "" {
		return modelName
	}
	return spec.DefaultModel
}

func quotedSupportedIDEs() string {
	items := make([]string, 0, len(supportedRegistryIDEOrder))
	for _, ide := range supportedRegistryIDEOrder {
		items = append(items, fmt.Sprintf("%q", ide))
	}
	return strings.Join(items, ", ")
}

func sortedEnvAssignments(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	assignments := make([]string, 0, len(keys))
	for _, key := range keys {
		assignments = append(assignments, fmt.Sprintf("%s=%s", key, formatShellArg(env[key])))
	}
	return assignments
}

func cloneAgentSpec(spec Spec) Spec {
	if len(spec.EnvVars) == 0 {
		return spec
	}
	spec.EnvVars = mapsClone(spec.EnvVars)
	return spec
}

func mapsClone(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func normalizeAddDirs(dirs []string) []string {
	if len(dirs) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(dirs))
	normalized := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func formatShellCommand(args []string) string {
	formatted := make([]string, len(args))
	for i, arg := range args {
		formatted[i] = formatShellArg(arg)
	}
	return strings.Join(formatted, " ")
}

func formatShellArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if strings.ContainsAny(arg, " \t\n\"'\\$`|&;<>*?[]{}()") {
		return strconv.Quote(arg)
	}
	return arg
}
