package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	windowManagerReadCapability  = "window_manager.read"
	windowManagerWriteCapability = "window_manager.write"
	windowManagerTag             = "window manager"
	windowManagerDesktopsTag     = "desktops"
	windowManagerWindowsTag      = "windows"
	windowManagerLayoutTag       = "layout"
	windowManagerTileTag         = "tile"
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
		tags: []string{windowManagerTag, windowManagerDesktopsTag, memoryListKey},
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
		description: "List managed windows from the authoritative workspace snapshot.",
		inputSchema: windowManagerWindowListInputSchema, outputSchema: windowManagerReadOutputSchema,
		risk: toolspkg.RiskRead, readOnly: true, capability: windowManagerReadCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, memoryListKey},
	},
	{
		id: toolspkg.ToolIDWindowOpen, nativeName: "window_open", title: "Window Open",
		description: "Open a new managed window or restore a minimized window.",
		inputSchema: windowManagerWindowOpenInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "open", "restore"},
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
		description: "Close a managed window, or minimize it while preserving a return anchor.",
		inputSchema: windowManagerWindowCloseInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskDestructive, destructive: true, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "close", "minimize"},
	},
	{
		id: toolspkg.ToolIDWindowFocus, nativeName: "window_focus", title: "Window Focus",
		description: "Focus a window or directional neighbor for one explicit client.",
		inputSchema: windowManagerWindowFocusInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "focus", descriptorKeywordClient},
	},
	{
		id: toolspkg.ToolIDWindowMove, nativeName: "window_move", title: "Window Move",
		description: "Move one window structurally, or move its tiled group without placement fields.",
		inputSchema: windowManagerWindowMoveInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "move", windowManagerTileTag},
	},
	{
		id: toolspkg.ToolIDWindowSwap, nativeName: "window_swap", title: "Window Swap",
		description: "Swap the structural positions of two managed windows.",
		inputSchema: windowManagerWindowSwapInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "swap"},
	},
	{
		id: toolspkg.ToolIDWindowFloat, nativeName: "window_float", title: "Window Float",
		description: "Toggle one managed window between floating and tiled placement.",
		inputSchema: windowManagerWindowFloatInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "floating", windowManagerTileTag},
	},
	{
		id: toolspkg.ToolIDWindowZoom, nativeName: "window_zoom", title: "Window Zoom",
		description: "Toggle one managed window's reusable focus desktop for an explicit client.",
		inputSchema: windowManagerWindowZoomInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerWindowsTag, "zoom", "focus desktop"},
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
		id: toolspkg.ToolIDLayoutArrange, nativeName: "layout_arrange", title: "Layout Arrange",
		description: "Arrange selected windows into a horizontal, vertical, grid, or stack group.",
		inputSchema: windowManagerLayoutArrangeInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "arrange", windowManagerTileTag},
	},
	{
		id: toolspkg.ToolIDLayoutResize, nativeName: "layout_resize", title: "Layout Resize",
		description: "Resize one structural split boundary by a normalized delta.",
		inputSchema: windowManagerLayoutResizeInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "resize", "split"},
	},
	{
		id: toolspkg.ToolIDLayoutBalance, nativeName: "layout_balance", title: "Layout Balance",
		description: "Balance one tiled group or split into equal structural weights.",
		inputSchema: windowManagerLayoutBalanceInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "balance"},
	},
	{
		id: toolspkg.ToolIDLayoutUndo, nativeName: "layout_undo", title: "Layout Undo",
		description: "Undo the latest durable window-manager topology command.",
		inputSchema: windowManagerLayoutHistoryInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "undo", "history"},
	},
	{
		id: toolspkg.ToolIDLayoutRedo, nativeName: "layout_redo", title: "Layout Redo",
		description: "Redo the latest reverted window-manager topology command.",
		inputSchema: windowManagerLayoutHistoryInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskMutating, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "redo", "history"},
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
		tags: []string{windowManagerTag, windowManagerLayoutTag, "validate", descriptorKeywordDiagnostics},
	},
	{
		id: toolspkg.ToolIDLayoutApply, nativeName: "layout_apply", title: "Layout Apply",
		description: "Validate and atomically replace the workspace layout at an expected revision.",
		inputSchema: windowManagerLayoutApplyInputSchema, outputSchema: windowManagerCommandOutputSchema,
		risk: toolspkg.RiskDestructive, destructive: true, capability: windowManagerWriteCapability,
		tags: []string{windowManagerTag, windowManagerLayoutTag, "apply", "replace"},
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
		descriptor.OutputSchema = json.RawMessage(spec.outputSchema)
		descriptors = append(descriptors, withRequiredCapabilities(descriptor, spec.capability))
	}
	return descriptors
}
