const MODIFIERS = new Map([
  ["meta", "CommandOrControl"],
  ["ctrl", "Control"],
  ["alt", "Alt"],
  ["shift", "Shift"],
] as const);

const NAMED_KEYS = new Map([
  ["Space", "Space"],
  ["Enter", "Enter"],
  ["Escape", "Esc"],
  ["Tab", "Tab"],
  ["Backspace", "Backspace"],
  ["Delete", "Delete"],
  ["ArrowUp", "Up"],
  ["ArrowDown", "Down"],
  ["ArrowLeft", "Left"],
  ["ArrowRight", "Right"],
  ["Home", "Home"],
  ["End", "End"],
  ["PageUp", "PageUp"],
  ["PageDown", "PageDown"],
] as const);

export class UnconvertibleShortcutError extends Error {
  readonly chord: string;

  constructor(chord: string) {
    super(`The shortcut chord ${JSON.stringify(chord)} cannot be registered by Electron.`);
    this.name = "UnconvertibleShortcutError";
    this.chord = chord;
  }
}

function acceleratorKey(token: string): string | null {
  const named = NAMED_KEYS.get(token as never);
  if (named) return named;
  if (/^Key[A-Z]$/u.test(token)) return token.slice(3);
  if (/^Digit[0-9]$/u.test(token)) return token.slice(5);
  if (/^F(?:[1-9]|1[0-9]|2[0-4])$/u.test(token)) return token;
  return null;
}

export function chordToAccelerator(chord: string): string {
  const tokens = chord.split("+");
  if (tokens.length < 2 || tokens.some(token => token.length === 0)) {
    throw new UnconvertibleShortcutError(chord);
  }
  const keyToken = tokens.at(-1);
  if (!keyToken) throw new UnconvertibleShortcutError(chord);
  const key = acceleratorKey(keyToken);
  if (!key) throw new UnconvertibleShortcutError(chord);
  const modifiers: string[] = [];
  const seen = new Set<string>();
  for (const token of tokens.slice(0, -1)) {
    const modifier = MODIFIERS.get(token as never);
    if (!modifier || seen.has(modifier)) throw new UnconvertibleShortcutError(chord);
    seen.add(modifier);
    modifiers.push(modifier);
  }
  if (modifiers.length === 0) throw new UnconvertibleShortcutError(chord);
  return [...modifiers, key].join("+");
}
