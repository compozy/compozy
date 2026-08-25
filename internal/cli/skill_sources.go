package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const skillSourcesCommandName = "sources"

type skillSourcesRecord struct {
	Scope       string                                          `json:"scope"`
	WorkspaceID string                                          `json:"workspace_id,omitempty"`
	Sources     []contract.SettingsSkillSourcePayload           `json:"sources"`
	Inherits    *contract.SettingsSkillSourceInheritancePayload `json:"inherits,omitempty"`
	workspace   string
}

func newSkillSourcesCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:   skillSourcesCommandName,
		Short: "Show configured skill sources and discovery diagnostics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			record, err := readSkillSources(cmd.Context(), cmd, deps, workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, skillSourcesBundle(record))
		},
	}
	cmd.Flags().StringVar(
		&workspace,
		workspaceSkillSource,
		"",
		"Override workspace context (ID, name, or path)",
	)
	return cmd
}

func readSkillSources(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (skillSourcesRecord, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return skillSourcesRecord{}, err
	}
	reader, ok := client.(settingsSkillsReader)
	if !ok {
		return skillSourcesRecord{}, errors.New("cli: daemon client does not support skill source diagnostics")
	}
	query := settingsSkillsScopeQuery{}
	record := skillSourcesRecord{Scope: string(contract.SettingsScopeUser)}
	if workspaceRef != "" {
		resolution, resolveErr := resolveWorkspaceCandidate(ctx, cmd, client, workspaceRef, workspaceResolutionFlag)
		if resolveErr != nil {
			return skillSourcesRecord{}, resolveErr
		}
		query.Scope = contract.SettingsScopeWorkspace
		query.WorkspaceID = resolution.ID
		record.Scope = string(contract.SettingsScopeWorkspace)
		record.WorkspaceID = resolution.ID
		record.workspace = resolution.Detail.Workspace.Name
		if strings.TrimSpace(record.workspace) == "" {
			record.workspace = workspaceRef
		}
	}
	response, err := reader.GetSettingsSkills(ctx, query)
	if err != nil {
		return skillSourcesRecord{}, fmt.Errorf("cli: read skill sources: %w", err)
	}
	record.Sources = append([]contract.SettingsSkillSourcePayload(nil), response.Sources...)
	record.Inherits = response.Inherits
	return record, nil
}

func skillSourcesBundle(record skillSourcesRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanTable("", []string{
					"SOURCE", "STATE", "WORKSPACE PATH", "GLOBAL PATH", "SKILLS", "NOTES",
				}, skillSourceRows(record.Sources)),
				skillSourcesFooter(record),
			), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(
				renderToonArray("sources", []string{
					"source", "state", "workspace_path", "global_path", "skills", "notes",
				}, skillSourceRows(record.Sources)),
				skillSourcesFooter(record),
			), nil
		},
	}
}

func skillSourceRows(sources []contract.SettingsSkillSourcePayload) [][]string {
	rows := make([][]string, 0, len(sources))
	for _, source := range sources {
		globalPath := source.GlobalPath
		if globalPath == "" {
			globalPath = source.Path
		}
		rows = append(rows, []string{
			source.Slug,
			skillSourceState(source),
			stringOrDash(source.WorkspacePath),
			stringOrDash(globalPath),
			skillSourceCount(source.Roots),
			stringOrDash(strings.Join(skillSourceNotes(source.Roots), ", ")),
		})
	}
	return rows
}

func skillSourceState(source contract.SettingsSkillSourcePayload) string {
	if source.AlwaysOn {
		return "always on"
	}
	if source.Enabled {
		return "enabled"
	}
	return "disabled"
}

func skillSourceCount(roots []contract.SettingsSkillSourceRootPayload) string {
	total := 0
	measured := false
	for _, root := range roots {
		if root.SkillCount == nil {
			continue
		}
		measured = true
		total += *root.SkillCount
	}
	if !measured {
		return "—"
	}
	return strconv.Itoa(total)
}

func skillSourceNotes(roots []contract.SettingsSkillSourceRootPayload) []string {
	notes := make([]string, 0, 5)
	for _, root := range roots {
		switch {
		case !root.Exists:
			notes = appendUniqueString(notes, "absent")
		case !root.Readable:
			notes = appendUniqueString(notes, "unreadable")
		}
		if root.Truncated {
			notes = appendUniqueString(notes, "truncated")
		}
		if len(root.SkippedLinks) > 0 {
			notes = appendUniqueString(notes, "links skipped")
		}
		if len(root.Collisions) > 0 {
			notes = appendUniqueString(notes, "collisions")
		}
	}
	return notes
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func skillSourcesFooter(record skillSourcesRecord) string {
	if record.Scope == string(contract.SettingsScopeUser) {
		return "scope: user · overrides: none"
	}
	overrides, inherits := skillSourceInheritanceLabels(record.Inherits)
	return fmt.Sprintf(
		"scope: workspace (%s) · overrides: %s · inherits: %s",
		stringOrDash(record.workspace), strings.Join(overrides, ", "), strings.Join(inherits, ", "),
	)
}

func skillSourceInheritanceLabels(
	value *contract.SettingsSkillSourceInheritancePayload,
) ([]string, []string) {
	if value == nil {
		return []string{"none"}, []string{"none"}
	}
	overrides := make([]string, 0, 2)
	inherits := make([]string, 0, 2)
	for _, item := range []struct {
		name      string
		inherited bool
	}{{"sources", value.Sources}, {"custom_sources", value.CustomSources}} {
		if item.inherited {
			inherits = append(inherits, item.name)
		} else {
			overrides = append(overrides, item.name)
		}
	}
	if len(overrides) == 0 {
		overrides = append(overrides, "none")
	}
	if len(inherits) == 0 {
		inherits = append(inherits, "none")
	}
	return overrides, inherits
}
