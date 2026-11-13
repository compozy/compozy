package daemon

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
)

const nativeToolsCommandsKey = "commands"

type compactClientsResult struct {
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    windowmanager.Revision    `json:"revision"`
	Clients     []compactClient           `json:"clients"`
}

var desktopClientsViews = resultViews[windowManagerClientsResult]{
	Compact: func(p windowManagerClientsResult) any {
		var rows []compactClient
		if p.Clients != nil {
			rows = make([]compactClient, len(p.Clients))
			for i, v := range p.Clients {
				rows[i] = projectCompactClient(v)
			}
		}
		return compactClientsResult{p.WorkspaceID, p.Revision, rows}
	},
	Preview: func(p windowManagerClientsResult) string { return fmt.Sprintf("%d clients", len(p.Clients)) },
}

type compactLayoutResult struct {
	WorkspaceID windowmanager.WorkspaceID `json:"workspace_id"`
	Revision    windowmanager.Revision    `json:"revision"`
	Snapshot    compactSnapshot           `json:"snapshot"`
}

var layoutGetViews = resultViews[windowManagerSnapshotResult]{
	Compact: func(p windowManagerSnapshotResult) any {
		return compactLayoutResult{p.WorkspaceID, p.Revision, projectCompactSnapshot(p.Snapshot)}
	},
	Preview: func(p windowManagerSnapshotResult) string {
		return fmt.Sprintf("read layout at revision %d", p.Revision)
	},
}

type compactPreviewResult struct {
	Deprecations []windowmanager.Deprecation `json:"deprecations"`
	WorkspaceID  windowmanager.WorkspaceID   `json:"workspace_id"`
	CommandID    windowmanager.CommandID     `json:"command_id"`
	Revision     windowmanager.Revision      `json:"revision"`
	Changed      bool                        `json:"changed"`
	Changes      windowmanager.ChangeSet     `json:"changes"`
	Diagnostics  []windowmanager.Diagnostic  `json:"diagnostics"`
	Client       *windowmanager.ClientView   `json:"client,omitzero"`
	Snapshot     compactSnapshot             `json:"snapshot"`
}

var layoutPreviewViews = resultViews[windowManagerPreviewResult]{
	Compact: func(p windowManagerPreviewResult) any {
		return compactPreviewResult{
			p.Deprecations,
			p.WorkspaceID,
			p.CommandID,
			p.Revision,
			p.Changed,
			p.Changes,
			p.Diagnostics,
			p.Client,
			projectCompactSnapshot(p.Snapshot),
		}
	},
	Preview: func(p windowManagerPreviewResult) string {
		return fmt.Sprintf("previewed %s at revision %d", p.CommandID, p.Revision)
	},
}
var paletteListViews = resultViews[[]contract.CmdPaletteCommand]{
	Compact: func(p []contract.CmdPaletteCommand) any {
		var rows []compactPaletteCommand
		if p != nil {
			rows = make([]compactPaletteCommand, len(p))
			for i, v := range p {
				rows[i] = projectCompactPaletteCommand(v)
			}
		}
		return map[string]any{nativeToolsCommandsKey: rows}
	},
	Full:    func(p []contract.CmdPaletteCommand) any { return map[string]any{nativeToolsCommandsKey: p} },
	Preview: func(p []contract.CmdPaletteCommand) string { return fmt.Sprintf("%d commands", len(p)) },
}
var commandListViews = resultViews[contract.SessionCommandsResponse]{
	Compact: func(p contract.SessionCommandsResponse) any {
		var rows []compactComposerCommand
		if p.Commands != nil {
			rows = make([]compactComposerCommand, len(p.Commands))
			for i, v := range p.Commands {
				rows[i] = projectCompactComposerCommand(v)
			}
		}
		return map[string]any{nativeToolsCommandsKey: rows, "revision": p.Revision}
	},
	Preview: func(p contract.SessionCommandsResponse) string { return fmt.Sprintf("%d commands", len(p.Commands)) },
}
var skillCatalogViews = resultViews[[]contract.SkillPayload]{
	Compact: func(p []contract.SkillPayload) any {
		var rows []compactSkillCatalogEntry
		if p != nil {
			rows = make([]compactSkillCatalogEntry, len(p))
			for i, v := range p {
				rows[i] = projectCompactSkillCatalogEntry(v)
			}
		}
		return map[string]any{nativeToolsSkillsKey: rows}
	},
	Full:    func(p []contract.SkillPayload) any { return map[string]any{nativeToolsSkillsKey: p} },
	Preview: func(p []contract.SkillPayload) string { return fmt.Sprintf("%d skills", len(p)) },
}
