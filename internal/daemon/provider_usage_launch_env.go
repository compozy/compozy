package daemon

import (
	"path/filepath"
	"strings"

	config "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerenv"
)

const envUtility = "env"

// applyLaunchEnvironment applies a launch command's environment to env the way the provider
// process sees it: leading NAME=value assignments, then an `env` utility prefix
// (`env [-i] [-u NAME]... NAME=value... command`) such as the one a second Claude account uses
// to point at its own CLAUDE_CONFIG_DIR. Usage collection must read that account's files.
func applyLaunchEnvironment(env []string, parsed config.LaunchCommand) []string {
	for _, entry := range parsed.Environment {
		key, value, _ := strings.Cut(entry, "=")
		env = providerenv.SetEnvValue(env, key, value)
	}
	if filepath.Base(parsed.Executable) != envUtility {
		return env
	}
	args := parsed.Args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			continue
		case arg == "-i" || arg == "-" || arg == "--ignore-environment":
			env = nil
		case arg == "-u" || arg == "--unset":
			if i+1 < len(args) {
				i++
				env = unsetEnvValue(env, args[i])
			}
		case strings.HasPrefix(arg, "--unset="):
			env = unsetEnvValue(env, strings.TrimPrefix(arg, "--unset="))
		case strings.HasPrefix(arg, "-u") && len(arg) > 2:
			env = unsetEnvValue(env, arg[2:])
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			key, value, ok := strings.Cut(arg, "=")
			if !ok || key == "" {
				return env
			}
			env = providerenv.SetEnvValue(env, key, value)
		}
	}
	return env
}

func unsetEnvValue(env []string, key string) []string {
	prefix := key + "="
	result := env[:0:0]
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return result
}
