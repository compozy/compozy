package runtimemcp

import (
	"os"
	"strings"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerenv"
)

func ProviderContext(
	home config.HomePaths,
	provider string,
	resolved config.ProviderConfig,
	environ []string,
	cwd, worktreeRoot string,
) (ReadContext, error) {
	policy := resolved.EffectiveHomePolicy()
	env, err := providerenv.ResolveHomeEnv(home, provider, policy, environ)
	if err != nil {
		return ReadContext{}, err
	}
	if resolved.EffectiveHarness() == config.ProviderHarnessPiACP || provider == "pi" {
		env, err = providerenv.ResolvePiAgentDirEnv(home, provider, policy, env)
		if err != nil {
			return ReadContext{}, err
		}
	}
	context := ReadContext{CWD: cwd, WorktreeRoot: worktreeRoot, Env: Environment(env)}
	for _, value := range env {
		if home, ok := strings.CutPrefix(value, "HOME="); ok {
			context.Home = home
		}
	}
	if context.Home == "" {
		context.Home, err = os.UserHomeDir()
		if err != nil {
			return ReadContext{}, err
		}
	}
	return ResolveContext(context), nil
}
