import { z } from "zod";

import { shortcutBindingProblem, type ShortcutBinding } from "./window-manager-shortcuts";
import type {
  WindowManagerBindingsConfig,
  WindowManagerConfig,
  WindowManagerShortcutMap,
  WindowManagerSnapConfig,
  WindowManagerWorkspaceConfig,
} from "./window-manager-types";

const gapsSchema = z.strictObject({
  inner: z.number().finite().nonnegative(),
  top: z.number().finite().nonnegative(),
  right: z.number().finite().nonnegative(),
  bottom: z.number().finite().nonnegative(),
  left: z.number().finite().nonnegative(),
});

const snapSchema = z
  .strictObject({
    edge_band: z.number().finite().positive(),
    corner_reach: z.number().finite().positive(),
    exit_slack: z.number().finite().nonnegative(),
    repeat_ratios: z.array(z.number().finite().min(0.1).max(0.9)).min(1),
  })
  .transform(
    (snap): WindowManagerSnapConfig => ({
      edgeBand: snap.edge_band,
      cornerReach: snap.corner_reach,
      exitSlack: snap.exit_slack,
      repeatRatios: snap.repeat_ratios,
    })
  );

const bindingsSchema = z
  .strictObject({
    top_center: z.enum(["none", "reserved", "zoom"]),
    bottom_center: z.enum(["none", "reserved", "zoom"]),
  })
  .transform(
    (bindings): WindowManagerBindingsConfig => ({
      topCenter: bindings.top_center,
      bottomCenter: bindings.bottom_center,
    })
  );

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
    new_window_policy: z.enum(["floating", "beside_focus"]).optional(),
    small_viewport_policy: z.enum(["stack", "reject"]).optional(),
    focus_policy: z.enum(["click_directional", "directional"]).optional(),
    focus_wrap: z.boolean().optional(),
    focus_follows_pointer: z.boolean().optional(),
    raise_on_focus: z.boolean().optional(),
    drag_away_policy: z.enum(["window", "group"]).optional(),
    group_move_modifier: z.enum(["alt", "control", "meta", "shift", "none"]).optional(),
    swap_modifier: z.enum(["alt", "control", "meta", "shift", "none"]).optional(),
    history_limit: z.number().int().positive().optional(),
    nav_stack_limit: z.number().int().min(1).max(200).optional(),
    closed_entry_limit: z.number().int().min(1).max(100).optional(),
    desktop_transition: z.enum(["slide", "crossfade", "instant"]).optional(),
    gaps: gapsSchema.optional(),
    snap: snapSchema.optional(),
    bindings: bindingsSchema.optional(),
    shortcuts: shortcutsSchema.optional(),
  })
  .transform(
    (config): WindowManagerWorkspaceConfig => ({
      newWindowPolicy: config.new_window_policy,
      smallViewportPolicy: config.small_viewport_policy,
      focusPolicy: config.focus_policy,
      focusWrap: config.focus_wrap,
      focusFollowsPointer: config.focus_follows_pointer,
      raiseOnFocus: config.raise_on_focus,
      dragAwayPolicy: config.drag_away_policy,
      groupMoveModifier: config.group_move_modifier,
      swapModifier: config.swap_modifier,
      historyLimit: config.history_limit,
      navStackLimit: config.nav_stack_limit,
      closedEntryLimit: config.closed_entry_limit,
      desktopTransition: config.desktop_transition,
      gaps: config.gaps,
      snap: config.snap,
      bindings: config.bindings,
      shortcuts: config.shortcuts,
    })
  );

export const windowManagerWireConfigSchema = z.strictObject({
  new_window_policy: z.enum(["floating", "beside_focus"]),
  small_viewport_policy: z.enum(["stack", "reject"]),
  focus_policy: z.enum(["click_directional", "directional"]),
  focus_wrap: z.boolean(),
  focus_follows_pointer: z.boolean(),
  raise_on_focus: z.boolean(),
  drag_away_policy: z.enum(["window", "group"]),
  group_move_modifier: z.enum(["alt", "control", "meta", "shift", "none"]),
  swap_modifier: z.enum(["alt", "control", "meta", "shift", "none"]),
  history_limit: z.number().int().min(1).max(500),
  nav_stack_limit: z.number().int().min(1).max(200),
  closed_entry_limit: z.number().int().min(1).max(100),
  desktop_transition: z.enum(["slide", "crossfade", "instant"]),
  gaps: gapsSchema,
  snap: snapSchema,
  bindings: bindingsSchema,
  shortcuts: shortcutsSchema,
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
    raiseOnFocus: config.raise_on_focus,
    dragAwayPolicy: config.drag_away_policy,
    groupMoveModifier: config.group_move_modifier,
    swapModifier: config.swap_modifier,
    historyLimit: config.history_limit,
    navStackLimit: config.nav_stack_limit,
    closedEntryLimit: config.closed_entry_limit,
    desktopTransition: config.desktop_transition,
    gaps: config.gaps,
    snap: config.snap,
    bindings: config.bindings,
    shortcuts: config.shortcuts,
    shortcutDefaults: defaults,
    effectiveShortcuts: effective,
  };
}
