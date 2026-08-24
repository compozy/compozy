package extensionpkg

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var reservedCommandFlags = map[string]struct{}{
	commandAgentFlagName: {}, "approval-token": {}, "cmd": {}, "help": {}, "input": {}, "json": {},
	"o": {}, "output": {}, commandSessionFlagName: {}, capabilityWorkspaceKey: {},
}

const reservedCommandFlagList = "agent, approval-token, cmd, help, input, json, o, output, session, workspace"

const (
	commandAgentFlagName   = "agent"
	commandSessionFlagName = "session"
)

func validateCommandDeclarations(manifest *Manifest) error {
	if manifest == nil {
		return nil
	}
	leaves := make(map[string]string)
	for _, toolName := range sortedMapKeys(manifest.Resources.Tools) {
		tool := manifest.Resources.Tools[toolName]
		if tool.Command == nil {
			continue
		}
		field := "resources.tools." + toolName + ".command"
		verb, err := validateCommandPath(field+".verb", tool.Command.Verb, 2)
		if err != nil {
			return err
		}
		if previous, ok := leaves[verb]; ok {
			return &ManifestValidationError{
				Field:   field + ".verb",
				Value:   verb,
				Message: "duplicate command path also declared by resources.tools." + previous,
			}
		}
		leaves[verb] = toolName
		if strings.TrimSpace(tool.Command.Summary) == "" {
			return &ManifestValidationError{
				Field:   field + ".summary",
				Message: "command summary is required",
			}
		}
		if _, err := projectCommandFlags(tool.InputSchema, tool.Command.Flags, field+".flags"); err != nil {
			return err
		}
	}

	if err := validateCommandLeafPrefixes(leaves); err != nil {
		return err
	}
	return validateCommandGroups(manifest.Resources.CommandGroups, leaves)
}

func validateManifestCommandResources(manifest *Manifest) error {
	if err := validateCommandDeclarations(manifest); err != nil {
		return err
	}
	return validateToolConfigs(manifest.Name, manifest.Resources.Tools)
}

func validateCommandLeafPrefixes(leaves map[string]string) error {
	for leaf, toolName := range leaves {
		prefix, nested := strings.CutSuffix(leaf, "/"+pathLeaf(leaf))
		if !nested || prefix == leaf {
			continue
		}
		if parentTool, exists := leaves[prefix]; exists {
			return &ManifestValidationError{
				Field: "resources.tools." + toolName + ".command.verb",
				Value: leaf,
				Message: fmt.Sprintf(
					"command group %q collides with executable leaf from resources.tools.%s",
					prefix,
					parentTool,
				),
			}
		}
	}
	return nil
}

func pathLeaf(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func validateCommandGroups(groups []extensioncontract.ExtensionCommandGroupSpec, leaves map[string]string) error {
	seen := make(map[string]struct{}, len(groups))
	for idx, group := range groups {
		field := fmt.Sprintf("resources.command_groups[%d].path", idx)
		path, err := validateCommandPath(field, group.Path, 1)
		if err != nil {
			return err
		}
		if _, exists := seen[path]; exists {
			return &ManifestValidationError{
				Field:   field,
				Value:   path,
				Message: "duplicate command group path",
			}
		}
		seen[path] = struct{}{}
		if strings.TrimSpace(group.Summary) == "" {
			return &ManifestValidationError{
				Field:   fmt.Sprintf("resources.command_groups[%d].summary", idx),
				Message: "command group summary is required",
			}
		}
		if toolName, exists := leaves[path]; exists {
			return &ManifestValidationError{
				Field:   field,
				Value:   path,
				Message: "command group collides with executable leaf from resources.tools." + toolName,
			}
		}
		backed := false
		for leaf := range leaves {
			if strings.HasPrefix(leaf, path+"/") {
				backed = true
				break
			}
		}
		if !backed {
			return &ManifestValidationError{
				Field:   field,
				Value:   path,
				Message: "command group has no leaf under its prefix",
			}
		}
	}
	return nil
}

func commandAmbiguityWarnings(manifest *Manifest, manifestPath string) []ValidationIssue {
	if manifest == nil {
		return nil
	}
	byFlattenedPath := make(map[string][]string)
	for _, toolName := range sortedMapKeys(manifest.Resources.Tools) {
		command := manifest.Resources.Tools[toolName].Command
		if command == nil {
			continue
		}
		verb := strings.TrimSpace(command.Verb)
		flattened := strings.ReplaceAll(verb, "/", "-")
		byFlattenedPath[flattened] = append(byFlattenedPath[flattened], verb)
	}
	warnings := make([]ValidationIssue, 0)
	for _, flattened := range sortedMapKeys(byFlattenedPath) {
		verbs := byFlattenedPath[flattened]
		slices.Sort(verbs)
		verbs = slices.Compact(verbs)
		if len(verbs) < 2 {
			continue
		}
		warnings = append(warnings, ValidationIssue{
			Path: manifestPath, Field: "resources.tools", Severity: IssueSeverityWarning,
			Message: fmt.Sprintf(
				"command paths %s are visually ambiguous when flattened as %q",
				strings.Join(verbs, ", "),
				flattened,
			),
		})
	}
	return warnings
}

func validateCommandPath(field, value string, maxDepth int) (string, error) {
	path := strings.TrimSpace(value)
	if path == "" {
		return "", &ManifestValidationError{
			Field:   field,
			Message: "command path is required",
		}
	}
	if strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") || strings.Contains(path, "//") {
		return "", &ManifestValidationError{
			Field:   field,
			Value:   path,
			Message: "command path contains an empty segment",
		}
	}
	if depth := len(strings.Split(path, "/")); depth > maxDepth {
		return "", &ManifestValidationError{
			Field: field, Value: path, Message: fmt.Sprintf("command path depth must be at most %d", maxDepth),
		}
	}
	return path, nil
}

func normalizeExtensionCommandSpec(src *toolspkg.ExtensionCommandSpec) *toolspkg.ExtensionCommandSpec {
	if src == nil {
		return nil
	}
	return &toolspkg.ExtensionCommandSpec{
		Verb: strings.TrimSpace(src.Verb), Summary: strings.TrimSpace(src.Summary),
		Example: strings.TrimSpace(src.Example), Flags: maps.Clone(src.Flags),
	}
}

func cloneExtensionCommandSpec(src *toolspkg.ExtensionCommandSpec) *toolspkg.ExtensionCommandSpec {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Flags = maps.Clone(src.Flags)
	return &cloned
}

func normalizeCommandGroups(
	src []extensioncontract.ExtensionCommandGroupSpec,
) []extensioncontract.ExtensionCommandGroupSpec {
	groups := cloneCommandGroups(src)
	for idx := range groups {
		groups[idx].Path = strings.TrimSpace(groups[idx].Path)
		groups[idx].Summary = strings.TrimSpace(groups[idx].Summary)
		groups[idx].Profile = strings.TrimSpace(groups[idx].Profile)
	}
	return groups
}

func cloneCommandGroups(
	src []extensioncontract.ExtensionCommandGroupSpec,
) []extensioncontract.ExtensionCommandGroupSpec {
	return slices.Clone(src)
}
