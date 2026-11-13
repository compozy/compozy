import type { WindowManagerConfig } from "@/systems/os";

/**
 * Serialize the behavior Settings edits, including zero gaps and global
 * shortcuts. Retired keys (`raise_on_focus`, `drag_away_policy`,
 * `group_move_modifier`, `snap.*`, `bindings.*`; `_spec.md` §Config Lifecycle)
 * are left out on purpose: Settings never writes a retired value back.
 */
export function windowManagerSettingsConfigToWire(config: WindowManagerConfig) {
  return {
    new_window_policy: config.newWindowPolicy,
    small_viewport_policy: config.smallViewportPolicy,
    focus_policy: config.focusPolicy,
    focus_wrap: config.focusWrap,
    focus_follows_pointer: config.focusFollowsPointer,
    swap_modifier: config.swapModifier,
    history_limit: config.historyLimit,
    nav_stack_limit: config.navStackLimit,
    closed_entry_limit: config.closedEntryLimit,
    desktop_transition: config.desktopTransition,
    gaps: config.gaps,
    shortcuts: config.shortcuts,
    global_shortcuts: config.globalShortcuts,
  };
}
