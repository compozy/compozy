package cli

import (
	"strconv"
	"strings"
)

func workspaceRecordBundle(item WorkspaceRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection(authoredContextWorkspaceValue, []keyValue{
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: workspaceNameValue, Value: stringOrDash(item.Name)},
				{Label: workspaceRootValue, Value: stringOrDash(item.RootDir)},
				{Label: "Additional Dirs", Value: stringOrDash(strings.Join(item.AddDirs, ", "))},
				{Label: "Default Agent", Value: stringOrDash(item.DefaultAgent)},
				{Label: workspaceSandboxValue, Value: stringOrDash(item.SandboxRef)},
				{Label: workspaceCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
				{Label: workspaceUpdatedValue, Value: stringOrDash(formatTime(item.UpdatedAt))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(workspaceSkillSource, []string{
				"id",
				automationNameKey,
				"root_dir",
				"add_dirs",
				"default_agent",
				"sandbox_ref",
				workspaceCreatedAtKey,
				automationUpdatedAtKey,
			}, []string{
				item.ID,
				item.Name,
				item.RootDir,
				strings.Join(item.AddDirs, "|"),
				item.DefaultAgent,
				item.SandboxRef,
				formatTime(item.CreatedAt),
				formatTime(item.UpdatedAt),
			}), nil
		},
	}
}

func workspaceListBundle(items []WorkspaceRecord) outputBundle {
	return listBundle(
		items,
		items,
		"Workspaces",
		[]string{
			"ID",
			workspaceNameValue,
			workspaceRootValue,
			"Add Dirs",
			"Default Agent",
			workspaceSandboxValue,
			workspaceUpdatedValue,
		},
		"workspaces",
		[]string{
			"id",
			automationNameKey,
			"root_dir",
			"add_dir_count",
			"default_agent",
			"sandbox_ref",
			automationUpdatedAtKey,
		},
		func(item WorkspaceRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.Name),
				stringOrDash(item.RootDir),
				strconv.Itoa(len(item.AddDirs)),
				stringOrDash(item.DefaultAgent),
				stringOrDash(item.SandboxRef),
				stringOrDash(formatTime(item.UpdatedAt)),
			}
		},
		func(item WorkspaceRecord) []string {
			return []string{
				item.ID,
				item.Name,
				item.RootDir,
				strconv.Itoa(len(item.AddDirs)),
				item.DefaultAgent,
				item.SandboxRef,
				formatTime(item.UpdatedAt),
			}
		},
	)
}

type workspaceDetailOutput struct {
	WorkspaceDetailRecord
	ResolutionSource string `json:"resolution_source,omitempty"`
}

func workspaceDetailBundle(detail WorkspaceDetailRecord, resolutionSource ...string) outputBundle {
	jsonValue := any(detail)
	if len(resolutionSource) > 0 && strings.TrimSpace(resolutionSource[0]) != "" {
		jsonValue = workspaceDetailOutput{
			WorkspaceDetailRecord: detail,
			ResolutionSource:      strings.TrimSpace(resolutionSource[0]),
		}
	}
	return outputBundle{
		jsonValue: jsonValue,
		human:     func() (string, error) { return renderWorkspaceDetailHuman(detail) },
		toon:      func() (string, error) { return renderWorkspaceDetailToon(detail) },
	}
}
