package config

import (
	"fmt"
	"strings"
)

// ValidateRemovedConfigPath rejects retired agent configuration without translating it.
func ValidateRemovedConfigPath(path []string) error {
	if len(path) >= 2 && path[0] == AgentsDirName && path[1] == "stays_on" {
		return fmt.Errorf(
			"removed config key %q: use %q",
			strings.Join(path, "."),
			strings.Join(append([]string{AgentsDirName, "always_on"}, path[2:]...), "."),
		)
	}
	return nil
}
