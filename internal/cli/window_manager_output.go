package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/spf13/cobra"
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
				windowManagerRevisionFlag,
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

func windowManagerResultBundle(result contract.WindowManagerResult) outputBundle {
	return windowManagerChangeBundle(result, windowManagerChangeSummary{
		humanTitle:      "Window manager result",
		toonName:        "window_manager_result",
		stateLabel:      "Applied",
		stateField:      "applied",
		revision:        uint64(result.Snapshot.Revision),
		state:           result.Applied,
		desktopChanges:  len(result.Changes.DesktopIDs),
		windowChanges:   len(result.Changes.WindowIDs),
		diagnosticCount: len(result.Diagnostics),
	})
}

func windowManagerPreviewBundle(preview contract.WindowManagerPreview) outputBundle {
	return windowManagerChangeBundle(preview, windowManagerChangeSummary{
		humanTitle:      "Window manager preview",
		toonName:        "window_manager_preview",
		stateLabel:      "Changed",
		stateField:      "changed",
		revision:        uint64(preview.Snapshot.Revision),
		state:           preview.Changed,
		desktopChanges:  len(preview.Changes.DesktopIDs),
		windowChanges:   len(preview.Changes.WindowIDs),
		diagnosticCount: len(preview.Diagnostics),
	})
}

func windowManagerDesktopListBundle(desktops []contract.WindowManagerDesktop) outputBundle {
	return listBundle(
		desktops, desktops, "Desktops", []string{"ID", "ORDER", windowManagerNameHeader, "PURPOSE"},
		"desktops", []string{"id", "order", windowManagerNameField, windowManagerPurposeField},
		func(desktop contract.WindowManagerDesktop) []string {
			return []string{string(desktop.ID), strconv.Itoa(desktop.Order), desktop.Name, string(desktop.Purpose)}
		},
		func(desktop contract.WindowManagerDesktop) []string {
			return []string{string(desktop.ID), strconv.Itoa(desktop.Order), desktop.Name, string(desktop.Purpose)}
		},
	)
}

func windowManagerWindowListBundle(snapshot contract.WindowManagerSnapshot) outputBundle {
	windows := make([]contract.WindowManagerWindow, 0, len(snapshot.Windows))
	for _, window := range snapshot.Windows {
		windows = append(windows, window)
	}
	sort.Slice(windows, func(left, right int) bool { return windows[left].ID < windows[right].ID })
	return listBundle(
		windows, windows, "Windows", []string{"ID", "APP", "DESKTOP", "PLACEMENT", "PATHNAME", "MINIMIZED"},
		"windows", []string{"id", "app", windowManagerDesktopID, "placement", "pathname", "minimized"},
		windowManagerWindowRow,
		windowManagerWindowRow,
	)
}

func windowManagerWindowRow(window contract.WindowManagerWindow) []string {
	return []string{
		string(window.ID),
		window.App,
		string(window.DesktopID),
		string(window.Placement),
		window.Route.Pathname,
		strconv.FormatBool(window.Minimized),
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
		[]string{cliCodeValue, "Path", authoredContextMessageValue},
		rows,
	)
}
