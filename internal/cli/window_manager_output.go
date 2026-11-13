package cli

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

const (
	windowManagerActiveField  = "active"
	windowManagerChangedField = "changed"
)

func windowManagerJSONBundle(name string, title string, value any) outputBundle {
	return outputBundle{
		jsonValue: value,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, value)
		},
		human: func() (string, error) {
			encoded, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return "", fmt.Errorf("cli: encode %s output: %w", name, err)
			}
			return title + "\n\n" + string(encoded), nil
		},
		toon: func() (string, error) {
			encoded, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("cli: encode %s output: %w", name, err)
			}
			return renderToonObject(name, []string{"value"}, []string{string(encoded)}), nil
		},
	}
}

func windowManagerLayoutDocumentBundle(document contract.WindowManagerLayoutDocument) outputBundle {
	bundle := windowManagerJSONBundle("window_layout", "Window layout", document)
	bundle.json = func(cmd *cobra.Command) error {
		return writeJSONWithoutWorkspaceResolution(cmd, document)
	}
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLineWithoutWorkspaceResolution(cmd, document)
	}
	return bundle
}

type windowManagerChangeSummary struct {
	humanTitle      string
	toonName        string
	stateLabel      string
	stateField      string
	revision        uint64
	state           bool
	desktopChanges  int
	windowChanges   int
	diagnosticCount int
}

func windowManagerChangeBundle[T any](value T, summary windowManagerChangeSummary) outputBundle {
	return outputBundle{
		jsonValue: value,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, value) },
		human: func() (string, error) {
			return renderHumanSection(summary.humanTitle, []keyValue{
				{Label: cliRevisionValue, Value: strconv.FormatUint(summary.revision, 10)},
				{Label: summary.stateLabel, Value: strconv.FormatBool(summary.state)},
				{Label: "Desktops changed", Value: strconv.Itoa(summary.desktopChanges)},
				{Label: "Windows changed", Value: strconv.Itoa(summary.windowChanges)},
				{Label: windowManagerDiagnosticsLabel, Value: strconv.Itoa(summary.diagnosticCount)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(summary.toonName, []string{
				cliRevisionKey,
				summary.stateField,
				"desktop_changes",
				"window_changes",
				windowManagerDiagnostics,
			}, []string{
				strconv.FormatUint(summary.revision, 10), strconv.FormatBool(summary.state),
				strconv.Itoa(summary.desktopChanges), strconv.Itoa(summary.windowChanges),
				strconv.Itoa(summary.diagnosticCount),
			}), nil
		},
	}
}

func windowManagerResultBundle(result *contract.WindowManagerResult) outputBundle {
	return windowManagerChangeBundle(result, windowManagerChangeSummary{
		humanTitle:      "Window manager result",
		toonName:        "window_manager_result",
		stateLabel:      "Applied",
		stateField:      cliAppliedKey,
		revision:        uint64(result.Snapshot.Revision),
		state:           result.Applied,
		desktopChanges:  len(result.Changes.DesktopIDs),
		windowChanges:   len(result.Changes.WindowIDs),
		diagnosticCount: len(result.Diagnostics),
	})
}

func windowManagerPreviewBundle(preview *contract.WindowManagerPreview) outputBundle {
	return windowManagerChangeBundle(preview, windowManagerChangeSummary{
		humanTitle:      "Window manager preview",
		toonName:        "window_manager_preview",
		stateLabel:      "Changed",
		stateField:      windowManagerChangedField,
		revision:        uint64(preview.Snapshot.Revision),
		state:           preview.Changed,
		desktopChanges:  len(preview.Changes.DesktopIDs),
		windowChanges:   len(preview.Changes.WindowIDs),
		diagnosticCount: len(preview.Diagnostics),
	})
}

func windowManagerDesktopListBundle(desktops []contract.WindowManagerDesktop) outputBundle {
	return listBundle(
		desktops, desktops, "Desktops", []string{"ID", "ORDER", windowManagerNameHeader},
		"desktops", []string{"id", "order", windowManagerNameField},
		func(desktop contract.WindowManagerDesktop) []string {
			return []string{string(desktop.ID), strconv.Itoa(desktop.Order), desktop.Name}
		},
		func(desktop contract.WindowManagerDesktop) []string {
			return []string{string(desktop.ID), strconv.Itoa(desktop.Order), desktop.Name}
		},
	)
}

func windowManagerWindowListBundle(snapshot contract.WindowManagerSnapshot) outputBundle {
	windows := make([]windowManagerWindowListItem, 0, len(snapshot.Windows))
	for _, desktop := range snapshot.Desktops {
		for index, tab := range desktop.Tabs {
			ids := windowManagerTabWindowIDs(tab.Root)
			for _, id := range ids {
				window, ok := snapshot.Windows[string(id)]
				if !ok {
					continue
				}
				windows = append(windows, windowManagerWindowListItem{
					WindowManagerWindow: window, TabIndex: index, Members: len(ids),
					Active: desktop.ActiveTabID == tab.ID, Pinned: tab.Pinned, NavDepth: len(window.NavStack),
				})
			}
		}
	}
	slices.SortFunc(
		windows,
		func(left, right windowManagerWindowListItem) int { return cmp.Compare(left.ID, right.ID) },
	)
	return listBundle(
		windows,
		windows,
		"Windows",
		[]string{"ID", "APP", "DESKTOP", "TAB", "TAB INDEX", "ACTIVE", "PINNED", "MEMBERS", "PATHNAME", "NAV DEPTH"},
		"windows",
		[]string{
			"id",
			appCommandKey,
			windowManagerDesktopID,
			"tab_id",
			"tab_index",
			windowManagerActiveField,
			"pinned",
			"members",
			"pathname",
			"nav_depth",
		},
		windowManagerWindowRow,
		windowManagerWindowRow,
	)
}

type windowManagerWindowListItem struct {
	contract.WindowManagerWindow
	TabIndex int  `json:"tab_index"`
	Active   bool `json:"active"`
	Pinned   bool `json:"pinned"`
	Members  int  `json:"members"`
	NavDepth int  `json:"nav_depth"`
}

func windowManagerTabWindowIDs(node contract.WindowManagerLayoutNode) []windowmanager.WindowID {
	if node.WindowID != nil {
		return []windowmanager.WindowID{*node.WindowID}
	}
	var ids []windowmanager.WindowID
	for _, child := range node.Children {
		ids = append(ids, windowManagerTabWindowIDs(child)...)
	}
	return ids
}

func windowManagerWindowRow(item windowManagerWindowListItem) []string {
	return []string{
		string(item.ID),
		item.App,
		string(item.DesktopID),
		string(item.TabID),
		strconv.Itoa(item.TabIndex),
		strconv.FormatBool(
			item.Active,
		),
		strconv.FormatBool(item.Pinned),
		strconv.Itoa(item.Members),
		item.Route.Pathname,
		strconv.Itoa(item.NavDepth),
	}
}

func windowManagerClientListBundle(clients []contract.WindowManagerClientView) outputBundle {
	return listBundle(
		clients, clients, "Window manager clients", []string{"ID", "ACTIVE DESKTOP", "FOCUSED WINDOW", "CONNECTED"},
		"window_manager_clients", []string{"id", "active_desktop_id", "focused_window_id", "connected_at"},
		windowManagerClientRow, windowManagerClientRow,
	)
}

func windowManagerClientRow(client contract.WindowManagerClientView) []string {
	focused := ""
	if client.FocusedWindowID != nil {
		focused = string(*client.FocusedWindowID)
	}
	return []string{
		string(client.ClientID), string(client.ActiveDesktopID), focused, client.ConnectedAt.Format(time.RFC3339),
	}
}

func windowManagerValidationBundle(validation contract.WindowManagerLayoutValidationResponse) outputBundle {
	return outputBundle{
		jsonValue: validation,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, validation) },
		human: func() (string, error) {
			summary := renderHumanSection("Layout validation", []keyValue{
				{Label: "Valid", Value: strconv.FormatBool(validation.Valid)},
				{Label: windowManagerDiagnosticsLabel, Value: strconv.Itoa(len(validation.Diagnostics))},
			})
			if len(validation.Diagnostics) == 0 {
				return summary, nil
			}
			return summary + "\n\n" + windowManagerLayoutDiagnosticsTable(validation.Diagnostics), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"layout_validation",
				[]string{windowManagerValidField, windowManagerDiagnostics},
				[]string{strconv.FormatBool(validation.Valid), strconv.Itoa(len(validation.Diagnostics))},
			), nil
		},
	}
}

func windowManagerLayoutDiagnosticsTable(diagnostics []windowmanager.Diagnostic) string {
	rows := make([][]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		rows = append(rows, []string{
			stringOrDash(diagnostic.Code),
			stringOrDash(diagnostic.Path),
			stringOrDash(diagnostic.Message),
		})
	}
	return renderHumanTable(
		windowManagerDiagnosticsLabel,
		[]string{cliCodeValue, automationPathValue, authoredContextMessageValue},
		rows,
	)
}
