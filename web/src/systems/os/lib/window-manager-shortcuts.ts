import {
  WINDOW_MANAGER_ACTIONS,
  isWindowManagerActionId,
  type WindowManagerActionDefinition,
  type WindowManagerActionId,
  type WindowManagerActionSection,
} from "./window-manager-command-registry";

type ShortcutModifier = "meta" | "control" | "alt" | "shift";
export type PrimaryShortcutModifier = "meta" | "control";
export type ShortcutBinding = readonly string[];
export type ShortcutMap = Readonly<Record<string, ShortcutBinding>>;

export interface ParsedShortcutChord {
  modifiers: ReadonlySet<ShortcutModifier>;
  code: string;
  canonical: string;
}

export interface ResolvedWindowManagerAction extends WindowManagerActionDefinition {
  chords: readonly ParsedShortcutChord[];
  shortcutLabels: readonly string[];
}

export interface ShortcutCheatsheetRow {
  id: string;
  actionIds: readonly WindowManagerActionId[];
  label: string;
  section: WindowManagerActionSection;
  bindings: ShortcutBinding;
  overridden: boolean;
}

export type ShortcutConflictKind = "blocked" | "shadowed";

export interface ShortcutConflict {
  chord: string;
  kind: ShortcutConflictKind;
  actionIds: readonly WindowManagerActionId[];
  winner?: { label: string; surface: string };
}

interface ShortcutRangeFamily {
  action: "desktop.switch" | "window.tab.jump";
  max: 8 | 9;
}

const RANGE_FAMILIES: readonly ShortcutRangeFamily[] = [
  { action: "desktop.switch", max: 9 },
  { action: "window.tab.jump", max: 8 },
];
const MODIFIER_ORDER = ["meta", "control", "alt", "shift"] as const;
const MODIFIER_KEYS = new Set(["Meta", "Control", "Alt", "Shift"]);
const KEY_CODE_PATTERN =
  /^(?:Key[A-Z]|Digit[0-9]|Arrow(?:Left|Right|Up|Down)|Bracket(?:Left|Right)|Comma|Period|Slash|Semicolon|Quote|Backquote|Minus|Equal|Backslash|Enter|Space|Tab|Escape|Backspace|Delete|Home|End|PageUp|PageDown|F(?:[1-9]|1[0-2]))$/;
const SURFACE_LOCAL_CHORDS = new Map([["meta+KeyB", { label: "Bold", surface: "composer" }]]);

export function parseShortcutChord(value: string): ParsedShortcutChord | null {
  const tokens = value.split("+").map(token => token.trim());
  if (tokens.length < 2 || tokens.some(token => token === "")) return null;
  const code = tokens.at(-1) ?? "";
  if (!KEY_CODE_PATTERN.test(code)) return null;
  const rawModifiers = tokens.slice(0, -1).map(token => token.toLowerCase());
  if (
    new Set(rawModifiers).size !== rawModifiers.length ||
    rawModifiers.some(modifier => !MODIFIER_ORDER.includes(modifier as ShortcutModifier))
  ) {
    return null;
  }
  const modifiers = new Set(rawModifiers as ShortcutModifier[]);
  if (modifiers.size === 0) return null;
  const ordered = MODIFIER_ORDER.filter(modifier => modifiers.has(modifier));
  return { modifiers, code, canonical: [...ordered, code].join("+") };
}

export function shortcutKeyGlyphs(
  chord: ParsedShortcutChord | string,
  primaryModifier: PrimaryShortcutModifier = "meta"
): readonly string[] {
  const range = typeof chord === "string" ? parseShortcutRange(chord) : null;
  const parsed =
    typeof chord === "string"
      ? parseShortcutChord(range ? `${range.prefix}${range.start}` : chord)
      : chord;
  if (parsed === null) return [];
  const modifierLabels: Record<ShortcutModifier, string> = {
    meta: "⌘",
    control: "⌃",
    alt: "⌥",
    shift: "⇧",
  };
  const codeLabels: Readonly<Record<string, string>> = {
    ArrowLeft: "←",
    ArrowRight: "→",
    ArrowUp: "↑",
    ArrowDown: "↓",
    BracketLeft: "[",
    BracketRight: "]",
    Space: "␣",
    Enter: "⏎",
    Escape: "⎋",
    Backspace: "⌫",
    Delete: "⌦",
    Tab: "⇥",
  };
  const portablePrimary = parsed.modifiers.has("meta") && !parsed.modifiers.has("control");
  const labels: string[] = [];
  for (const modifier of MODIFIER_ORDER) {
    if (!parsed.modifiers.has(modifier)) continue;
    labels.push(
      modifier === "meta" && portablePrimary && primaryModifier === "control"
        ? modifierLabels.control
        : modifierLabels[modifier]
    );
  }
  const code = range
    ? `${range.start}–${range.end}`
    : (codeLabels[parsed.code] ?? parsed.code.replace(/^Key/, "").replace(/^Digit/, ""));
  return [...labels, code];
}

export function shortcutLabel(
  chord: ParsedShortcutChord | string,
  primaryModifier: PrimaryShortcutModifier = "meta"
): string {
  return shortcutKeyGlyphs(chord, primaryModifier).join("");
}

export function isShortcutOverrideActionId(value: string): boolean {
  return isWindowManagerActionId(value) || RANGE_FAMILIES.some(family => family.action === value);
}

export function shortcutBindingProblem(actionId: string, binding: ShortcutBinding): string | null {
  if (!isShortcutOverrideActionId(actionId)) return `Unknown window-manager action ${actionId}.`;
  if (binding.length === 0 || (binding.length === 1 && binding[0]?.trim() === "")) return null;
  const family = RANGE_FAMILIES.find(candidate => candidate.action === actionId);
  for (const member of binding) {
    if (member.trim() === "") return "Empty array members are not shortcuts; use an empty array.";
    const range = parseShortcutRange(member);
    if (family) {
      if (range === null || range.start < 1 || range.end > family.max || range.start > range.end) {
        return `${actionId} requires matching Digit ranges from 1 through ${family.max}.`;
      }
      continue;
    }
    if (range !== null) return "Range bindings are valid only for indexed shortcut families.";
    if (parseShortcutChord(member) === null) {
      return "Shortcut must contain modifiers plus one KeyboardEvent.code.";
    }
  }
  if (family) {
    const ranges = binding.map(member => parseShortcutRange(member));
    const first = ranges[0];
    if (first && ranges.some(range => range?.start !== first.start || range.end !== first.end)) {
      return "Range alternates must cover the same members.";
    }
  }
  return null;
}

function parseShortcutRange(value: string): { prefix: string; start: number; end: number } | null {
  const match = value.trim().match(/^(.*Digit)([1-9])\.\.([1-9])$/);
  if (!match) return null;
  const [, prefix = "", startRaw = "", endRaw = ""] = match;
  const probe = parseShortcutChord(`${prefix}${startRaw}`);
  if (probe === null) return null;
  return { prefix: probe.canonical.slice(0, -1), start: Number(startRaw), end: Number(endRaw) };
}

export function expandShortcutOverrides(overrides: ShortcutMap): Record<string, ShortcutBinding> {
  const expanded: Record<string, ShortcutBinding> = {};
  for (const [actionId, binding] of Object.entries(overrides)) {
    const family = RANGE_FAMILIES.find(candidate => candidate.action === actionId);
    if (!family) {
      expanded[actionId] = binding.flatMap(member => {
        const parsed = parseShortcutChord(member);
        return parsed ? [parsed.canonical] : [];
      });
      continue;
    }
    for (const raw of binding) {
      const range = parseShortcutRange(raw);
      if (!range) continue;
      for (let slot = range.start; slot <= range.end && slot <= family.max; slot += 1) {
        const memberId = `${family.action}.${slot}`;
        expanded[memberId] = [...(expanded[memberId] ?? []), `${range.prefix}${slot}`];
      }
    }
  }
  return expanded;
}

export function effectiveShortcutMap(base: ShortcutMap, overrides: ShortcutMap): ShortcutMap {
  const effective: Record<string, ShortcutBinding> = Object.fromEntries(
    Object.entries(base).map(([actionId, binding]) => [actionId, [...binding]])
  );
  for (const family of RANGE_FAMILIES) {
    if (!(family.action in overrides)) continue;
    for (let slot = 1; slot <= family.max; slot += 1) delete effective[`${family.action}.${slot}`];
  }
  for (const [actionId, binding] of Object.entries(expandShortcutOverrides(overrides))) {
    effective[actionId] = [...binding];
  }
  return effective;
}

export function resolveWindowManagerActions(
  effective: ShortcutMap,
  primaryModifier: PrimaryShortcutModifier = "meta"
): readonly ResolvedWindowManagerAction[] {
  return WINDOW_MANAGER_ACTIONS.map(action => {
    const chords = (effective[action.id] ?? []).flatMap(raw => {
      const parsed = parseShortcutChord(raw);
      return parsed ? [parsed] : [];
    });
    return {
      ...action,
      chords,
      shortcutLabels: chords.map(chord => shortcutLabel(chord, primaryModifier)),
    };
  });
}

export function shortcutActionLabel(
  effective: ShortcutMap | undefined,
  actionId: WindowManagerActionId,
  platform: string
): string | null {
  const bindings = effective?.[actionId] ?? [];
  if (bindings.length === 0) return null;
  const primaryModifier = primaryShortcutModifier(platform);
  const labels = bindings.flatMap(binding => {
    const label = shortcutLabel(binding, primaryModifier);
    return label ? [label] : [];
  });
  return labels.length > 0 ? labels.join(" / ") : null;
}

/** Collapses indexed families while keeping each effective chord visible once. */
export function deriveShortcutCheatsheet(
  effective: ShortcutMap,
  overrides: ShortcutMap
): readonly ShortcutCheatsheetRow[] {
  const actions = resolveWindowManagerActions(effective);
  const rows: ShortcutCheatsheetRow[] = [];
  for (const action of actions) {
    const family = action.id.startsWith("window.tab.jump.")
      ? ({ action: "window.tab.jump", max: 8 } as const)
      : /^desktop\.switch\.\d$/.test(action.id)
        ? ({ action: "desktop.switch", max: 9 } as const)
        : null;
    if (family) {
      if (!action.id.endsWith(".1")) continue;
      const actionIds = Array.from(
        { length: family.max },
        (_, index) => `${family.action}.${index + 1}` as WindowManagerActionId
      );
      rows.push({
        id: family.action,
        actionIds,
        label: family.action === "window.tab.jump" ? "Jump to tab" : "Switch to desktop",
        section: action.section,
        bindings: compactIndexedBindings(actionIds, effective),
        overridden:
          overrides[family.action] !== undefined ||
          actionIds.some(id => overrides[id] !== undefined),
      });
      continue;
    }
    rows.push({
      id: action.id,
      actionIds: [action.id],
      label: action.label,
      section: action.section,
      bindings: action.chords.map(chord => chord.canonical),
      overridden: overrides[action.id] !== undefined,
    });
  }
  return rows;
}

function compactIndexedBindings(
  actionIds: readonly WindowManagerActionId[],
  effective: ShortcutMap
): ShortcutBinding {
  const maxAlternates = Math.max(0, ...actionIds.map(id => effective[id]?.length ?? 0));
  const compact: string[] = [];
  const seen = new Set<string>();
  const append = (binding: string) => {
    if (seen.has(binding)) return;
    seen.add(binding);
    compact.push(binding);
  };
  for (let alternate = 0; alternate < maxAlternates; alternate += 1) {
    const bindings = actionIds.map(id => effective[id]?.[alternate]);
    const parsed = bindings.map(binding => (binding ? parseShortcutChord(binding) : null));
    const first = parsed[0];
    const uniformRange =
      first !== null &&
      parsed.every((chord, index) => {
        if (chord === null || chord.code !== `Digit${index + 1}`) return false;
        return [...chord.modifiers].sort().join("+") === [...first.modifiers].sort().join("+");
      });
    if (uniformRange) {
      append(`${first.canonical.slice(0, -1)}1..${actionIds.length}`);
      continue;
    }
    for (const binding of bindings) {
      if (binding) append(binding);
    }
  }
  return compact;
}

export function findShortcutConflicts(
  overrides: ShortcutMap,
  defaults: ShortcutMap
): readonly ShortcutConflict[] {
  const effective = effectiveShortcutMap(defaults, overrides);
  const byChord = new Map<string, WindowManagerActionId[]>();
  for (const action of resolveWindowManagerActions(effective)) {
    for (const chord of action.chords) {
      const holders = byChord.get(chord.canonical);
      if (holders) holders.push(action.id);
      else byChord.set(chord.canonical, [action.id]);
    }
  }
  const conflicts: ShortcutConflict[] = [];
  for (const [chord, actionIds] of byChord) {
    if (actionIds.length > 1) conflicts.push({ chord, kind: "blocked", actionIds });
    const winner = SURFACE_LOCAL_CHORDS.get(chord);
    if (winner) conflicts.push({ chord, kind: "shadowed", actionIds, winner });
  }
  return conflicts;
}

export function chordFromKeyboardEvent(event: KeyboardEvent): string | null {
  if (MODIFIER_KEYS.has(event.key)) return null;
  if (!KEY_CODE_PATTERN.test(event.code)) return null;
  const held: ShortcutModifier[] = [];
  if (event.metaKey) held.push("meta");
  if (event.ctrlKey) held.push("control");
  if (event.altKey) held.push("alt");
  if (event.shiftKey) held.push("shift");
  if (held.length === 0) return null;
  const ordered = MODIFIER_ORDER.filter(modifier => held.includes(modifier));
  return [...ordered, event.code].join("+");
}

/** `meta` is Command on Apple and the portable primary modifier elsewhere. */
export function primaryShortcutModifier(platform: string): PrimaryShortcutModifier {
  if (platform.trim() === "") return "meta";
  return /^(?:Mac|iPhone|iPad|iPod)/i.test(platform) ? "meta" : "control";
}

export function shortcutMatches(
  event: KeyboardEvent,
  chord: ParsedShortcutChord,
  primaryModifier: PrimaryShortcutModifier = "meta"
): boolean {
  const portablePrimary = chord.modifiers.has("meta") && !chord.modifiers.has("control");
  const expectsMeta = portablePrimary ? primaryModifier === "meta" : chord.modifiers.has("meta");
  const expectsControl = portablePrimary
    ? primaryModifier === "control"
    : chord.modifiers.has("control");
  return (
    event.code === chord.code &&
    event.metaKey === expectsMeta &&
    event.ctrlKey === expectsControl &&
    event.altKey === chord.modifiers.has("alt") &&
    event.shiftKey === chord.modifiers.has("shift")
  );
}
