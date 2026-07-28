import { z } from "zod";

import { isWindowManagerActionId } from "./window-manager-command-registry";
import { parseShortcutChord } from "./window-manager-shortcuts";
import type {
  WindowManagerBindingsConfig,
  WindowManagerConfig,
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

const shortcutsSchema = z.record(z.string(), z.string()).superRefine((shortcuts, context) => {
  for (const [actionId, chord] of Object.entries(shortcuts)) {
    if (!isWindowManagerActionId(actionId)) {
      context.addIssue({
        code: "custom",
        message: `Unknown window-manager action ${actionId}.`,
        path: [actionId],
      });
    }
    if (parseShortcutChord(chord) === null) {
      context.addIssue({
        code: "custom",
        message: "Shortcut must contain modifiers plus one KeyboardEvent.code.",
        path: [actionId],
      });
    }
  }
});

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
      desktopTransition: config.desktop_transition,
      gaps: config.gaps,
      snap: config.snap,
      bindings: config.bindings,
      shortcuts: config.shortcuts,
    })
  );

export const windowManagerConfigSchema = z
  .strictObject({
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
    desktop_transition: z.enum(["slide", "crossfade", "instant"]),
    gaps: gapsSchema,
    snap: snapSchema,
    bindings: bindingsSchema,
    shortcuts: shortcutsSchema,
  })
  .transform(
    (config): WindowManagerConfig => ({
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
      desktopTransition: config.desktop_transition,
      gaps: config.gaps,
      snap: config.snap,
      bindings: config.bindings,
      shortcuts: config.shortcuts,
    })
  );

const settingsWindowManagerResponseSchema = z.strictObject({
  section: z.literal("window-manager"),
  scope: z.literal("global"),
  available_scopes: z.tuple([z.literal("global")]),
  config: windowManagerConfigSchema,
});

export function parseSettingsWindowManagerConfig(value: unknown): WindowManagerConfig {
  return settingsWindowManagerResponseSchema.parse(value).config;
}
