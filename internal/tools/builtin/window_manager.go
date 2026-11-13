package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	windowManagerReadCapability  = "window_manager.read"
	windowManagerWriteCapability = "window_manager.write"
	windowManagerTag             = "window manager"
	windowManagerDesktopsTag     = "desktops"
	windowManagerWindowsTag      = "windows"
	windowManagerLayoutTag       = "layout"
	windowManagerTileTag         = "tile"
	windowManagerStackTag        = "stack"
	windowManagerTabsTag         = "tabs"
	windowManagerResizeTag       = "resize"
)

type windowManagerDescriptorSpec struct {
	id           toolspkg.ToolID
	nativeName   string
	title        string
	description  string
	inputSchema  string
	outputSchema string
	risk         toolspkg.RiskClass
	readOnly     bool
	destructive  bool
	capability   string
	tags         []string
}

var windowManagerToolSpecs = []windowManagerDescriptorSpec{
	{
		id: toolspkg.ToolIDDesktopList, nativeName: "desktop_list", title: "Desktop List",
		description: "List persistent desktops in one workspace.",
		inputSchema: windowManagerReadInputSchema, outputSchema: windowManagerReadOutputSchema,
		risk: toolspkg.RiskRead, readOnly: true, capability: windowManagerReadCapability,
		tags: []string{windowManagerTag, windowManagerDesktopsTag, descriptorKeywordList},
	},
	{
		id: toolspkg.ToolIDDesktopCreate, nativeName: "desktop_create", title: "Desktop Create",
		description: "Create one persistent desktop through revision-checked window-manager state.",
		inputSchema: windowManagerDesktopCreateInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerDesktopsTag, descriptorKeywordCreate},
	},
	{
		id: toolspkg.ToolIDDesktopUpdate, nativeName: "desktop_update", title: "Desktop Update",
		description: "Rename one persistent desktop through revision-checked window-manager state.",
		inputSchema: windowManagerDesktopUpdateInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerDesktopsTag, "rename"},
	},
	{
		id: toolspkg.ToolIDDesktopReorder, nativeName: "desktop_reorder", title: "Desktop Reorder",
		description: "Move one persistent desktop to a new ordered position.",
		inputSchema: windowManagerDesktopReorderInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerDesktopsTag, "reorder"},
	},
	{
		id: toolspkg.ToolIDDesktopSwitch, nativeName: "desktop_switch", title: "Desktop Switch",
		description: "Switch one explicit client's active desktop without changing workspace scope.",
		inputSchema: windowManagerDesktopSwitchInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerDesktopsTag, "switch", descriptorKeywordClient},
	},
	{
		id: toolspkg.ToolIDDesktopDelete, nativeName: "desktop_delete", title: "Desktop Delete",
		description: "Delete one persistent desktop and explicitly transfer its windows when required.",
		inputSchema: windowManagerDesktopDeleteInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskDestructive, destructive: true, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerDesktopsTag, "delete"},
	},
	{
		id: toolspkg.ToolIDDesktopClients, nativeName: "desktop_clients", title: "Desktop Clients",
		description: "List connected client-local active desktop and focus projections.",
		inputSchema: windowManagerReadInputSchema, outputSchema: windowManagerReadOutputSchema,
		risk: toolspkg.RiskRead, readOnly: true, capability: windowManagerReadCapability,
		tags: []string{windowManagerTag, windowManagerDesktopsTag, "clients"},
	},
	{
		id: toolspkg.ToolIDWindowList, nativeName: "window_list", title: "Window List",
		description: "List v6 windows with tab_id. Deprecated include_minimized is ignored until removal in v0.8.0.",
		inputSchema: windowManagerWindowListInputSchema, outputSchema: windowManagerReadOutputSchema,
		risk: toolspkg.RiskRead, readOnly: true, capability: windowManagerReadCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, descriptorKeywordList},
	},
	{
		id:         toolspkg.ToolIDWindowOpen,
		nativeName: "window_open",
		title:      "Window Open",
		description: "Open a window as a tab or split member. Deprecated restore_window_id activates an existing window; " +
			"floating and floating_rect are retired until removal in v0.8.0.",
		inputSchema:  windowManagerWindowOpenInputSchema,
		outputSchema: windowManagerCommandOutputSchema,
		risk:         toolspkg.RiskMutating,
		capability:   windowManagerWriteCapability,
		tags:         []string{windowManagerTag, windowManagerWindowsTag, "open", "restore"},
	},
	{
		id: toolspkg.ToolIDWindowNavigate, nativeName: "window_navigate", title: "Window Navigate",
		description: "Persist a managed window's internal application route and optionally focus one explicit client.",
		inputSchema: windowManagerWindowNavigateInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "navigate", "route"},
	},
	{
		id: toolspkg.ToolIDWindowClose, nativeName: "window_close", title: "Window Close",
		description: "Close a window or tabs by scope. Deprecated minimize is accepted as a no-op until v0.8.0.",
		inputSchema: windowManagerWindowCloseInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskDestructive, destructive: true, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "close", "minimize"},
	},
	{
		id:         toolspkg.ToolIDWindowGroup,
		nativeName: "window_group",
		title:      "Window Group",
		description: "Deprecated until v0.8.0: make each listed window an adjacent tab after the target. " +
			"Use window_move with placement tab; insert_index is ignored.",
		inputSchema:  windowManagerWindowGroupInputSchema,
		outputSchema: windowManagerCommandOutputSchema,
		risk:         toolspkg.RiskMutating,
		capability:   windowManagerWriteCapability,
		tags: []string{
			windowManagerTag, windowManagerWindowsTag, "group", windowManagerStackTag, windowManagerTabsTag,
		},
	},
	{
		id: toolspkg.ToolIDWindowReorder, nativeName: "window_reorder", title: "Window Reorder",
		description: "Reorder the window's tab within its pinned or unpinned range.",
		inputSchema: windowManagerWindowReorderInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{
			windowManagerTag, windowManagerWindowsTag, "reorder", windowManagerStackTag, windowManagerTabsTag,
		},
	},
	{
		id: toolspkg.ToolIDWindowActivate, nativeName: "window_activate", title: "Window Activate",
		description: "Activate the window's tab for its desktop and the issuing client.",
		inputSchema: windowManagerWindowActivateInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{
			windowManagerTag, windowManagerWindowsTag, "activate", windowManagerStackTag, windowManagerTabsTag,
		},
	},
	{
		id: toolspkg.ToolIDWindowPin, nativeName: "window_pin", title: "Window Pin",
		description: "Pin or unpin the window's complete tab.",
		inputSchema: windowManagerWindowPinInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{
			windowManagerTag, windowManagerWindowsTag, "pin", windowManagerStackTag, windowManagerTabsTag,
		},
	},
	{
		id: toolspkg.ToolIDWindowReopen, nativeName: "window_reopen", title: "Window Reopen",
		description: "Restore the newest workspace-level closed window entry.",
		inputSchema: windowManagerWindowReopenInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "reopen", "restore", windowManagerTabsTag},
	},
	{
		id: toolspkg.ToolIDWindowFocus, nativeName: "window_focus", title: "Window Focus",
		description: "Focus a window or directional neighbor for one explicit client.",
		inputSchema: windowManagerWindowFocusInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "focus", descriptorKeywordClient},
	},
	{
		id:         toolspkg.ToolIDWindowMove,
		nativeName: "window_move",
		title:      "Window Move",
		description: "Move a window or complete tab to a tab or split edge; target_tab_id addresses a tab root. " +
			"Floating placement and rectangles are deprecated until v0.8.0.",
		inputSchema:  windowManagerWindowMoveInputSchema,
		outputSchema: windowManagerCommandOutputSchema,
		risk:         toolspkg.RiskMutating,
		capability:   windowManagerWriteCapability,
		tags:         []string{windowManagerTag, windowManagerWindowsTag, "move", windowManagerTileTag},
	},
	{
		id: toolspkg.ToolIDWindowResize, nativeName: "window_resize", title: "Window Resize",
		description: "Deprecated no-op until v0.8.0. Use layout_resize to resize a split seam.",
		inputSchema: windowManagerWindowResizeInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, windowManagerResizeTag, windowManagerTileTag},
	},
	{
		id: toolspkg.ToolIDWindowSwap, nativeName: "window_swap", title: "Window Swap",
		description: "Swap the structural positions of two managed windows.",
		inputSchema: windowManagerWindowSwapInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "swap"},
	},
	{
		id:           toolspkg.ToolIDWindowFloat,
		nativeName:   "window_float",
		title:        "Window Float",
		description:  "Deprecated no-op until v0.8.0: floating windows were removed. Use window_move with placement tab.",
		inputSchema:  windowManagerWindowFloatInputSchema,
		outputSchema: windowManagerCommandOutputSchema,
		risk:         toolspkg.RiskMutating,
		capability:   windowManagerWriteCapability,
		tags:         []string{windowManagerTag, windowManagerWindowsTag, "floating", windowManagerTileTag},
	},
	{
		id: toolspkg.ToolIDWindowZoom, nativeName: "window_zoom", title: "Window Zoom",
		description: "Deprecated no-op until v0.8.0: zoom was removed. Use window_move with placement tab.",
		inputSchema: windowManagerWindowZoomInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "zoom", "maximize"},
	},
	{
		id: toolspkg.ToolIDLayoutGet, nativeName: "layout_get", title: "Layout Get",
		description: "Read the complete authoritative window-manager snapshot.",
		inputSchema: windowManagerReadInputSchema, outputSchema: windowManagerReadOutputSchema,
		risk: toolspkg.RiskRead, readOnly: true, capability: windowManagerReadCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, descriptorKeywordSnapshot},
	},
	{
		id: toolspkg.ToolIDLayoutPreview, nativeName: "layout_preview", title: "Layout Preview",
		description: "Validate and preview one semantic command without persisting it.",
		inputSchema: windowManagerLayoutPreviewInputSchema, outputSchema: windowManagerPreviewOutputSchema,
		risk: toolspkg.RiskRead, readOnly: true, capability: windowManagerReadCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "preview", "dry run"},
	},
	{
		id:         toolspkg.ToolIDLayoutArrange,
		nativeName: "layout_arrange",
		title:      "Layout Arrange",
		description: "Arrange windows into one split tab. Deprecated stack creates adjacent tabs; " +
			"frame, group_id and keep_frames are ignored until removal in v0.8.0.",
		inputSchema:  windowManagerLayoutArrangeInputSchema,
		outputSchema: windowManagerCommandOutputSchema,
		risk:         toolspkg.RiskMutating,
		capability:   windowManagerWriteCapability,
		tags:         []string{windowManagerTag, windowManagerLayoutTag, "arrange", windowManagerTileTag},
	},
	{
		id: toolspkg.ToolIDLayoutResize, nativeName: "layout_resize", title: "Layout Resize",
		description: "Resize one structural split boundary by a normalized delta.",
		inputSchema: windowManagerLayoutResizeInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, windowManagerResizeTag, "split"},
	},
	{
		id: toolspkg.ToolIDLayoutFrameResize, nativeName: "layout_frame_resize", title: "Layout Frame Resize",
		description: "Deprecated no-op until v0.8.0. Use layout_resize to resize a split seam.",
		inputSchema: windowManagerLayoutFrameResizeInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{
			windowManagerTag,
			windowManagerLayoutTag,
			windowManagerResizeTag,
			"frame",
			windowManagerTileTag,
		},
	},
	{
		id:           toolspkg.ToolIDLayoutBalance,
		nativeName:   "layout_balance",
		title:        "Layout Balance",
		description:  "Balance every split in a tab, or one split. group_id is a deprecated alias of tab_id until v0.8.0.",
		inputSchema:  windowManagerLayoutBalanceInputSchema,
		outputSchema: windowManagerCommandOutputSchema,
		risk:         toolspkg.RiskMutating,
		capability:   windowManagerWriteCapability,
		tags:         []string{windowManagerTag, windowManagerLayoutTag, "balance"},
	},
	{
		id: toolspkg.ToolIDLayoutUndo, nativeName: "layout_undo", title: "Layout Undo",
		description: "Undo the latest durable window-manager topology command.",
		inputSchema: windowManagerLayoutHistoryInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "undo", historyTag},
	},
	{
		id: toolspkg.ToolIDLayoutRedo, nativeName: "layout_redo", title: "Layout Redo",
		description: "Redo the latest reverted window-manager topology command.",
		inputSchema: windowManagerLayoutHistoryInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "redo", historyTag},
	},
	{
		id: toolspkg.ToolIDLayoutExport, nativeName: "layout_export", title: "Layout Export",
		description: "Export a history-free declarative layout document.",
		inputSchema: windowManagerReadInputSchema, outputSchema: windowManagerReadOutputSchema,
		risk: toolspkg.RiskRead, readOnly: true, capability: windowManagerReadCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "export"},
	},
	{
		id: toolspkg.ToolIDLayoutValidate, nativeName: "layout_validate", title: "Layout Validate",
		description: "Validate a raw declarative layout and return stable diagnostics without writing.",
		inputSchema: windowManagerLayoutValidateInputSchema, outputSchema: windowManagerReadOutputSchema,
		risk: toolspkg.RiskRead, readOnly: true, capability: windowManagerReadCapability,
		tags: []string{
			windowManagerTag,
			windowManagerLayoutTag,
			descriptorKeywordValidate,
			descriptorKeywordDiagnostics,
		},
	},
	{
		id: toolspkg.ToolIDLayoutApply, nativeName: "layout_apply", title: "Layout Apply",
		description: "Validate and atomically replace the workspace layout at an expected revision.",
		inputSchema: windowManagerLayoutApplyInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskDestructive, destructive: true, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "apply", descriptorKeywordReplace},
	},
}

func windowManagerDescriptors() []toolspkg.Descriptor {
	descriptors := make([]toolspkg.Descriptor, 0, len(windowManagerToolSpecs))
	for _, spec := range windowManagerToolSpecs {
		descriptor := nativeDescriptor(
			spec.id,
			spec.nativeName,
			spec.title,
			spec.description,
			spec.inputSchema,
			spec.risk,
			spec.readOnly,
			spec.destructive,
			false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDWindowManager},
			spec.tags,
			[]string{spec.title, spec.description},
		)
		metadata := descriptor.ExecutionMetadata()
		metadata.ReservedWorkspacePartition = true
		descriptor.ToolExecutionMetadata = &metadata
		descriptor.OutputSchema = json.RawMessage(spec.outputSchema)
		switch spec.id {
		case toolspkg.ToolIDDesktopClients:
			descriptor = withResultViews(descriptor, desktopClientsCompactSchema, spec.outputSchema)
		case toolspkg.ToolIDLayoutGet:
			descriptor = withResultViews(descriptor, layoutGetCompactSchema, spec.outputSchema)
		case toolspkg.ToolIDLayoutPreview:
			descriptor = withResultViews(descriptor, layoutPreviewCompactSchema, spec.outputSchema)
		}
		descriptors = append(descriptors, withRequiredCapabilities(descriptor, spec.capability))
	}
	return descriptors
}
