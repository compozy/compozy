package session

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	speedpkg "github.com/compozy/compozy/internal/speed"
	shellquote "github.com/kballard/go-shellquote"
)

const openClawRuntimeProvider = "openclaw"

type providerRuntimeAdapter struct {
	strategy acp.RuntimeApplicationStrategy
	compile  func(acp.StartOpts) (acp.StartOpts, error)
}

var providerRuntimeAdapters = map[string]providerRuntimeAdapter{
	cursorRuntimeProvider: {
		strategy: acp.RuntimeApplicationLaunchArg,
		compile:  compileCursorRuntime,
	},
	openClawRuntimeProvider: {
		strategy: acp.RuntimeApplicationProviderManaged,
		compile:  compileProviderManagedRuntime,
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

func compileCursorRuntime(opts acp.StartOpts) (acp.StartOpts, error) {
	model := strings.TrimSpace(opts.PreferredModel)
	if model == "" {
		opts.ReasoningEffort = ""
		opts.Speed = ""
		return opts, nil
	}
	transportModel := strings.TrimSpace(opts.ExpectedTransportModel)
	if transportModel == "" {
		return acp.StartOpts{}, errors.New("session: Cursor transport model binding is required")
	}
	command, err := cursorLaunchCommand(opts.Command, transportModel)
	if err != nil {
		return acp.StartOpts{}, err
	}
	opts.Command = command
	opts.ExpectedTransportModel = transportModel
	opts.ReasoningEffort = ""
	opts.Speed = ""
	return opts, nil
}

func compileProviderManagedRuntime(opts acp.StartOpts) (acp.StartOpts, error) {
	if strings.TrimSpace(opts.PreferredModel) != "" {
		return acp.StartOpts{}, errors.New(
			"session: provider-managed runtime does not support model selection",
		)
	}
	if strings.TrimSpace(opts.ReasoningEffort) != "" {
		return acp.StartOpts{}, errors.New(
			"session: provider-managed runtime does not support reasoning effort",
		)
	}
	if opts.Speed != "" && opts.Speed != speedpkg.SpeedNormal {
		return acp.StartOpts{}, errors.New(
			"session: provider-managed runtime does not support Fast mode",
		)
	}
	if len(opts.ACPOptions) > 0 {
		return acp.StartOpts{}, errors.New(
			"session: provider-managed runtime does not support ACP options",
		)
	}
	if strings.TrimSpace(opts.ExpectedTransportModel) != "" {
		return acp.StartOpts{}, errors.New(
			"session: provider-managed runtime cannot use a transport model binding",
		)
	}
	opts.PreferredModel = ""
	opts.ReasoningEffort = ""
	opts.Speed = ""
	opts.ExpectedTransportModel = ""
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
