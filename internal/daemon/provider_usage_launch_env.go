package daemon

import (
	"path/filepath"
	"strings"

	"github.com/kballard/go-shellquote"

	config "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerenv"
)

const envUtility = "env"

// applyLaunchEnvironment applies a launch command's environment to env the way the provider
// process sees it: leading NAME=value assignments, then an `env` utility prefix
// (`env [-i] [-u NAME] [-C DIR] [-S STRING] NAME=value... command`) such as the one a second Claude account uses
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
		if split, ok, rest := envSplitString(args, i); ok {
			// `env -S` packs further options, assignments, and the command into one argument.
			words, err := shellquote.Split(split)
			if err != nil {
				return env
			}
			args = append(words, args[rest:]...)
			i = -1
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			env, i = applyEnvOption(env, args, i)
			continue
		}
		key, value, ok := strings.Cut(args[i], "=")
		if !ok || key == "" {
			return env // the command starts here
		}
		env = providerenv.SetEnvValue(env, key, value)
	}
	return env
}

// applyEnvOption applies one `env` option at index i and returns the index of its last argument.
func applyEnvOption(env []string, args []string, i int) ([]string, int) {
	arg := args[i]
	switch {
	case arg == "-i" || arg == "-" || arg == "--ignore-environment":
		return nil, i
	case arg == "-u" || arg == "--unset":
		if i+1 < len(args) {
			return unsetEnvValue(env, args[i+1]), i + 1
		}
	case strings.HasPrefix(arg, "--unset="):
		return unsetEnvValue(env, strings.TrimPrefix(arg, "--unset=")), i
	case strings.HasPrefix(arg, "-u") && len(arg) > 2:
		return unsetEnvValue(env, arg[2:]), i
	case arg == "-C" || arg == "--chdir":
		return env, i + 1 // the working directory operand is not an assignment
	}
	return env, i
}

// envSplitString reports the string of an `env -S`/`--split-string` option at index i and the
// index of the first argument after it.
func envSplitString(args []string, i int) (string, bool, int) {
	arg := args[i]
	switch {
	case arg == "-S" || arg == "--split-string":
		if i+1 < len(args) {
			return args[i+1], true, i + 2
		}
		return "", false, i + 1
	case strings.HasPrefix(arg, "--split-string="):
		return strings.TrimPrefix(arg, "--split-string="), true, i + 1
	case strings.HasPrefix(arg, "-S") && len(arg) > 2:
		return arg[2:], true, i + 1
	default:
		return "", false, i + 1
	}
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
