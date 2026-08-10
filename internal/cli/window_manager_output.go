package cli

import (
	"encoding/json"
	"fmt"
	"sort"
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
	memberships := windowManagerStackMemberships(snapshot.Desktops)
	windows := make([]windowManagerWindowListItem, 0, len(snapshot.Windows))
	for _, window := range snapshot.Windows {
		item := windowManagerWindowListItem{
			WindowManagerWindow: window,
			NavDepth:            len(window.NavStack),
		}
		if membership, ok := memberships[window.ID]; ok {
			stackID := membership.stackID
			memberOrder := membership.memberOrder
			item.StackID = &stackID
			item.MemberOrder = &memberOrder
			item.Active = membership.active
		}
		windows = append(windows, item)
	}
	sort.Slice(windows, func(left, right int) bool {
		return windows[left].ID < windows[right].ID
	})
	return listBundle(
		windows, windows, "Windows",
		[]string{
			"ID", "APP", "DESKTOP", "PLACEMENT", "PATHNAME", "STACK", "MEMBER ORDER", "ACTIVE", "PINNED", "NAV DEPTH",
		},
		"windows",
		[]string{
			"id", appCommandKey, windowManagerDesktopID, "placement", "pathname", "stack_id", "member_order",
			windowManagerActiveField, "pinned", "nav_depth",
		},
		windowManagerWindowRow,
		windowManagerWindowRow,
	)
}

type windowManagerWindowListItem struct {
	contract.WindowManagerWindow
	StackID     *windowmanager.NodeID `json:"stack_id,omitempty"`
	MemberOrder *int                  `json:"member_order,omitempty"`
	Active      bool                  `json:"active"`
	NavDepth    int                   `json:"nav_depth"`
}

type windowManagerStackMembership struct {
	stackID     windowmanager.NodeID
	memberOrder int
	active      bool
}

func windowManagerStackMemberships(
	desktops []contract.WindowManagerDesktop,
) map[windowmanager.WindowID]windowManagerStackMembership {
	memberships := make(map[windowmanager.WindowID]windowManagerStackMembership)
	for _, desktop := range desktops {
		for _, stack := range desktop.FloatingStacks {
			addWindowManagerStackMemberships(memberships, stack.ID, stack.WindowIDs, stack.ActiveID)
		}
		for _, group := range desktop.Groups {
			addWindowManagerNodeStackMemberships(memberships, group.Root)
		}
	}
	return memberships
}

func addWindowManagerNodeStackMemberships(
	memberships map[windowmanager.WindowID]windowManagerStackMembership,
	node contract.WindowManagerLayoutNode,
) {
	if node.Kind == windowmanager.NodeKindStack {
		addWindowManagerStackMemberships(memberships, node.ID, node.WindowIDs, node.ActiveID)
	}
	for _, child := range node.Children {
		addWindowManagerNodeStackMemberships(memberships, child)
	}
}

func addWindowManagerStackMemberships(
	memberships map[windowmanager.WindowID]windowManagerStackMembership,
	stackID windowmanager.NodeID,
	windowIDs []windowmanager.WindowID,
	activeID *windowmanager.WindowID,
) {
	for index, windowID := range windowIDs {
		memberships[windowID] = windowManagerStackMembership{
			stackID: stackID, memberOrder: index, active: activeID != nil && *activeID == windowID,
		}
	}
}

func windowManagerWindowRow(item windowManagerWindowListItem) []string {
	window := item.WindowManagerWindow
	stackID := ""
	if item.StackID != nil {
		stackID = string(*item.StackID)
	}
	memberOrder := ""
	if item.MemberOrder != nil {
		memberOrder = strconv.Itoa(*item.MemberOrder)
	}
	return []string{
		string(window.ID),
		window.App,
		string(window.DesktopID),
		string(window.Placement),
		window.Route.Pathname,
		stackID,
		memberOrder,
		strconv.FormatBool(item.Active),
		strconv.FormatBool(window.Pinned),
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
