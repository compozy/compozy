import { z } from "zod";

import { shortcutBindingProblem, type ShortcutBinding } from "./window-manager-shortcuts";
import type { WindowManagerShortcutMap } from "./window-manager-shortcut-types";
import type { WindowManagerConfig, WindowManagerWorkspaceConfig } from "./window-manager-types";

const gapsSchema = z.strictObject({
  inner: z.number().finite().nonnegative(),
  top: z.number().finite().nonnegative(),
  right: z.number().finite().nonnegative(),
  bottom: z.number().finite().nonnegative(),
  left: z.number().finite().nonnegative(),
});

/**
 * Config keys retired with tabs and splits (`_spec.md` §Config Lifecycle). The
 * daemon ignores them and may send them as zero values, leave them out, or pass
 * an old `config.toml` value through, so they are accepted with any value and
 * dropped: no parse fails on them and no domain type carries them. Remove in
 * v0.8.0, when the daemon stops serving them.
 */
const retiredConfigKeys = {
  raise_on_focus: z.unknown().optional(),
  drag_away_policy: z.unknown().optional(),
  group_move_modifier: z.unknown().optional(),
  snap: z.unknown().optional(),
  bindings: z.unknown().optional(),
};

const shortcutBindingSchema = z
  .union([z.string(), z.array(z.string())])
  .transform((binding): ShortcutBinding => {
    if (typeof binding === "string") return binding.trim() === "" ? [] : [binding];
    return binding;
  });

export const shortcutsSchema = z
  .record(z.string(), shortcutBindingSchema)
  .superRefine((shortcuts, context) => {
    for (const [actionId, binding] of Object.entries(shortcuts)) {
      const problem = shortcutBindingProblem(actionId, binding);
      if (problem) context.addIssue({ code: "custom", message: problem, path: [actionId] });
    }
  })
  .transform((shortcuts): WindowManagerShortcutMap => shortcuts);

export const windowManagerWorkspaceConfigSchema = z
  .strictObject({
    new_window_policy: z.enum(["floating", "beside_focus", "tab"]).optional(),
    small_viewport_policy: z.enum(["stack", "reject"]).optional(),
    focus_policy: z.enum(["click_directional", "directional"]).optional(),
    focus_wrap: z.boolean().optional(),
    focus_follows_pointer: z.boolean().optional(),
    swap_modifier: z.enum(["alt", "control", "meta", "shift", "none"]).optional(),
    history_limit: z.number().int().positive().optional(),
    nav_stack_limit: z.number().int().min(1).max(200).optional(),
    closed_entry_limit: z.number().int().min(1).max(100).optional(),
    desktop_transition: z.enum(["slide", "crossfade", "instant"]).optional(),
    gaps: gapsSchema.optional(),
    shortcuts: shortcutsSchema.optional(),
    global_shortcuts: z.record(z.string(), z.string()).optional(),
    ...retiredConfigKeys,
  })
  .transform((config): WindowManagerWorkspaceConfig => ({
    newWindowPolicy: config.new_window_policy,
    smallViewportPolicy: config.small_viewport_policy,
    focusPolicy: config.focus_policy,
    focusWrap: config.focus_wrap,
    focusFollowsPointer: config.focus_follows_pointer,
    swapModifier: config.swap_modifier,
    historyLimit: config.history_limit,
    navStackLimit: config.nav_stack_limit,
    closedEntryLimit: config.closed_entry_limit,
    desktopTransition: config.desktop_transition,
    gaps: config.gaps,
    shortcuts: config.shortcuts,
    globalShortcuts: config.global_shortcuts,
  }));

export const windowManagerWireConfigSchema = z.strictObject({
  new_window_policy: z.enum(["floating", "beside_focus", "tab"]),
  small_viewport_policy: z.enum(["stack", "reject"]),
  focus_policy: z.enum(["click_directional", "directional"]),
  focus_wrap: z.boolean(),
  focus_follows_pointer: z.boolean(),
  swap_modifier: z.enum(["alt", "control", "meta", "shift", "none"]),
  history_limit: z.number().int().min(1).max(500),
  nav_stack_limit: z.number().int().min(1).max(200),
  closed_entry_limit: z.number().int().min(1).max(100),
  desktop_transition: z.enum(["slide", "crossfade", "instant"]),
  gaps: gapsSchema,
  shortcuts: shortcutsSchema,
  global_shortcuts: z.record(z.string(), z.string()),
  ...retiredConfigKeys,
});

export function toWindowManagerConfig(
  config: z.output<typeof windowManagerWireConfigSchema>,
  defaults: WindowManagerShortcutMap,
  effective: WindowManagerShortcutMap
): WindowManagerConfig {
  return {
    newWindowPolicy: config.new_window_policy,
    smallViewportPolicy: config.small_viewport_policy,
    focusPolicy: config.focus_policy,
    focusWrap: config.focus_wrap,
    focusFollowsPointer: config.focus_follows_pointer,
    swapModifier: config.swap_modifier,
    historyLimit: config.history_limit,
    navStackLimit: config.nav_stack_limit,
    closedEntryLimit: config.closed_entry_limit,
    desktopTransition: config.desktop_transition,
    gaps: config.gaps,
    shortcuts: config.shortcuts,
    shortcutDefaults: defaults,
    effectiveShortcuts: effective,
    globalShortcuts: config.global_shortcuts,
  };
}
