package extensionpkg

import (
	"fmt"
	"slices"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// CommandDescriptor is one executable command leaf projected from an active extension tool.
type CommandDescriptor struct {
	Extension        string                          `json:"extension"`
	Verb             string                          `json:"verb"`
	ToolID           string                          `json:"tool_id"`
	Summary          string                          `json:"summary"`
	Example          string                          `json:"example,omitempty"`
	Flags            []extensioncontract.CommandFlag `json:"flags"`
	RiskClass        toolspkg.RiskClass              `json:"risk_class"`
	ApprovalRequired bool                            `json:"approval_required"`
}

// CommandGroupDescriptor associates one presentation-only group with its extension.
type CommandGroupDescriptor struct {
	Extension string `json:"extension"`
	Path      string `json:"path"`
	Summary   string `json:"summary"`
}

// Commands returns the storage-free command projection for the effective workspace extension set.
func (m *Manager) Commands(workspaceID string) ([]CommandDescriptor, []CommandGroupDescriptor, error) {
	if m == nil {
		return nil, nil, ErrManagerRequired
	}
	workspaceID = strings.TrimSpace(workspaceID)
	commands := make([]CommandDescriptor, 0)
	groups := make([]CommandGroupDescriptor, 0)
	for _, info := range m.ListForWorkspace(workspaceID) {
		if !info.Enabled {
			continue
		}
		extension, err := m.GetForInstance(InstanceKey{Name: info.Name, WorkspaceID: workspaceID})
		if err != nil {
			return nil, nil, fmt.Errorf("extension: resolve command source %q: %w", info.Name, err)
		}
		if !commandExtensionAvailable(extension) {
			continue
		}
		extensionCommands, extensionGroups, err := projectExtensionCommands(extension.Manifest)
		if err != nil {
			return nil, nil, fmt.Errorf("extension: project commands for %q: %w", info.Name, err)
		}
		commands = append(commands, extensionCommands...)
		groups = append(groups, extensionGroups...)
	}
	slices.SortFunc(commands, compareCommandDescriptors)
	slices.SortFunc(groups, compareCommandGroupDescriptors)
	return commands, groups, nil
}

func commandExtensionAvailable(extension *Extension) bool {
	return extension != nil && extension.Info.Enabled && extension.Manifest != nil &&
		extension.Status.Registered && extension.Status.Active && extension.Status.Healthy
}

func projectExtensionCommands(
	manifest *Manifest,
) ([]CommandDescriptor, []CommandGroupDescriptor, error) {
	if manifest == nil {
		return nil, nil, nil
	}
	if err := validateCommandDeclarations(manifest); err != nil {
		return nil, nil, err
	}
	toolDescriptors, err := ResolveManifestToolDescriptors(manifest)
	if err != nil {
		return nil, nil, err
	}
	commands := make([]CommandDescriptor, 0, len(toolDescriptors))
	for index := range toolDescriptors {
		descriptor := &toolDescriptors[index]
		command := descriptor.RuntimeDescriptor.Command
		if command == nil {
			continue
		}
		flags, err := projectCommandFlags(
			descriptor.Tool.InputSchema,
			command.Flags,
			"resources.tools."+descriptor.Name+".command.flags",
		)
		if err != nil {
			return nil, nil, err
		}
		commands = append(commands, CommandDescriptor{
			Extension:        manifest.Name,
			Verb:             strings.TrimSpace(command.Verb),
			ToolID:           descriptor.Tool.ID.String(),
			Summary:          strings.TrimSpace(command.Summary),
			Example:          strings.TrimSpace(command.Example),
			Flags:            flags,
			RiskClass:        descriptor.Tool.Risk,
			ApprovalRequired: descriptor.Tool.RequiresInteraction,
		})
	}
	groups := projectCommandGroups(manifest.Name, manifest.Resources.CommandGroups, commands)
	return commands, groups, nil
}

func projectCommandGroups(
	extensionName string,
	declared []extensioncontract.ExtensionCommandGroupSpec,
	commands []CommandDescriptor,
) []CommandGroupDescriptor {
	byPath := make(map[string]CommandGroupDescriptor, len(declared))
	for _, group := range declared {
		path := strings.TrimSpace(group.Path)
		byPath[path] = CommandGroupDescriptor{
			Extension: extensionName, Path: path, Summary: strings.TrimSpace(group.Summary),
		}
	}
	for _, command := range commands {
		parts := strings.Split(command.Verb, "/")
		if len(parts) != 2 {
			continue
		}
		if _, declared := byPath[parts[0]]; !declared {
			byPath[parts[0]] = CommandGroupDescriptor{Extension: extensionName, Path: parts[0]}
		}
	}
	groups := make([]CommandGroupDescriptor, 0, len(byPath))
	for _, group := range byPath {
		groups = append(groups, group)
	}
	return groups
}

func compareCommandDescriptors(left, right CommandDescriptor) int {
	if compared := strings.Compare(left.Extension, right.Extension); compared != 0 {
		return compared
	}
	return strings.Compare(left.Verb, right.Verb)
}

func compareCommandGroupDescriptors(left, right CommandGroupDescriptor) int {
	if compared := strings.Compare(left.Extension, right.Extension); compared != 0 {
		return compared
	}
	return strings.Compare(left.Path, right.Path)
}
