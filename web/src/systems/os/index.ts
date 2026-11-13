export { DesktopShell } from "./components/desktop-shell";
export { DesktopShellSkeleton } from "./components/desktop-shell-skeleton";
export {
  ShortcutBindingKeys,
  type ShortcutBindingKeysProps,
} from "./components/shortcut-binding-keys";
export { createOsRouteSync } from "./components/os-route-sync";
export { OsWindowSurface, type OsWindowSurfaceProps } from "./components/os-window-surface";
export { OsRouteNotFound } from "./components/os-route-not-found";
export { OsShellContext, type OsShellHandle } from "./contexts/os-shell-context";
export { useOsShell } from "./hooks/use-os-shell";
export { useOpenFilesApp, useOpenProjectFiles } from "./hooks/use-open-files-app";
export { useShellFocusedSessionId } from "./hooks/use-focused-session-id";
export { useDesktop } from "./hooks/use-desktop";
// Pass `{ reduced }` to `runViewTransition` from any system's in-window transition.
export { useOsReducedMotion } from "./hooks/use-os-reduced-motion";
export { isOsReducedMotion } from "./lib/reduced-motion";
export { openContextMenuFromKeyboard } from "./lib/context-menu-keyboard";
export {
  OS_APPS,
  getOsApp,
  preloadOsAppCode,
  resolveAppForPath,
  matchSessionInstance,
} from "./lib/app-registry";
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
export { desktopShellBridge, isDesktopShell } from "./lib/desktop-shell-bridge";
export { EDITOR_SETTINGS_PATH, reportOpenInEditorResult } from "./lib/open-in-editor-report";
export type {
  OpenInEditorErrorCode,
  OpenInEditorParams,
  OpenInEditorResult,
} from "./lib/desktop-shell-bridge";
export { stableWindowManagerClientId } from "./lib/window-manager-client-identity";

// Bindings and aliases: one daemon-owned settings section, mutated through one
// path. The daemon decides conflicts and echoes the section it produced, so no
// surface predicts the outcome of an overwrite (ADR-006).
export {
  fetchWindowManagerSettings,
  updateWindowManagerBindings,
  WindowManagerSettingsError,
  WindowManagerBindingsApplyError,
  type WindowManagerBindingsResult,
  type WindowManagerBindingUpdate,
  type WindowManagerMutationCode,
} from "./adapters/window-manager-settings-api";
export {
  parseSettingsWindowManagerSection,
  type WindowManagerAliasMap,
  type WindowManagerExtensionDefault,
  type WindowManagerSettingsScope,
  type WindowManagerSettingsSection,
  type WindowManagerSettingsWire,
  type WindowManagerShortcutCommand,
  type WindowManagerShortcutDiagnostic,
} from "./lib/window-manager-settings-section";
export {
  globalShortcutRegistrationSchema,
  globalShortcutRegistrationStatusSchema,
  type GlobalShortcutRegistrationStatus,
} from "./lib/window-manager-global-shortcut-schema";
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
} from "./lib/window-manager-stream-socket";
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
  LayoutAxis,
  LayoutNode,
  LayoutTab,
  WindowManagerClientView,
  WindowManagerConfig,
  WindowManagerDragModifier,
  WindowManagerRegisteredClientView,
  WindowManagerSnapshot,
} from "./lib/window-manager-types";
export { parseWindowManagerSnapshot } from "./lib/window-manager-schemas";
export {
  OsSplitSeam,
  OsSplitView,
  type OsSplitSeamProps,
  type OsSplitSeamSlot,
  type OsSplitViewProps,
} from "./components/os-split-view";
export type {
  WindowManagerGlobalShortcutMap,
  WindowManagerGlobalShortcutRegistration,
  WindowManagerShortcutBinding,
  WindowManagerShortcutMap,
} from "./lib/window-manager-shortcut-types";

// Per-window pixel floors, shared with the Settings layout editor so both read the same minimums.
export { buildWindowManagerMinimums } from "./lib/window-manager-view";

// The one command-palette registry projection. Every surface that renders a
// command — palette, menubar, cheatsheet, settings shortcut table — reads it,
// which is what makes their ids, labels and chords identical by construction.
export { CmdPaletteRegistryProvider } from "./contexts/cmd-palette-registry-context";
export { usePaletteCommand, usePaletteRegistry } from "./hooks/use-palette-registry";
export { registryShortcutActions } from "./lib/cmd-palette-shortcut-actions";
export { cmdPaletteKeys } from "./lib/cmd-palette-query-keys";
export { resetCmdPalettePersonalization } from "./adapters/cmd-palette-api";
export type {
  CmdPaletteCommand,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "./lib/cmd-palette-types";

// Keyboard grammar. Chord parsing and conflict detection live here; which
// commands exist comes from the registry above.
export {
  type ShortcutActionDefinition,
  chordFromKeyboardEvent,
  coveringShortcutFamily,
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
  SHORTCUT_RANGE_FAMILIES,
  primaryShortcutModifier,
  type ParsedShortcutChord,
  type ResolvedWindowManagerAction,
  type ShortcutBinding,
  type ShortcutCheatsheetRow,
  type ShortcutConflict,
  type ShortcutConflictKind,
  type ShortcutMap,
  type ShortcutRangeFamily,
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

export {
  parseWindowManagerSettingsApply,
  windowManagerApplyFailed,
  windowManagerApplyMessage,
  type WindowManagerSettingsApply,
} from "./lib/window-manager-settings-result";

export { lastOpenTarget, rememberOpenTarget } from "./lib/open-target-preference";
export type { CompozyShellBridge, OpenTarget } from "./lib/desktop-shell-bridge";
