package session

import (
	"slices"

	"github.com/compozy/compozy/internal/subprocess"
)

func providerLookPath(env []string, cwd string) func(string) (string, error) {
	snapshot := slices.Clone(env)
	return func(command string) (string, error) {
		return subprocess.ResolveExecutable(command, snapshot, cwd)
	}
}
