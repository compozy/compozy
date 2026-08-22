package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type profileListRecord struct {
	Name                   string                                  `json:"name"`
	Color                  string                                  `json:"color"`
	Icon                   *string                                 `json:"icon"`
	Emoji                  *string                                 `json:"emoji"`
	State                  string                                  `json:"state"`
	Current                bool                                    `json:"current"`
	WorkItems              int                                     `json:"work_items"`
	NeedsSetup             bool                                    `json:"needs_setup,omitempty"`
	CredentialRequirements []contract.ProfileCredentialRequirement `json:"credential_requirements,omitempty"`
}

type profileCurrentRecord struct {
	Profile   string `json:"profile"`
	Source    string `json:"source"`
	Note      string `json:"note,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

func profileListBundle(items []contract.Profile, current string) outputBundle {
	rows := make([]profileListRecord, 0, len(items))
	for _, item := range items {
		rows = append(rows, profileListRecord{
			Name: item.Name, Color: item.Color, Icon: item.Icon, Emoji: item.Emoji, State: item.State,
			Current: item.Name == current, WorkItems: item.WorkItems, NeedsSetup: item.NeedsSetup,
			CredentialRequirements: append([]contract.ProfileCredentialRequirement(nil), item.CredentialRequirements...),
		})
	}
	return outputBundle{
		jsonValue: rows,
		json:      func(cmd *cobra.Command) error { return writeJSONWithoutWorkspaceResolution(cmd, rows) },
		jsonl:     func(cmd *cobra.Command) error { return writeProfileJSONLines(cmd, rows) },
		human: func() (string, error) {
			tableRows := make([][]string, 0, len(rows))
			for _, row := range rows {
				marker := " "
				if row.Current {
					marker = "*"
				}
				symbol := "●"
				if row.Emoji != nil {
					symbol = *row.Emoji
				}
				tableRows = append(tableRows, []string{
					marker, row.Name, symbol, row.State, fmt.Sprintf("%d items", row.WorkItems),
				})
			}
			return renderHumanTable("", []string{"", "NAME", "SYMBOL", "STATE", "WORK"}, tableRows), nil
		},
		toon: func() (string, error) {
			toonRows := make([][]string, 0, len(rows))
			for _, row := range rows {
				toonRows = append(toonRows, []string{
					row.Name, row.Color, row.State, strconv.FormatBool(row.Current), strconv.Itoa(row.WorkItems),
				})
			}
			return renderToonArray("profiles", []string{"name", "color", "state", "current", "work_items"}, toonRows), nil
		},
	}
}

func profileCurrentBundle(resolution profileResolution) outputBundle {
	workspace := resolution.WorkspaceName
	if workspace == "" {
		workspace = resolution.WorkspaceID
	}
	record := profileCurrentRecord{
		Profile: resolution.Profile.Name, Source: resolution.Source, Note: resolution.Note, Workspace: workspace,
	}
	return outputBundle{
		jsonValue: record,
		json:      func(cmd *cobra.Command) error { return writeJSONWithoutWorkspaceResolution(cmd, record) },
		jsonl:     func(cmd *cobra.Command) error { _, err := writeProfileResolutionFrame(cmd); return err },
		human: func() (string, error) {
			source := resolution.Source
			if source == profileResolutionRemembered && workspace != "" {
				source = "remembered choice of workspace " + workspace
			}
			if resolution.Note != "" {
				source += "; " + resolution.Note
			}
			return fmt.Sprintf("%s (%s)", resolution.Profile.Name, source), nil
		},
		toon: func() (string, error) {
			return renderToonObject("profile", []string{"profile", "source", "note", "workspace"}, []string{
				record.Profile, record.Source, record.Note, record.Workspace,
			}), nil
		},
	}
}

func profileUseBundle(selection contract.ProfileSelection, workspace workspaceResolution, hasWorkspace bool) outputBundle {
	message := "Active global profile: " + selection.Profile + "."
	if hasWorkspace {
		name := workspace.Detail.Workspace.Name
		if name == "" {
			name = workspace.ID
		}
		message = "Active profile for workspace " + name + ": " + selection.Profile + "."
	}
	return simpleProfileBundle(selection, message)
}

func profileMutationBundle(profile contract.Profile, message string) outputBundle {
	return simpleProfileBundle(profile, message)
}

func simpleProfileBundle(value any, message string) outputBundle {
	return outputBundle{
		jsonValue: value,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, value) },
		human:     func() (string, error) { return message, nil },
		toon:      func() (string, error) { return renderToonObject("result", []string{"message"}, []string{message}), nil },
	}
}

func profileRenameBundle(oldName, newName string, plan contract.RenameProfilePlan, result contract.RenameProfileResponse) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, result) },
		human: func() (string, error) {
			lines := []string{fmt.Sprintf("Renamed %s → %s. Work, config, and credentials follow the profile.", oldName, newName)}
			renamed := make([]string, 0)
			for _, item := range result.RepoResults {
				if item.Renamed {
					renamed = append(renamed, item.WorkspaceID)
				}
			}
			if len(renamed) > 0 {
				lines = append(lines, "Repo folders renamed in "+strings.Join(renamed, ", ")+" — commit those changes when ready.")
			} else if len(plan.RepoCandidates) > 0 {
				lines = append(lines, fmt.Sprintf("Repo folders matching %q found in %d workspaces; left dormant.", oldName, len(plan.RepoCandidates)))
			}
			return strings.Join(lines, "\n"), nil
		},
		toon: func() (string, error) {
			return renderToonObject("profile_rename", []string{"old_name", "new_name", "renamed"}, []string{
				oldName, newName, strconv.FormatBool(result.Renamed),
			}), nil
		},
	}
}

func profileArchiveBundle(name string, result contract.ArchiveProfileResponse) outputBundle {
	message := "Archived " + name + ". Its work stays visible under All profiles."
	if len(result.PausedAutomations) > 0 {
		message += fmt.Sprintf("\nPaused automations (%d): %s.", len(result.PausedAutomations), strings.Join(result.PausedAutomations, ", "))
	}
	if result.FrozenQueuedRuns > 0 {
		message += fmt.Sprintf("\nQueued work frozen with the profile: %d runs.", result.FrozenQueuedRuns)
	}
	return simpleProfileBundle(result, message)
}

func profileUnarchiveBundle(name string, result contract.UnarchiveProfileResponse) outputBundle {
	message := "Unarchived " + name + "."
	if len(result.PausedAutomations) > 0 {
		message += fmt.Sprintf(" Paused automations (%d) stay paused: %s.", len(result.PausedAutomations), strings.Join(result.PausedAutomations, ", "))
	}
	return simpleProfileBundle(result, message)
}

func profileDeleteBundle(name string, result contract.DeleteProfileResponse) outputBundle {
	return simpleProfileBundle(result, "Deleted "+name+". The name is free.")
}

func profileOperationsBundle(items []contract.ProfileOperation) outputBundle {
	return listBundle(items, items, "", []string{"ID", "KIND", "PROFILE", "STATUS", "STEP", "ERROR"},
		"profile_operations", []string{"id", "kind", "profile", "status", "step", "error"},
		func(item contract.ProfileOperation) []string {
			return []string{item.ID, item.Kind, item.Profile, item.Status, item.Step, item.Error}
		},
		func(item contract.ProfileOperation) []string {
			return []string{item.ID, item.Kind, item.Profile, item.Status, item.Step, item.Error}
		},
	)
}

func profileOperationBundle(item contract.ProfileOperation) outputBundle {
	return simpleProfileBundle(item, fmt.Sprintf("Profile operation %s: %s.", item.ID, item.Status))
}

func renderProfileDeletePreview(name string, removed contract.ProfileRemovalSummary) string {
	return fmt.Sprintf(
		"%s owns no work. This removes permanently:\n  agents: %d   skills: %d   loops: %d   mcp servers: %d\n  config overrides: %d keys   credential overrides: %d   memory: %d entries   desktops: %d saved arrangements",
		name, removed.Agents, removed.Skills, removed.Loops, removed.MCPServers,
		removed.ConfigKeys, removed.CredentialOverrides, removed.MemoryEntries, removed.DesktopPartitions,
	)
}
