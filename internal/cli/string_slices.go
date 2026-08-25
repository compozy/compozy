package cli

import "strings"

func trimStringAtoms(values []string) []string {
	atoms := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			atoms = append(atoms, trimmed)
		}
	}
	return atoms
}
