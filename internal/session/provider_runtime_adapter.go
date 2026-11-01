package session

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	shellquote "github.com/kballard/go-shellquote"
)

type providerRuntimeAdapter struct {
	strategy acp.RuntimeApplicationStrategy
	// liveApply records when a session_config change sent to a live process takes effect;
	// empty means the conservative next-turn default.
	liveApply acp.RuntimeLiveApplication
	compile   func(acp.StartOpts) (acp.StartOpts, error)
}

var providerRuntimeAdapters = map[string]providerRuntimeAdapter{
	cursorRuntimeProvider: {
		strategy: acp.RuntimeApplicationLaunchArg,
		compile:  compileCursorRuntime,
	},
	// claude-agent-acp applies set_config_option to the running Agent SDK query
	// (applyFlagSettings/setModel) without an in-flight guard.
	runtimeProviderClaude: {
		strategy:  acp.RuntimeApplicationSessionConfig,
		liveApply: acp.RuntimeLiveApplyNextStep,
	},
}

func applyProviderRuntimeAdapter(
	resolved compozyconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, error) {
	providerID := strings.TrimSpace(resolved.RuntimeProvider)
	if providerID == "" {
		providerID = strings.TrimSpace(resolved.Provider)
	}
	adapter, ok := providerRuntimeAdapters[providerID]
	if !ok {
		opts.RuntimeStrategy = RuntimeStrategyForProvider(providerID)
		return opts, nil
	}
	opts.RuntimeStrategy = adapter.strategy
	if adapter.compile == nil {
		return opts, nil
	}
	return adapter.compile(opts)
}

// RuntimeStrategyForProvider returns the public runtime application strategy
// owned by the provider adapter registry.
func RuntimeStrategyForProvider(providerID string) acp.RuntimeApplicationStrategy {
	adapter, ok := providerRuntimeAdapters[strings.TrimSpace(providerID)]
	if !ok {
		return acp.RuntimeApplicationSessionConfig
	}
	return adapter.strategy
}

// RuntimeLiveApplyForProvider reports when a live session_config change reaches the agent.
// Providers that apply runtime controls outside session config report none: their change
// waits for the next prompt's process binding.
func RuntimeLiveApplyForProvider(providerID string) acp.RuntimeLiveApplication {
	if RuntimeStrategyForProvider(providerID) != acp.RuntimeApplicationSessionConfig {
		return ""
	}
	adapter, ok := providerRuntimeAdapters[strings.TrimSpace(providerID)]
	if ok && adapter.liveApply != "" {
		return adapter.liveApply
	}
	return acp.RuntimeLiveApplyNextTurn
}

func compileCursorRuntime(opts acp.StartOpts) (acp.StartOpts, error) {
	model := strings.TrimSpace(opts.PreferredModel)
	if model == "" {
		opts.ReasoningEffort = ""
		opts.Speed = ""
		return opts, nil
	}
	transportModel := strings.TrimSpace(opts.LaunchModelID)
	if transportModel == "" {
		return acp.StartOpts{}, errors.New("session: Cursor transport model binding is required")
	}
	command, err := cursorLaunchCommand(opts.Command, transportModel)
	if err != nil {
		return acp.StartOpts{}, err
	}
	opts.Command = command
	opts.LaunchModelID = transportModel
	opts.ReasoningEffort = ""
	opts.Speed = ""
	return opts, nil
}

func cursorLaunchCommand(command string, transportModel string) (string, error) {
	argv, err := shellquote.Split(strings.TrimSpace(command))
	if err != nil {
		return "", fmt.Errorf("session: parse Cursor command: %w", err)
	}
	if len(argv) == 0 {
		return "", errors.New("session: Cursor command is required")
	}
	if slices.Contains(argv, "--model") || slices.Contains(argv, "-m") {
		return "", errors.New("session: Cursor provider command must not contain a model override")
	}
	acpIndex := slices.Index(argv, "acp")
	if acpIndex < 0 {
		return "", errors.New("session: Cursor provider command must contain the acp subcommand")
	}
	withModel := make([]string, 0, len(argv)+2)
	withModel = append(withModel, argv[:acpIndex]...)
	withModel = append(withModel, "--model", transportModel)
	withModel = append(withModel, argv[acpIndex:]...)
	return shellquote.Join(withModel...), nil
}
