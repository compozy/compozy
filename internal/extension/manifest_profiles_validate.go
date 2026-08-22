package extensionpkg

import (
	"fmt"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/vault"
)

type manifestPlacement struct {
	field string
	name  string
}

func validateManifestProfiles(manifest *Manifest) error {
	seen := make(map[string]struct{}, len(manifest.Profiles))
	for index, declaration := range manifest.Profiles {
		field := fmt.Sprintf("profiles[%d]", index)
		name, err := profilepkg.NormalizeName(declaration.Name)
		if err != nil {
			return &ManifestValidationError{Field: field + ".name", Value: declaration.Name, Message: err.Error()}
		}
		if _, duplicate := seen[name]; duplicate {
			return &ManifestValidationError{Field: field + ".name", Value: name, Message: "duplicate declared profile"}
		}
		seen[name] = struct{}{}
		if _, _, _, err := profilepkg.NormalizeIdentity(
			declaration.Color, declaration.Icon, declaration.Emoji,
		); err != nil {
			return &ManifestValidationError{Field: field, Message: err.Error()}
		}
		credentialSeen := make(map[string]struct{}, len(declaration.Credentials))
		for credentialIndex, credential := range declaration.Credentials {
			credentialField := fmt.Sprintf("%s.credentials[%d]", field, credentialIndex)
			ref := fmt.Sprintf(
				"vault:profiles/%s/providers/%s/%s",
				name,
				strings.TrimSpace(credential.Provider),
				strings.TrimSpace(credential.Slot),
			)
			if _, err := vault.ParseProfileSecretRef(ref); err != nil {
				return &ManifestValidationError{Field: credentialField, Message: err.Error()}
			}
			key := credential.Provider + "\x00" + credential.Slot
			if _, duplicate := credentialSeen[key]; duplicate {
				return &ManifestValidationError{Field: credentialField, Message: "duplicate credential requirement"}
			}
			credentialSeen[key] = struct{}{}
		}
	}
	return validateManifestPlacements(manifest)
}

func validateManifestPlacements(manifest *Manifest) error {
	for _, placement := range manifestPlacements(manifest) {
		name := strings.TrimSpace(placement.name)
		if name == "" || name == "default" {
			continue
		}
		if _, err := profilepkg.NormalizeName(name); err != nil {
			return &ManifestValidationError{Field: placement.field, Value: name, Message: err.Error()}
		}
	}
	return nil
}

func manifestPlacements(manifest *Manifest) []manifestPlacement {
	placements := make([]manifestPlacement, 0)
	appendPaths := func(field string, values []ManifestResourcePath) {
		for index, value := range values {
			placements = append(placements, manifestPlacement{
				field: fmt.Sprintf("resources.%s[%d].profile", field, index), name: value.Profile,
			})
		}
	}
	appendPaths("skills", manifest.Resources.Skills)
	appendPaths("loops", manifest.Resources.Loops)
	appendPaths("agents", manifest.Resources.Agents)
	appendPaths("automation", manifest.Resources.Automation)
	appendPaths("layouts", manifest.Resources.Layouts)
	for index, hook := range manifest.Resources.Hooks {
		placements = append(placements, manifestPlacement{field: fmt.Sprintf("resources.hooks[%d].profile", index), name: hook.Profile})
	}
	for name, tool := range manifest.Resources.Tools {
		placements = append(placements, manifestPlacement{field: "resources.tools." + name + ".profile", name: tool.Profile})
	}
	for name, server := range manifest.Resources.MCPServers {
		placements = append(placements, manifestPlacement{field: "resources.mcp_servers." + name + ".profile", name: server.Profile})
	}
	for index, group := range manifest.Resources.CommandGroups {
		placements = append(placements, manifestPlacement{field: fmt.Sprintf("resources.command_groups[%d].profile", index), name: group.Profile})
	}
	for index, command := range manifest.Resources.CmdPalette.Commands {
		placements = append(placements, manifestPlacement{field: fmt.Sprintf("resources.cmd_palette.commands[%d].profile", index), name: command.Profile})
	}
	for index, view := range manifest.Resources.CmdPalette.Views {
		placements = append(placements, manifestPlacement{field: fmt.Sprintf("resources.cmd_palette.views[%d].profile", index), name: view.Profile})
	}
	return placements
}
