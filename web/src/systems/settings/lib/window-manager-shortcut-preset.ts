import {
  deriveShortcutCheatsheet,
  effectiveShortcutMap,
  findShortcutConflicts,
  type ShortcutActionDefinition,
  type ShortcutConflict,
  type ShortcutMap,
} from "@/systems/os";

export const TERMINAL_SHORTCUT_PRESET: ShortcutMap = {
  "window.focus.left": ["control+ArrowLeft", "alt+KeyH"],
  "window.focus.down": ["control+ArrowDown", "alt+KeyJ"],
  "window.focus.up": ["control+ArrowUp", "alt+KeyK"],
  "window.focus.right": ["control+ArrowRight", "alt+KeyL"],
  "window.focus.last": ["control+alt+KeyO", "alt+KeyO"],
  "window.close": ["meta+KeyW", "alt+KeyW"],
  "window.zoom": ["control+alt+KeyZ", "alt+KeyY"],
  "window.tab.new": ["meta+KeyT", "alt+KeyT"],
  "window.tab.next": ["control+Tab", "control+alt+KeyL"],
  "window.tab.previous": ["control+shift+Tab", "control+alt+KeyH"],
  "window.tab.jump": ["control+alt+Digit1..8"],
  "window.tab.last": ["control+alt+Digit9"],
  "session.cycle.next": ["control+shift+ArrowDown", "control+alt+KeyJ"],
  "session.cycle.previous": ["control+shift+ArrowUp", "control+alt+KeyK"],
  "window.tile.bottom-left": ["control+alt+shift+KeyJ"],
  "window.tile.bottom-right": ["control+alt+shift+KeyK"],
  "session.focus.attention": ["control+alt+KeyA", "alt+KeyI"],
  "workspace.picker": ["meta+shift+KeyO", "meta+control+KeyP"],
  "workspace.cycle.next": ["meta+shift+BracketRight", "meta+control+KeyJ"],
  "workspace.cycle.previous": ["meta+shift+BracketLeft", "meta+control+KeyK"],
  "desktop.create": ["meta+shift+KeyN", "meta+control+KeyN"],
  "desktop.switch": ["control+Digit1..9", "meta+Digit1..9"],
};

export const TERMINAL_SHORTCUT_PRESET_TOML = `[window_manager.shortcuts]
"window.focus.left" = ["control+ArrowLeft", "alt+KeyH"]
"window.focus.down" = ["control+ArrowDown", "alt+KeyJ"]
"window.focus.up" = ["control+ArrowUp", "alt+KeyK"]
"window.focus.right" = ["control+ArrowRight", "alt+KeyL"]
"window.focus.last" = ["control+alt+KeyO", "alt+KeyO"]
"window.close" = ["meta+KeyW", "alt+KeyW"]
"window.zoom" = ["control+alt+KeyZ", "alt+KeyY"]
"window.tab.new" = ["meta+KeyT", "alt+KeyT"]
"window.tab.next" = ["control+Tab", "control+alt+KeyL"]
"window.tab.previous" = ["control+shift+Tab", "control+alt+KeyH"]
"window.tab.jump" = "control+alt+Digit1..8"
"window.tab.last" = "control+alt+Digit9"
"session.cycle.next" = ["control+shift+ArrowDown", "control+alt+KeyJ"]
"session.cycle.previous" = ["control+shift+ArrowUp", "control+alt+KeyK"]
"window.tile.bottom-left" = "control+alt+shift+KeyJ"
"window.tile.bottom-right" = "control+alt+shift+KeyK"
"session.focus.attention" = ["control+alt+KeyA", "alt+KeyI"]
"workspace.picker" = ["meta+shift+KeyO", "meta+control+KeyP"]
"workspace.cycle.next" = ["meta+shift+BracketRight", "meta+control+KeyJ"]
"workspace.cycle.previous" = ["meta+shift+BracketLeft", "meta+control+KeyK"]
"desktop.create" = ["meta+shift+KeyN", "meta+control+KeyN"]
"desktop.switch" = ["control+Digit1..9", "meta+Digit1..9"]`;

export interface ShortcutPresetChange {
  actionId: string;
  before: readonly string[];
  after: readonly string[];
  hazards: readonly ("altgr" | "browser-tabs" | "displaced-default")[];
  conflicts: readonly ShortcutConflict[];
}

export type ShortcutPresetRevertToken = Readonly<Record<string, readonly string[] | null>>;

function sameBinding(left: readonly string[] | undefined, right: readonly string[] | undefined) {
  return JSON.stringify(left ?? []) === JSON.stringify(right ?? []);
}

export function previewTerminalShortcutPreset(
  overrides: ShortcutMap,
  defaults: ShortcutMap,
  actions: readonly ShortcutActionDefinition[]
): readonly ShortcutPresetChange[] {
  const before = effectiveShortcutMap(defaults, overrides);
  const afterOverrides = applyTerminalShortcutPreset(overrides).overrides;
  const after = effectiveShortcutMap(defaults, afterOverrides);
  const conflicts = findShortcutConflicts(afterOverrides, defaults);
  const beforeRows = new Map(
    deriveShortcutCheatsheet(before, overrides, actions).map(row => [row.id, row.bindings])
  );
  const afterRows = new Map(
    deriveShortcutCheatsheet(after, afterOverrides, actions).map(row => [row.id, row.bindings])
  );
  return Object.keys(TERMINAL_SHORTCUT_PRESET).flatMap(actionId => {
    const next = TERMINAL_SHORTCUT_PRESET[actionId] ?? [];
    const beforeBinding = beforeRows.get(actionId) ?? before[actionId] ?? overrides[actionId] ?? [];
    const afterBinding = afterRows.get(actionId) ?? after[actionId] ?? next;
    if (sameBinding(beforeBinding, afterBinding)) return [];
    const hazards: ShortcutPresetChange["hazards"][number][] = [];
    if (beforeBinding.length > 0) hazards.push("displaced-default");
    if (next.some(chord => chord.includes("control+alt+"))) hazards.push("altgr");
    if (next.some(chord => chord.startsWith("control+Digit"))) hazards.push("browser-tabs");
    return [
      {
        actionId,
        before: beforeBinding,
        after: afterBinding,
        hazards,
        conflicts: conflicts.filter(conflict =>
          conflict.actionIds.some(owner => owner === actionId || owner.startsWith(`${actionId}.`))
        ),
      },
    ];
  });
}

export function applyTerminalShortcutPreset(overrides: ShortcutMap): {
  overrides: ShortcutMap;
  changed: boolean;
  revert: ShortcutPresetRevertToken | null;
} {
  const changed = Object.entries(TERMINAL_SHORTCUT_PRESET).some(
    ([actionId, binding]) => !sameBinding(overrides[actionId], binding)
  );
  if (!changed) return { overrides, changed: false, revert: null };
  const revert: Record<string, readonly string[] | null> = {};
  for (const actionId of Object.keys(TERMINAL_SHORTCUT_PRESET)) {
    revert[actionId] = overrides[actionId] ? [...overrides[actionId]] : null;
  }
  return {
    overrides: { ...overrides, ...TERMINAL_SHORTCUT_PRESET },
    changed: true,
    revert,
  };
}

export function revertTerminalShortcutPreset(
  overrides: ShortcutMap,
  token: ShortcutPresetRevertToken
): ShortcutMap {
  const restored: Record<string, readonly string[]> = { ...overrides };
  for (const [actionId, binding] of Object.entries(token)) {
    if (binding === null) delete restored[actionId];
    else restored[actionId] = [...binding];
  }
  return restored;
}
