package agent

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/model"
)

// Spec defines how to bootstrap an ACP-compatible agent process.
type Spec struct {
	ID                 string
	DisplayName        string
	SetupAgentName     string
	DefaultModel       string
	Command            string
	FixedArgs          []string
	ProbeArgs          []string
	Fallbacks          []Launcher
	SupportsAddDirs    bool
	UsesBootstrapModel bool
	EnvVars            map[string]string
	DocsURL            string
	InstallHint        string
	FullAccessModeID   string
	BootstrapArgs      func(modelName, reasoningEffort string, addDirs []string, accessMode string) []string
}

// Launcher defines one ACP-compatible command shape for a runtime.
type Launcher struct {
	Command   string
	FixedArgs []string
	ProbeArgs []string
}

// DriverCatalogEntry exposes the stable command catalog for one supported ACP runtime.
type DriverCatalogEntry struct {
	IDE                string
	DisplayName        string
	CanonicalCommand   []string
	CanonicalProbe     []string
	FallbackLaunchers  []DriverCatalogLauncher
	SupportsAddDirs    bool
	UsesBootstrapModel bool
	DocsURL            string
	InstallHint        string
}

// DriverCatalogLauncher describes one fallback launcher for a supported ACP runtime.
type DriverCatalogLauncher struct {
	Command []string
	Probe   []string
}

// RuntimeRegistry captures the ACP runtime validation surface needed by
// execution and kernel code.
type RuntimeRegistry interface {
	ValidateRuntimeConfig(cfg *model.RuntimeConfig) error
	EnsureAvailable(ctx context.Context, cfg *model.RuntimeConfig) error
}

// Registry exposes the supported ACP runtime catalog through a value that can be
// passed around as a dependency.
type Registry struct{}

var _ RuntimeRegistry = Registry{}

var supportedRegistryIDEOrder = []string{
	model.IDEClaude,
	model.IDECodex,
	model.IDEDroid,
	model.IDECursor,
	model.IDEOpenCode,
	model.IDEPi,
	model.IDEGemini,
	model.IDECopilot,
}

var (
	registryMu = sync.RWMutex{}
	registry   = map[string]Spec{
		model.IDEClaude: {
			ID:              model.IDEClaude,
			DisplayName:     "Claude",
			SetupAgentName:  "claude-code",
			DefaultModel:    model.DefaultClaudeModel,
			Command:         "claude-agent-acp",
			SupportsAddDirs: true,
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "@agentclientprotocol/claude-agent-acp"},
				},
			},
			DocsURL:          "https://github.com/agentclientprotocol/claude-agent-acp",
			InstallHint:      "Install `@agentclientprotocol/claude-agent-acp` and expose `claude-agent-acp` on PATH.",
			FullAccessModeID: "bypassPermissions",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDECodex: {
			ID:              model.IDECodex,
			DisplayName:     "Codex",
			SetupAgentName:  "codex",
			DefaultModel:    model.DefaultCodexModel,
			Command:         "codex-acp",
			SupportsAddDirs: true,
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "@zed-industries/codex-acp"},
				},
			},
			DocsURL:            "https://github.com/zed-industries/codex-acp",
			InstallHint:        "Install the Codex ACP adapter from the GitHub releases or via `npx @zed-industries/codex-acp`, then expose `codex-acp` on PATH.",
			UsesBootstrapModel: true,
			BootstrapArgs: func(modelName, reasoningEffort string, _ []string, accessMode string) []string {
				args := make([]string, 0, 10)
				if strings.TrimSpace(modelName) != "" {
					args = append(args, "-c", codexConfigOverride("model", modelName))
				}
				if strings.TrimSpace(reasoningEffort) != "" {
					args = append(args, "-c", codexConfigOverride("model_reasoning_effort", reasoningEffort))
				}
				if accessMode != model.AccessModeFull {
					return args
				}
				return append(args,
					"-c", `approval_policy="never"`,
					"-c", `sandbox_mode="danger-full-access"`,
					"-c", `web_search="live"`,
				)
			},
		},
		model.IDEDroid: {
			ID:             model.IDEDroid,
			DisplayName:    "Droid",
			SetupAgentName: "droid",
			DefaultModel:   model.DefaultCodexModel,
			Command:        "droid",
			FixedArgs:      []string{"exec", "--output-format", "acp"},
			ProbeArgs:      []string{"exec", "--help"},
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "droid", "exec", "--output-format", "acp"},
					ProbeArgs: []string{"--yes", "droid", "exec", "--help"},
				},
			},
			UsesBootstrapModel: true,
			EnvVars: map[string]string{
				"DROID_DISABLE_AUTO_UPDATE":         "true",
				"FACTORY_DROID_AUTO_UPDATE_ENABLED": "false",
			},
			DocsURL:     "https://factory.ai/product/cli",
			InstallHint: "Install or upgrade Droid so `droid exec --output-format acp` is available.",
			BootstrapArgs: func(modelName, reasoningEffort string, _ []string, accessMode string) []string {
				args := make([]string, 0, 5)
				if accessMode == model.AccessModeFull {
					args = append(args, "--skip-permissions-unsafe")
				}
				args = append(args, "--model", modelName, "--reasoning-effort", reasoningEffort)
				return args
			},
		},
		model.IDECursor: {
			ID:             model.IDECursor,
			DisplayName:    "Cursor",
			SetupAgentName: "cursor",
			DefaultModel:   model.DefaultCursorModel,
			Command:        "cursor-agent",
			FixedArgs:      []string{"acp"},
			ProbeArgs:      []string{"acp", "--help"},
			DocsURL:        "https://cursor.com/docs/cli/acp",
			InstallHint:    "Install the Cursor agent CLI package and expose `cursor-agent` on PATH so `cursor-agent acp` works.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDEOpenCode: {
			ID:             model.IDEOpenCode,
			DisplayName:    "OpenCode",
			SetupAgentName: "opencode",
			DefaultModel:   model.DefaultOpenCodeModel,
			Command:        "opencode",
			FixedArgs:      []string{"acp"},
			ProbeArgs:      []string{"acp", "--help"},
			DocsURL:        "https://opencode.ai",
			InstallHint:    "Install or upgrade OpenCode so the `opencode acp` subcommand is available.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDEPi: {
			ID:             model.IDEPi,
			DisplayName:    "Pi",
			SetupAgentName: "pi",
			DefaultModel:   model.DefaultPiModel,
			Command:        "pi-acp",
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "pi-acp"},
				},
			},
			DocsURL:     "https://github.com/svkozak/pi-acp",
			InstallHint: "Install `pi-acp` and expose the `pi-acp` binary on PATH.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDEGemini: {
			ID:             model.IDEGemini,
			DisplayName:    "Gemini",
			SetupAgentName: "gemini-cli",
			DefaultModel:   model.DefaultGeminiModel,
			Command:        "gemini",
			FixedArgs:      []string{"--acp"},
			ProbeArgs:      []string{"--acp", "--help"},
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "@google/gemini-cli", "--acp"},
					ProbeArgs: []string{"--yes", "@google/gemini-cli", "--acp", "--help"},
				},
			},
			DocsURL:     "https://geminicli.com",
			InstallHint: "Install Gemini CLI with ACP support so `gemini --acp` succeeds.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
		model.IDECopilot: {
			ID:             model.IDECopilot,
			DisplayName:    "Copilot CLI",
			SetupAgentName: "github-copilot",
			DefaultModel:   model.DefaultCopilotModel,
			Command:        "copilot",
			FixedArgs:      []string{"--acp"},
			ProbeArgs:      []string{"--acp", "--help"},
			Fallbacks: []Launcher{
				{
					Command:   "npx",
					FixedArgs: []string{"--yes", "@github/copilot", "--acp"},
					ProbeArgs: []string{"--yes", "@github/copilot", "--acp", "--help"},
				},
			},
			DocsURL:     "https://docs.github.com/en/copilot/reference/copilot-cli-reference/acp-server",
			InstallHint: "Install GitHub Copilot CLI so `copilot --acp` succeeds.",
			BootstrapArgs: func(_ string, _ string, _ []string, _ string) []string {
				return nil
			},
		},
	}
)

// DefaultRegistry returns the default ACP runtime registry handle.
func DefaultRegistry() RuntimeRegistry {
	return Registry{}
}

// ValidateRuntimeConfig verifies that the runtime config references a supported agent runtime.
func (Registry) ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	return ValidateRuntimeConfig(cfg)
}

// EnsureAvailable verifies that the configured ACP agent binary is installed and executable.
func (Registry) EnsureAvailable(ctx context.Context, cfg *model.RuntimeConfig) error {
	return EnsureAvailable(ctx, cfg)
}

// DisplayName returns the human-readable display name for an agent runtime.
func DisplayName(ide string) string {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return ""
	}
	return spec.DisplayName
}

// SetupAgentName returns the setup/install agent identifier for one ACP runtime.
func SetupAgentName(ide string) (string, error) {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(spec.SetupAgentName) == "" {
		return "", fmt.Errorf("agent runtime %q does not declare a setup agent", ide)
	}
	return spec.SetupAgentName, nil
}

func codexConfigOverride(key, value string) string {
	return strings.TrimSpace(key) + "=" + strconv.Quote(strings.TrimSpace(value))
}

// DriverCatalog returns the stable ACP driver catalog in the supported IDE order.
func DriverCatalog() []DriverCatalogEntry {
	snapshot := currentCatalogSnapshot()

	entries := make([]DriverCatalogEntry, 0, len(snapshot.order))
	for _, ide := range snapshot.order {
		spec, ok := snapshot.specs[ide]
		if !ok {
			continue
		}
		entries = append(entries, driverCatalogEntryFromSpec(spec))
	}
	return entries
}

// DriverCatalogEntryForIDE returns the stable driver catalog entry for one supported IDE.
func DriverCatalogEntryForIDE(ide string) (DriverCatalogEntry, error) {
	spec, err := lookupAgentSpec(ide)
	if err != nil {
		return DriverCatalogEntry{}, err
	}
	return driverCatalogEntryFromSpec(spec), nil
}

func lookupAgentSpec(ide string) (Spec, error) {
	snapshot := currentCatalogSnapshot()
	spec, ok := snapshot.specs[strings.TrimSpace(strings.ToLower(ide))]
	if !ok {
		return Spec{}, fmt.Errorf("unknown agent runtime %q", ide)
	}
	return cloneAgentSpec(spec), nil
}

func cloneAgentSpec(spec Spec) Spec {
	spec.EnvVars = mapsClone(spec.EnvVars)
	spec.FixedArgs = slices.Clone(spec.FixedArgs)
	spec.ProbeArgs = slices.Clone(spec.ProbeArgs)
	spec.Fallbacks = cloneLaunchers(spec.Fallbacks)
	return spec
}

func driverCatalogEntryFromSpec(spec Spec) DriverCatalogEntry {
	primary := spec.primaryLauncher()
	entry := DriverCatalogEntry{
		IDE:                spec.ID,
		DisplayName:        spec.DisplayName,
		CanonicalCommand:   primary.catalogCommand(),
		CanonicalProbe:     primary.probeCommand(),
		SupportsAddDirs:    spec.SupportsAddDirs,
		UsesBootstrapModel: spec.UsesBootstrapModel,
		DocsURL:            spec.DocsURL,
		InstallHint:        spec.InstallHint,
	}
	if len(spec.Fallbacks) > 0 {
		entry.FallbackLaunchers = make([]DriverCatalogLauncher, 0, len(spec.Fallbacks))
		for _, launcher := range spec.Fallbacks {
			entry.FallbackLaunchers = append(entry.FallbackLaunchers, DriverCatalogLauncher{
				Command: launcher.catalogCommand(),
				Probe:   launcher.probeCommand(),
			})
		}
	}
	return entry
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

func cloneLaunchers(src []Launcher) []Launcher {
	if len(src) == 0 {
		return nil
	}
	dst := make([]Launcher, len(src))
	for i, launcher := range src {
		dst[i] = Launcher{
			Command:   launcher.Command,
			FixedArgs: slices.Clone(launcher.FixedArgs),
			ProbeArgs: slices.Clone(launcher.ProbeArgs),
		}
	}
	return dst
}
