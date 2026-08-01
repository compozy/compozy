package tools

// Window-manager native tool IDs: desktops, windows, and layout surfaces.
const (
	// ToolIDDesktopList lists persistent desktops.
	ToolIDDesktopList ToolID = "compozy__desktop_list"
	// ToolIDDesktopCreate creates a persistent desktop.
	ToolIDDesktopCreate ToolID = "compozy__desktop_create"
	// ToolIDDesktopUpdate renames a persistent desktop.
	ToolIDDesktopUpdate ToolID = "compozy__desktop_update"
	// ToolIDDesktopReorder changes persistent desktop order.
	ToolIDDesktopReorder ToolID = "compozy__desktop_reorder"
	// ToolIDDesktopSwitch changes one explicit client's active desktop.
	ToolIDDesktopSwitch ToolID = "compozy__desktop_switch"
	// ToolIDDesktopDelete deletes a persistent desktop.
	ToolIDDesktopDelete ToolID = "compozy__desktop_delete"
	// ToolIDDesktopClients lists connected window-manager clients.
	ToolIDDesktopClients ToolID = "compozy__desktop_clients"
	// ToolIDWindowList lists managed windows.
	ToolIDWindowList ToolID = "compozy__window_list"
	// ToolIDWindowOpen opens or restores a managed window.
	ToolIDWindowOpen ToolID = "compozy__window_open"
	// ToolIDWindowNavigate persists one managed window's internal route.
	ToolIDWindowNavigate ToolID = "compozy__window_navigate"
	// ToolIDWindowClose closes or minimizes a managed window.
	ToolIDWindowClose ToolID = "compozy__window_close"
	// ToolIDWindowGroup groups windows into one ordered stack.
	ToolIDWindowGroup ToolID = "compozy__window_group"
	// ToolIDWindowReorder reorders one member within its stack.
	ToolIDWindowReorder ToolID = "compozy__window_reorder"
	// ToolIDWindowActivate durably activates one stacked window.
	ToolIDWindowActivate ToolID = "compozy__window_activate"
	// ToolIDWindowPin changes one window's pinned state.
	ToolIDWindowPin ToolID = "compozy__window_pin"
	// ToolIDWindowReopen restores the newest closed entry.
	ToolIDWindowReopen ToolID = "compozy__window_reopen"
	// ToolIDWindowFocus focuses a managed window for one explicit client.
	ToolIDWindowFocus ToolID = "compozy__window_focus"
	// ToolIDWindowMove moves a managed window.
	ToolIDWindowMove ToolID = "compozy__window_move"
	// ToolIDWindowResize resizes a window frame, detaching split members into islands.
	ToolIDWindowResize ToolID = "compozy__window_resize"
	// ToolIDWindowSwap swaps two managed windows.
	ToolIDWindowSwap ToolID = "compozy__window_swap"
	// ToolIDWindowFloat toggles one managed window's floating state.
	ToolIDWindowFloat ToolID = "compozy__window_float"
	// ToolIDWindowZoom toggles one managed window's focus desktop.
	ToolIDWindowZoom ToolID = "compozy__window_zoom"
	// ToolIDLayoutGet reads the authoritative topology snapshot.
	ToolIDLayoutGet ToolID = "compozy__layout_get"
	// ToolIDLayoutPreview previews one semantic layout command.
	ToolIDLayoutPreview ToolID = "compozy__layout_preview"
	// ToolIDLayoutArrange arranges windows into a tiled group.
	ToolIDLayoutArrange ToolID = "compozy__layout_arrange"
	// ToolIDLayoutResize resizes a split boundary.
	ToolIDLayoutResize ToolID = "compozy__layout_resize"
	// ToolIDLayoutFrameResize atomically rewrites adjacent island frames.
	ToolIDLayoutFrameResize ToolID = "compozy__layout_frame_resize"
	// ToolIDLayoutBalance balances a group or split.
	ToolIDLayoutBalance ToolID = "compozy__layout_balance"
	// ToolIDLayoutUndo reverts the latest durable topology command.
	ToolIDLayoutUndo ToolID = "compozy__layout_undo"
	// ToolIDLayoutRedo reapplies the latest reverted topology command.
	ToolIDLayoutRedo ToolID = "compozy__layout_redo"
	// ToolIDLayoutExport exports a history-free layout document.
	ToolIDLayoutExport ToolID = "compozy__layout_export"
	// ToolIDLayoutValidate validates a raw layout document.
	ToolIDLayoutValidate ToolID = "compozy__layout_validate"
	// ToolIDLayoutApply atomically applies a validated layout document.
	ToolIDLayoutApply ToolID = "compozy__layout_apply"
)
