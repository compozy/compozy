import type { ShortcutBinding } from "./window-manager-shortcuts";
import type { WindowManagerConfig, WindowManagerWorkspaceConfig } from "./window-manager-types";
import { effectiveShortcutMap } from "./window-manager-shortcut-overrides";

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
    swapModifier: workspace.swapModifier ?? global.swapModifier,
    historyLimit: workspace.historyLimit ?? global.historyLimit,
    navStackLimit: workspace.navStackLimit ?? global.navStackLimit,
    closedEntryLimit: workspace.closedEntryLimit ?? global.closedEntryLimit,
    desktopTransition: workspace.desktopTransition ?? global.desktopTransition,
    gaps: { ...(workspace.gaps ?? global.gaps) },
    shortcuts: { ...global.shortcuts, ...workspace.shortcuts },
    globalShortcuts: { ...global.globalShortcuts, ...workspace.globalShortcuts },
    shortcutDefaults: global.shortcutDefaults,
    effectiveShortcuts: workspace.shortcuts
      ? effectiveShortcutMap(workspaceShortcutBase(global), {
          ...global.shortcuts,
          ...workspace.shortcuts,
        })
      : global.effectiveShortcuts,
  };
}

/**
 * The keymap workspace overrides resolve against: the shipped defaults, plus
 * bindings the daemon resolved for commands outside them (extension defaults).
 *
 * Resolving from defaults rather than the global effective map matters under
 * override-wins: a default the global overrides displaced comes back when a
 * workspace override moves the claiming command elsewhere.
 */
function workspaceShortcutBase(global: WindowManagerConfig): Record<string, ShortcutBinding> {
  const base: Record<string, ShortcutBinding> = {};
  for (const [actionId, binding] of Object.entries(global.effectiveShortcuts)) {
    if (!(actionId in global.shortcutDefaults) && !(actionId in global.shortcuts)) {
      base[actionId] = binding;
    }
  }
  return { ...base, ...global.shortcutDefaults };
}
