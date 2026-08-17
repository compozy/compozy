import type { WindowManagerConfig, WindowManagerWorkspaceConfig } from "./window-manager-types";
import { effectiveShortcutMap } from "./window-manager-shortcuts";

/** Deterministic global + workspace merge mirroring the daemon's effective config. */
export function effectiveWindowManagerConfig(
  global: WindowManagerConfig,
  workspace: WindowManagerWorkspaceConfig
): WindowManagerConfig {
  return {
    newWindowPolicy: workspace.newWindowPolicy ?? global.newWindowPolicy,
    smallViewportPolicy: workspace.smallViewportPolicy ?? global.smallViewportPolicy,
    focusPolicy: workspace.focusPolicy ?? global.focusPolicy,
    focusWrap: workspace.focusWrap ?? global.focusWrap,
    focusFollowsPointer: workspace.focusFollowsPointer ?? global.focusFollowsPointer,
    raiseOnFocus: workspace.raiseOnFocus ?? global.raiseOnFocus,
    dragAwayPolicy: workspace.dragAwayPolicy ?? global.dragAwayPolicy,
    groupMoveModifier: workspace.groupMoveModifier ?? global.groupMoveModifier,
    swapModifier: workspace.swapModifier ?? global.swapModifier,
    historyLimit: workspace.historyLimit ?? global.historyLimit,
    navStackLimit: workspace.navStackLimit ?? global.navStackLimit,
    closedEntryLimit: workspace.closedEntryLimit ?? global.closedEntryLimit,
    desktopTransition: workspace.desktopTransition ?? global.desktopTransition,
    gaps: { ...(workspace.gaps ?? global.gaps) },
    snap: {
      ...(workspace.snap ?? global.snap),
      repeatRatios: [...(workspace.snap?.repeatRatios ?? global.snap.repeatRatios)],
    },
    bindings: { ...(workspace.bindings ?? global.bindings) },
    shortcuts: { ...global.shortcuts, ...workspace.shortcuts },
    shortcutDefaults: global.shortcutDefaults,
    effectiveShortcuts: workspace.shortcuts
      ? effectiveShortcutMap(global.effectiveShortcuts, workspace.shortcuts)
      : global.effectiveShortcuts,
  };
}
