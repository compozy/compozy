package providerexec

import (
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/subprocess"
	shellquote "github.com/kballard/go-shellquote"
)

// StrategyKind identifies how a provider command reaches its ACP runtime.
type StrategyKind string

const (
	// StrategyDirectACP launches a CLI that speaks ACP itself.
	StrategyDirectACP StrategyKind = "direct_acp"
	// StrategyNativeCLIBridge launches an ACP adapter that wraps another installed CLI.
	StrategyNativeCLIBridge StrategyKind = "native_cli_bridge"
	// StrategyHarnessAdapter launches a shared harness configured for a downstream API provider.
	StrategyHarnessAdapter StrategyKind = "harness_adapter"
)

// NativeCLI identifies the installed CLI consumed by an ACP bridge.
type NativeCLI struct {
	Command      string
	BridgeEnvKey string
}

// Strategy describes the provider-neutral execution plan for one effective provider command.
type Strategy struct {
	Kind      StrategyKind
	NativeCLI NativeCLI
}

type bridgeSpec struct {
	adapterToken string
	nativeCLI    NativeCLI
}

var bridgeRegistry = []bridgeSpec{
	{
		adapterToken: "@agentclientprotocol/claude-agent-acp",
		nativeCLI:    NativeCLI{Command: "claude", BridgeEnvKey: "CLAUDE_CODE_EXECUTABLE"},
	},
	{
		adapterToken: "@agentclientprotocol/codex-acp",
		nativeCLI:    NativeCLI{Command: "codex", BridgeEnvKey: "CODEX_PATH"},
	},
}

// StrategyFor classifies the effective provider command without coupling execution to a provider ID.
func StrategyFor(provider compozyconfig.ProviderConfig) Strategy {
	if nativeCLI, ok := nativeCLIBridgeForCommand(provider.Command); ok {
		return Strategy{Kind: StrategyNativeCLIBridge, NativeCLI: nativeCLI}
	}
	if provider.EffectiveHarness() == compozyconfig.ProviderHarnessPiACP {
		return Strategy{Kind: StrategyHarnessAdapter}
	}
	return Strategy{Kind: StrategyDirectACP}
}

// ResolveNativeCLI resolves a bridge's native CLI against the exact launch environment.
func ResolveNativeCLI(strategy Strategy, env []string, cwd string) (string, error) {
	if strategy.Kind != StrategyNativeCLIBridge {
		return "", nil
	}
	executable, err := subprocess.ResolveExecutable(
		strategy.NativeCLI.Command,
		append([]string(nil), env...),
		cwd,
	)
	if err != nil {
		return "", fmt.Errorf(
			"provider executable: resolve bridge native CLI %q: %w",
			strategy.NativeCLI.Command,
			err,
		)
	}
	return executable, nil
}

func nativeCLIBridgeForCommand(command string) (NativeCLI, bool) {
	args, err := shellquote.Split(strings.TrimSpace(command))
	if err != nil {
		return NativeCLI{}, false
	}
	for _, arg := range args {
		token := normalizedAdapterToken(arg)
		for _, bridge := range bridgeRegistry {
			if token == bridge.adapterToken {
				return bridge.nativeCLI, true
			}
		}
	}
	return NativeCLI{}, false
}

func normalizedAdapterToken(value string) string {
	token := strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(token, "@") {
		return token
	}
	versionSeparator := strings.LastIndex(token, "@")
	packageSeparator := strings.Index(token, "/")
	if versionSeparator > packageSeparator {
		return token[:versionSeparator]
	}
	return token
}
