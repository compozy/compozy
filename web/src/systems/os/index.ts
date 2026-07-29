export { DesktopShell } from "./components/desktop-shell";
export { createOsRouteSync } from "./components/os-route-sync";
export { OsRouteNotFound } from "./components/os-route-not-found";
export { OsShellContext, type OsShellHandle } from "./contexts/os-shell-context";
export { useOsShell } from "./hooks/use-os-shell";
export { useDesktop } from "./hooks/use-desktop";
export { OS_APPS, getOsApp, resolveAppForPath, matchSessionInstance } from "./lib/app-registry";
export { RoutingCoordinator, type OsRouterPort } from "./lib/routing-coordinator";
export { WindowManagerRuntime } from "./runtime/window-manager-runtime";
export { fetchWindowManagerSnapshot } from "./adapters/window-manager-api";
export {
  windowManagerConfigOptions,
  windowManagerKeys,
  windowManagerSnapshotOptions,
} from "./lib/window-manager-query";
export {
  type WindowManagerSocket,
  type WindowManagerSocketFactory,
} from "./hooks/use-window-manager-stream";
export {
  OS_COMPACT_BREAKPOINT,
  osWindowId,
  type OsAppId,
  type OsDesktopRuntime,
  type OsDesktopRuntimeStore,
  type OsRect,
  type OsWindow,
  type OsWindowRoute,
  type WindowManagerController,
} from "./lib/os-types";
export type {
  WindowManagerClientView,
  WindowManagerConfig,
  WindowManagerDragModifier,
  WindowManagerSnapshot,
} from "./lib/window-manager-types";

// Window geometry. The projector, the seam math and the floating clamp are the
// runtime's own — Settings renders the same rects the shell renders instead of
// computing a second, divergent model.
export { clampFloatingRect, projectLayout, type FloatingRectCommit } from "./lib/layout-projection";
export { applySeamPreviewToDesktop, seamWeightDelta } from "./lib/seam-preview";
export { buildWindowManagerMinimums } from "./lib/window-manager-view";
export type {
  LayoutAxis,
  LayoutDesktop,
  LayoutGroup,
  LayoutLeafNode,
  LayoutNode,
  LayoutProjection,
  LayoutProjectionDiagnostic,
  LayoutProjectionInput,
  LayoutSplitNode,
  LayoutStackNode,
  NormalizedRect,
  PixelRect,
  PixelSize,
  ProjectedSeam,
  ProjectedStack,
  ProjectedWindow,
  ProjectionGaps,
  WindowMinimums,
} from "./lib/window-manager-types";

// Keyboard grammar. The action registry is the shipped default keymap; Settings
// edits overrides against it rather than restating the list.
export {
  WINDOW_MANAGER_ACTIONS,
  WINDOW_PLACEMENT_COMMANDS,
  isWindowManagerActionId,
  type WindowManagerActionDefinition,
  type WindowManagerActionId,
  type WindowPlacementId,
} from "./lib/window-manager-command-registry";
export {
  chordFromKeyboardEvent,
  findShortcutConflicts,
  parseShortcutChord,
  resolveWindowManagerActions,
  shortcutLabel,
  type ParsedShortcutChord,
  type ResolvedWindowManagerAction,
  type ShortcutConflict,
  type ShortcutConflictKind,
} from "./lib/window-manager-shortcuts";
