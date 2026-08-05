package subprocess

import (
	"errors"
	"fmt"
	"path/filepath"
)

func resolvedCommand(
	command string,
	executable string,
	args []string,
	env []string,
	dir string,
) (string, []string, error) {
	resolvedPath := executable
	if resolvedPath == "" {
		var err error
		resolvedPath, err = ResolveExecutable(command, env, dir)
		if err != nil {
			return "", nil, fmt.Errorf("subprocess: resolve executable %q: %w", command, err)
		}
	} else if !filepath.IsAbs(resolvedPath) {
		return "", nil, errors.New("subprocess: resolved executable must be absolute")
	}

	commandArgs := make([]string, 0, len(args)+1)
	commandArgs = append(commandArgs, resolvedPath)
	commandArgs = append(commandArgs, args...)
	return resolvedPath, commandArgs, nil
}
