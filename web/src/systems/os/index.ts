export { DesktopShell } from "./components/desktop-shell";
export {
  ShortcutBindingKeys,
  type ShortcutBindingKeysProps,
} from "./components/shortcut-binding-keys";
export { createOsRouteSync } from "./components/os-route-sync";
export { OsWindowSurface, type OsWindowSurfaceProps } from "./components/os-window-frame";
export { OsRouteNotFound } from "./components/os-route-not-found";
export { OsShellContext, type OsShellHandle } from "./contexts/os-shell-context";
export { useOsShell } from "./hooks/use-os-shell";
export { useDesktop } from "./hooks/use-desktop";
export { OS_APPS, getOsApp, resolveAppForPath, matchSessionInstance } from "./lib/app-registry";
export { getOsAppDescriptor, OS_APP_DESCRIPTORS } from "./lib/app-catalog";
export { validateMarketplaceDetailSearch } from "./apps/marketplace/marketplace-detail-search";
export { RoutingCoordinator, type OsRouterPort } from "./lib/routing-coordinator";
export { WindowManagerRuntime } from "./runtime/window-manager-runtime";
export { fetchWindowManagerSnapshot } from "./adapters/window-manager-api";
export {
  windowManagerConfigOptions,
  windowManagerKeys,
  windowManagerScopeKey,
  windowManagerSettingsOptions,
  windowManagerSnapshotOptions,
} from "./lib/window-manager-query";

// Bindings and aliases: one daemon-owned settings section, mutated through one
// path. The daemon decides conflicts and echoes the section it produced, so no
// surface predicts the outcome of an overwrite (ADR-006).
export {
  fetchWindowManagerSettings,
  updateWindowManagerBindings,
  WindowManagerSettingsError,
  type WindowManagerBindingUpdate,
  type WindowManagerMutationCode,
} from "./adapters/window-manager-settings-api";
export type {
  WindowManagerAliasMap,
  WindowManagerExtensionDefault,
  WindowManagerSettingsScope,
  WindowManagerSettingsSection,
  WindowManagerShortcutCommand,
  WindowManagerShortcutDiagnostic,
} from "./lib/window-manager-settings-section";
export {
  CORE_SHORTCUT_SOURCE,
  groupShortcutRowsBySource,
  orderShortcutSections,
  orderShortcutSources,
  shortcutSourceLabel,
  type ShortcutSourceGroup,
} from "./lib/shortcut-source-groups";
export {
  type WindowManagerSocket,
  type WindowManagerSocketFactory,
} from "./hooks/use-window-manager-stream";
export {
  OS_COMPACT_BREAKPOINT,
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
export type {
  WindowManagerGlobalShortcutMap,
  WindowManagerGlobalShortcutRegistration,
  WindowManagerShortcutBinding,
  WindowManagerShortcutMap,
} from "./lib/window-manager-shortcut-types";

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

// The one command-palette registry projection. Every surface that renders a
// command — palette, menubar, cheatsheet, settings shortcut table — reads it,
// which is what makes their ids, labels and chords identical by construction.
export { CmdPaletteRegistryProvider } from "./contexts/cmd-palette-registry-context";
export { usePaletteCommand, usePaletteRegistry } from "./hooks/use-palette-registry";
export { registryBindableIds, registryShortcutActions } from "./lib/cmd-palette-shortcut-actions";
export { cmdPaletteKeys } from "./lib/cmd-palette-query-keys";
export type {
  CmdPaletteCommand,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "./lib/cmd-palette-types";

// Window geometry presets for direct manipulation (the zoom menu, the tiling
// diagrams in Settings). Not a command catalog — invocation goes through the
// registry projection above.
export {
  WINDOW_ARRANGE_COMMANDS,
  WINDOW_PLACEMENT_COMMANDS,
  type WindowPlacementId,
} from "./lib/window-placement-presets";

// Keyboard grammar. Chord parsing and conflict detection live here; which
// commands exist comes from the registry above.
export {
  type ShortcutActionDefinition,
  chordFromKeyboardEvent,
  deriveShortcutCheatsheet,
  effectiveShortcutMap,
  expandShortcutOverrides,
  findShortcutConflicts,
  isShortcutOverrideActionId,
  parseShortcutChord,
  resolveWindowManagerActions,
  shortcutActionLabel,
  shortcutBindingProblem,
  shortcutKeyGlyphs,
  shortcutLabel,
  type ParsedShortcutChord,
  type ResolvedWindowManagerAction,
  type ShortcutBinding,
  type ShortcutCheatsheetRow,
  type ShortcutConflict,
  type ShortcutConflictKind,
  type ShortcutMap,
} from "./lib/window-manager-shortcuts";

// Attention: the system notification channel's truthful platform state, read by
// Settings so the switch can never claim to be armed when the platform refused.
export {
  requestSystemNotifications,
  showSystemNotification,
  systemNotificationsArmed,
  systemNotificationsSupported,
  systemNotificationState,
  type SystemNotificationState,
} from "./lib/system-notification-channel";
