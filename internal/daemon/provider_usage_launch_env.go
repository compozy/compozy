package daemon

import (
	"errors"
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
// It also reports the keys the command itself assigned a non-empty value that no later `-u`/`-i`
// removed; inherited daemon values are never owned.
func applyLaunchEnvironment(env []string, parsed config.LaunchCommand) ([]string, map[string]bool, error) {
	owned := map[string]bool{}
	assign := func(key, value string) {
		env = providerenv.SetEnvValue(env, key, value)
		if value == "" {
			delete(owned, key)
		} else {
			owned[key] = true
		}
	}
	for _, entry := range parsed.Environment {
		key, value, _ := strings.Cut(entry, "=")
		assign(key, value)
	}
	if filepath.Base(parsed.Executable) != envUtility {
		return env, owned, nil
	}
	args := parsed.Args
	for i := 0; i < len(args); i++ {
		if split, ok, rest := envSplitString(args, i); ok {
			// GNU env escape semantics differ from shell quoting; ambiguous routes must not collect.
			if strings.ContainsAny(split, `\$`) {
				return nil, nil, errors.New(
					"usage collection does not support escapes or expansion in env split strings",
				)
			}
			words, err := shellquote.Split(split)
			if err != nil {
				return nil, nil, err
			}
			args = append(words, args[rest:]...)
			i = -1
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			env, i = applyEnvOption(env, owned, args, i)
			continue
		}
		key, value, ok := strings.Cut(args[i], "=")
		if !ok || key == "" {
			return env, owned, nil // the command starts here
		}
		assign(key, value)
	}
	return env, owned, nil
}

// applyEnvOption applies one `env` option at index i, dropping removed keys from owned, and
// returns the index of its last argument.
func applyEnvOption(env []string, owned map[string]bool, args []string, i int) ([]string, int) {
	arg := args[i]
	unset := func(key string) []string { delete(owned, key); return unsetEnvValue(env, key) }
	switch {
	case arg == "-i" || arg == "-" || arg == "--ignore-environment":
		clear(owned)
		return nil, i
	case arg == "-u" || arg == "--unset":
		if i+1 < len(args) {
			return unset(args[i+1]), i + 1
		}
	case strings.HasPrefix(arg, "--unset="):
		return unset(strings.TrimPrefix(arg, "--unset=")), i
	case strings.HasPrefix(arg, "-u") && len(arg) > 2:
		return unset(arg[2:]), i
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
