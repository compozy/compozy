import {
  WINDOW_MANAGER_ACTIONS,
  type WindowManagerActionDefinition,
  type WindowManagerActionId,
} from "./window-manager-command-registry";

type ShortcutModifier = "meta" | "control" | "alt" | "shift";

export interface ParsedShortcutChord {
  modifiers: ReadonlySet<ShortcutModifier>;
  code: string;
  canonical: string;
}

export interface ResolvedWindowManagerAction extends WindowManagerActionDefinition {
  chord: ParsedShortcutChord | null;
  shortcutLabel: string | null;
}

/**
 * `override` — two stored overrides share a chord, which `CanonicalShortcuts`
 * rejects, so the save fails. `default` — one override lands on another action's
 * shipped chord; the daemon stores it and the shell resolves whichever action
 * comes first in `WINDOW_MANAGER_ACTIONS`.
 */
export type ShortcutConflictKind = "override" | "default";

export interface ShortcutConflict {
  chord: string;
  kind: ShortcutConflictKind;
  actionIds: readonly WindowManagerActionId[];
}

const MODIFIER_ORDER = ["meta", "control", "alt", "shift"] as const;
const MODIFIER_KEYS = new Set(["Meta", "Control", "Alt", "Shift"]);
const KEY_CODE_PATTERN =
  /^(?:Key[A-Z]|Digit[0-9]|Arrow(?:Left|Right|Up|Down)|Bracket(?:Left|Right)|Comma|Period|Slash|Semicolon|Quote|Backquote|Minus|Equal|Backslash|Enter|Space|Tab|Escape|Backspace|Delete|Home|End|PageUp|PageDown|F(?:[1-9]|1[0-2]))$/;

export function parseShortcutChord(value: string): ParsedShortcutChord | null {
  const tokens = value
    .split("+")
    .map(token => token.trim())
    .filter(Boolean);
  if (tokens.length < 2) return null;
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
  const ordered = MODIFIER_ORDER.filter(modifier => modifiers.has(modifier));
  return { modifiers, code, canonical: [...ordered, code].join("+") };
}

export function shortcutLabel(chord: ParsedShortcutChord): string {
  const modifierLabels = { meta: "⌘", control: "⌃", alt: "⌥", shift: "⇧" };
  const codeLabel = chord.code
    .replace(/^Key/, "")
    .replace(/^Digit/, "")
    .replace("ArrowLeft", "←")
    .replace("ArrowRight", "→")
    .replace("ArrowUp", "↑")
    .replace("ArrowDown", "↓")
    .replace("BracketLeft", "[")
    .replace("BracketRight", "]");
  const labels: string[] = [];
  for (const modifier of MODIFIER_ORDER) {
    if (chord.modifiers.has(modifier)) labels.push(modifierLabels[modifier]);
  }
  return [...labels, codeLabel].join("");
}

export function resolveWindowManagerActions(
  overrides: Readonly<Record<string, string>>
): readonly ResolvedWindowManagerAction[] {
  return WINDOW_MANAGER_ACTIONS.map(action => {
    const raw = overrides[action.id] ?? action.defaultChord;
    const chord = raw ? parseShortcutChord(raw) : null;
    return { ...action, chord, shortcutLabel: chord ? shortcutLabel(chord) : null };
  });
}

/**
 * Reads one keypress as a chord, applying the daemon's grammar
 * (`canonicalShortcutChord`): at least one modifier, exactly one allowlisted
 * `KeyboardEvent.code`. Returns null for a modifier-only press so a recorder can
 * keep listening while the user builds the combination.
 */
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

/**
 * Groups every action that resolves to the same chord. `override` conflicts
 * block the save; `default` conflicts are shadowing the caller should explain.
 */
export function findShortcutConflicts(
  overrides: Readonly<Record<string, string>>
): readonly ShortcutConflict[] {
  const byChord = new Map<string, WindowManagerActionId[]>();
  for (const action of resolveWindowManagerActions(overrides)) {
    if (!action.chord) continue;
    const holders = byChord.get(action.chord.canonical);
    if (holders) holders.push(action.id);
    else byChord.set(action.chord.canonical, [action.id]);
  }
  const conflicts: ShortcutConflict[] = [];
  for (const [chord, actionIds] of byChord) {
    if (actionIds.length < 2) continue;
    const overridden = actionIds.filter(id => overrides[id] != null).length;
    conflicts.push({ chord, kind: overridden >= 2 ? "override" : "default", actionIds });
  }
  return conflicts;
}

export function shortcutMatches(event: KeyboardEvent, chord: ParsedShortcutChord): boolean {
  return (
    event.code === chord.code &&
    event.metaKey === chord.modifiers.has("meta") &&
    event.ctrlKey === chord.modifiers.has("control") &&
    event.altKey === chord.modifiers.has("alt") &&
    event.shiftKey === chord.modifiers.has("shift")
  );
}
