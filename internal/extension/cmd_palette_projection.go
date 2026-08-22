package extensionpkg

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

const (
	CmdPaletteSourceHealthy   = "healthy"
	CmdPaletteSourceUnhealthy = "unhealthy"
	CmdPaletteSourceDisabled  = "disabled"
)

// CmdPaletteProjection is the complete storage-free palette snapshot for one workspace.
type CmdPaletteProjection struct {
	Commands []CmdPaletteProjectedCommand
	Views    []CmdPaletteProjectedView
	Defaults []CmdPaletteDefaultShortcut
	Sources  []CmdPaletteSourceStatus
}

// CmdPaletteProjectedCommand is a validated, namespaced extension command.
type CmdPaletteProjectedCommand struct {
	ID                string
	Title             string
	Section           string
	Icon              string
	Keywords          []string
	Arguments         []CmdPaletteArgument
	Action            CmdPaletteAction
	Destructive       bool
	Confirmation      *CmdPaletteConfirmation
	Execution         CmdPaletteResolvedExecutionPolicy
	Extension         string
	UnavailableReason string
}

// CmdPaletteResolvedExecutionPolicy is the effective policy after action defaults.
type CmdPaletteResolvedExecutionPolicy struct {
	SingleFlight bool
	RetrySafe    bool
}

// CmdPaletteProjectedView is a validated, namespaced extension view.
type CmdPaletteProjectedView struct {
	ID                string
	Title             string
	Kind              string
	SourceTool        string
	Program           bool
	Extension         string
	UnavailableReason string
}

// CmdPaletteDefaultShortcut is one extension default in deterministic enable order.
type CmdPaletteDefaultShortcut struct {
	CommandID string
	Chord     string
	Extension string
	Active    bool
}

// CmdPaletteSourceStatus keeps source membership separate from runtime health.
type CmdPaletteSourceStatus struct {
	Source string
	Status string
	Reason string
}

type cmdPaletteInstance struct {
	extension *managedExtension
	order     time.Time
	enabled   bool
}

// CmdPalette returns one atomic projection for an explicit profile lens.
func (m *Manager) CmdPalette(workspaceID string, profile ProfileLens) (CmdPaletteProjection, error) {
	return m.cmdPaletteProjection(workspaceID, profile, false)
}

// CmdPaletteSettings returns installed contributions for operator inspection,
// including disabled extensions that do not belong to the live catalog.
func (m *Manager) CmdPaletteSettings(workspaceID string, profile ProfileLens) (CmdPaletteProjection, error) {
	return m.cmdPaletteProjection(workspaceID, profile, true)
}

func (m *Manager) cmdPaletteProjection(
	workspaceID string,
	profile ProfileLens,
	includeDisabled bool,
) (CmdPaletteProjection, error) {
	if m == nil {
		return CmdPaletteProjection{}, ErrManagerRequired
	}
	workspaceID = strings.TrimSpace(workspaceID)
	profile = profile.normalize()
	if !profile.valid() {
		return CmdPaletteProjection{}, fmt.Errorf("extension: command palette profile id and name are required")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances, err := m.cmdPaletteInstancesLocked(workspaceID, profile, includeDisabled)
	if err != nil {
		return CmdPaletteProjection{}, err
	}
	projection := CmdPaletteProjection{
		Commands: make([]CmdPaletteProjectedCommand, 0),
		Views:    make([]CmdPaletteProjectedView, 0),
		Defaults: make([]CmdPaletteDefaultShortcut, 0),
		Sources:  make([]CmdPaletteSourceStatus, 0),
	}
	for _, instance := range instances {
		if err := m.projectCmdPaletteInstanceLocked(
			&projection,
			instance.extension,
			profile.Name,
			instance.enabled,
		); err != nil {
			return CmdPaletteProjection{}, err
		}
	}
	return projection, nil
}

func (m *Manager) cmdPaletteInstancesLocked(
	workspaceID string,
	profile ProfileLens,
	includeDisabled bool,
) ([]cmdPaletteInstance, error) {
	byName := make(map[string]*managedExtension, len(m.extensions)+len(m.devExtensions))
	maps.Copy(byName, m.extensions)
	if workspaceID != "" {
		for key, extension := range m.devExtensions {
			if key.WorkspaceID == workspaceID {
				byName[key.Name] = extension
			}
		}
	}
	instances := make([]cmdPaletteInstance, 0, len(byName))
	for _, extension := range byName {
		if extension == nil || extension.manifest == nil {
			continue
		}
		enabled := extension.info.Enabled
		if m.registry != nil {
			var err error
			enabled, err = m.registry.IsEnabledForProfile(extension.info.Name, profile.ID)
			if err != nil {
				return nil, fmt.Errorf(
					"extension: resolve %q enablement for profile %q: %w",
					extension.info.Name,
					profile.Name,
					err,
				)
			}
		}
		if !includeDisabled && !enabled {
			continue
		}
		palette := extension.manifest.Resources.CmdPalette
		if len(palette.Commands) == 0 && len(palette.Views) == 0 {
			continue
		}
		instances = append(instances, cmdPaletteInstance{
			extension: extension,
			order:     extension.info.InstalledAt,
			enabled:   enabled,
		})
	}
	slices.SortFunc(instances, compareCmdPaletteInstances)
	return instances, nil
}

func compareCmdPaletteInstances(left, right cmdPaletteInstance) int {
	if left.order.IsZero() != right.order.IsZero() {
		if left.order.IsZero() {
			return 1
		}
		return -1
	}
	if compared := left.order.Compare(right.order); compared != 0 {
		return compared
	}
	return strings.Compare(left.extension.info.Name, right.extension.info.Name)
}

func (m *Manager) projectCmdPaletteInstanceLocked(
	projection *CmdPaletteProjection,
	extension *managedExtension,
	profileName string,
	enabled bool,
) error {
	manifest := extension.manifest
	tools, err := validateManifestCmdPalette(manifest)
	if err != nil {
		return fmt.Errorf("extension: project command palette for %q: %w", manifest.Name, err)
	}
	status := m.statusLocked(extension)
	health, reason := cmdPaletteExtensionHealth(status, enabled)
	sourceID := "ext." + manifest.Name
	contributed := false
	for _, command := range manifest.Resources.CmdPalette.Commands {
		if !cmdPaletteCommandVisible(manifest, command, profileName) {
			continue
		}
		projected, projectErr := projectCmdPaletteCommand(manifest.Name, command, tools, reason)
		if projectErr != nil {
			return projectErr
		}
		contributed = true
		projection.Commands = append(projection.Commands, projected)
		if command.DefaultShortcut != "" {
			projection.Defaults = append(projection.Defaults, CmdPaletteDefaultShortcut{
				CommandID: projected.ID, Chord: command.DefaultShortcut,
				Extension: manifest.Name, Active: health == CmdPaletteSourceHealthy,
			})
		}
	}
	for _, view := range manifest.Resources.CmdPalette.Views {
		if !cmdPaletteViewVisible(manifest, view, profileName) {
			continue
		}
		contributed = true
		projection.Views = append(projection.Views, projectCmdPaletteView(manifest.Name, view, tools, reason))
	}
	if contributed {
		projection.Sources = append(projection.Sources, CmdPaletteSourceStatus{
			Source: sourceID, Status: health, Reason: reason,
		})
	}
	return nil
}

func cmdPaletteCommandVisible(manifest *Manifest, command CmdPaletteCommand, profileName string) bool {
	if !manifestPlacementVisible(command.Profile, profileName) {
		return false
	}
	if command.Action.Kind != cmdPaletteActionTool {
		return true
	}
	tool, exists := manifest.Resources.Tools[command.Action.Tool]
	return exists && manifestPlacementVisible(tool.Profile, profileName)
}

func cmdPaletteViewVisible(manifest *Manifest, view CmdPaletteView, profileName string) bool {
	if !manifestPlacementVisible(view.Profile, profileName) {
		return false
	}
	if view.Source == nil {
		return true
	}
	tool, exists := manifest.Resources.Tools[view.Source.Tool]
	return exists && manifestPlacementVisible(tool.Profile, profileName)
}

func projectCmdPaletteCommand(
	extensionName string,
	command CmdPaletteCommand,
	tools map[string]ManifestToolDescriptor,
	unavailableReason string,
) (CmdPaletteProjectedCommand, error) {
	action := command.Action
	switch action.Kind {
	case cmdPaletteActionTool:
		descriptor, exists := tools[action.Tool]
		if !exists {
			return CmdPaletteProjectedCommand{}, cmdPaletteUnknownRef("action.tool", "tool", action.Tool)
		}
		action.Tool = descriptor.Tool.ID.String()
	case cmdPaletteActionView:
		action.View = cmdPaletteNamespacedID(extensionName, action.View)
	}
	if action.Args != nil {
		cloned := make(map[string]any, len(action.Args))
		maps.Copy(cloned, action.Args)
		action.Args = cloned
	}
	section := command.Section
	if section == "" {
		section = extensionName
	}
	return CmdPaletteProjectedCommand{
		ID: cmdPaletteNamespacedID(extensionName, command.ID), Title: command.Title,
		Section: section, Icon: command.Icon, Keywords: slices.Clone(command.Keywords),
		Arguments: cloneCmdPaletteArguments(command.Arguments), Action: action,
		Destructive: command.Destructive, Confirmation: cloneCmdPaletteConfirmation(command.Confirmation),
		Execution: resolveCmdPaletteExecution(command), Extension: extensionName,
		UnavailableReason: unavailableReason,
	}, nil
}

func projectCmdPaletteView(
	extensionName string,
	view CmdPaletteView,
	tools map[string]ManifestToolDescriptor,
	unavailableReason string,
) CmdPaletteProjectedView {
	projected := CmdPaletteProjectedView{
		ID: cmdPaletteNamespacedID(extensionName, view.ID), Title: view.Title, Kind: view.Kind,
		Program: view.Program, Extension: extensionName, UnavailableReason: unavailableReason,
	}
	if view.Source != nil {
		projected.SourceTool = tools[view.Source.Tool].Tool.ID.String()
	}
	return projected
}

func cmdPaletteNamespacedID(extensionName, localID string) string {
	return "ext." + strings.TrimSpace(extensionName) + "." + strings.TrimSpace(localID)
}

func cmdPaletteExtensionHealth(status ExtensionStatus, enabled bool) (string, string) {
	if !enabled {
		return CmdPaletteSourceDisabled, fmt.Sprintf("extension %s is disabled", status.Name)
	}
	if status.Registered && status.Active && status.Healthy {
		return CmdPaletteSourceHealthy, ""
	}
	detail := strings.TrimSpace(status.HealthMessage)
	if detail == "" {
		detail = strings.TrimSpace(status.LastError)
	}
	if detail == "" {
		detail = strings.TrimSpace(status.FailureCode)
	}
	if detail == "" {
		detail = "runtime unavailable"
	}
	return CmdPaletteSourceUnhealthy, fmt.Sprintf("extension %s is unhealthy (%s)", status.Name, detail)
}

func resolveCmdPaletteExecution(command CmdPaletteCommand) CmdPaletteResolvedExecutionPolicy {
	policy := CmdPaletteResolvedExecutionPolicy{
		SingleFlight: command.Action.Kind == cmdPaletteActionTool || command.Destructive,
		RetrySafe:    command.Action.Kind != cmdPaletteActionTool && !command.Destructive,
	}
	if command.Execution == nil {
		return policy
	}
	if command.Execution.SingleFlight != nil {
		policy.SingleFlight = *command.Execution.SingleFlight
	}
	if command.Execution.RetrySafe != nil {
		policy.RetrySafe = *command.Execution.RetrySafe
	}
	return policy
}

func cloneCmdPaletteArguments(source []CmdPaletteArgument) []CmdPaletteArgument {
	result := slices.Clone(source)
	for index := range result {
		result[index].Options = slices.Clone(source[index].Options)
	}
	return result
}

func cloneCmdPaletteConfirmation(source *CmdPaletteConfirmation) *CmdPaletteConfirmation {
	if source == nil {
		return nil
	}
	cloned := *source
	return &cloned
}
