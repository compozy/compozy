package cli

import (
	"fmt"

	"strings"
)

func mergeWorkspaceAddDirs(existing []string, add []string, remove []string) ([]string, error) {
	addDirs := trimmedUniqueStrings(add)
	removeDirs := trimmedUniqueStrings(remove)

	removeSet := make(map[string]struct{}, len(removeDirs))
	for _, dir := range removeDirs {
		removeSet[dir] = struct{}{}
	}
	for _, dir := range addDirs {
		if _, exists := removeSet[dir]; exists {
			return nil, fmt.Errorf("cli: cannot add and remove the same directory: %s", dir)
		}
	}

	merged := make([]string, 0, len(existing)+len(addDirs))
	seen := make(map[string]struct{}, len(existing)+len(addDirs))
	for _, dir := range trimmedUniqueStrings(existing) {
		if _, removed := removeSet[dir]; removed {
			continue
		}
		if _, exists := seen[dir]; exists {
			continue
		}
		seen[dir] = struct{}{}
		merged = append(merged, dir)
	}
	for _, dir := range addDirs {
		if _, exists := seen[dir]; exists {
			continue
		}
		seen[dir] = struct{}{}
		merged = append(merged, dir)
	}
	return merged, nil
}

func trimmedUniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
