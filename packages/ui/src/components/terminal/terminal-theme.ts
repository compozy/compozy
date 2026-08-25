/**
 * Token → emulator theme bridge.
 *
 * The emulator cannot read CSS custom properties, so the ramp declared in
 * `tokens.css` is resolved through `getComputedStyle` and handed over as plain
 * values. A token that resolves to nothing is omitted rather than guessed: the
 * emulator's own default is honest, an invented hex is not.
 */

/** The emulator theme keys this bridge fills. Mirrors xterm's `ITheme` subset. */
export interface TerminalThemeColors {
  background?: string;
  foreground?: string;
  cursor?: string;
  cursorAccent?: string;
  selectionBackground?: string;
  black?: string;
  red?: string;
  green?: string;
  yellow?: string;
  blue?: string;
  magenta?: string;
  cyan?: string;
  white?: string;
  brightBlack?: string;
  brightRed?: string;
  brightGreen?: string;
  brightYellow?: string;
  brightBlue?: string;
  brightMagenta?: string;
  brightCyan?: string;
  brightWhite?: string;
}

/** Emulator theme key → the custom property that owns its value. */
const TERMINAL_THEME_TOKENS: ReadonlyArray<readonly [keyof TerminalThemeColors, string]> = [
  ["background", "--terminal-bg"],
  ["foreground", "--terminal-fg"],
  ["cursor", "--terminal-cursor"],
  ["cursorAccent", "--terminal-bg"],
  ["selectionBackground", "--terminal-selection"],
  ["black", "--terminal-ansi-0"],
  ["red", "--terminal-ansi-1"],
  ["green", "--terminal-ansi-2"],
  ["yellow", "--terminal-ansi-3"],
  ["blue", "--terminal-ansi-4"],
  ["magenta", "--terminal-ansi-5"],
  ["cyan", "--terminal-ansi-6"],
  ["white", "--terminal-ansi-7"],
  ["brightBlack", "--terminal-ansi-8"],
  ["brightRed", "--terminal-ansi-9"],
  ["brightGreen", "--terminal-ansi-10"],
  ["brightYellow", "--terminal-ansi-11"],
  ["brightBlue", "--terminal-ansi-12"],
  ["brightMagenta", "--terminal-ansi-13"],
  ["brightCyan", "--terminal-ansi-14"],
  ["brightWhite", "--terminal-ansi-15"],
];

/**
 * Resolves the emulator palette from the tokens in scope at `element`.
 *
 * Reading through the mounted element rather than `document.documentElement`
 * means a scoped token override — a story decorator, a themed subtree — is
 * honoured without the bridge knowing anything about it.
 */
export function readTerminalTheme(element: HTMLElement): TerminalThemeColors {
  const view = element.ownerDocument?.defaultView;
  if (!view) return {};
  const computed = view.getComputedStyle(element);
  const theme: TerminalThemeColors = {};
  for (const [key, token] of TERMINAL_THEME_TOKENS) {
    const value = computed.getPropertyValue(token).trim();
    if (value) theme[key] = value;
  }
  return theme;
}

/** True when two resolved palettes would paint the same screen. */
export function terminalThemesEqual(
  left: TerminalThemeColors,
  right: TerminalThemeColors
): boolean {
  return TERMINAL_THEME_TOKENS.every(([key]) => left[key] === right[key]);
}

/**
 * Re-reads the palette whenever the document's theme carriers change.
 *
 * A theme switch lands as a class or inline custom-property change on the root
 * element; observing both is what makes the emulator follow a switch instead of
 * keeping the palette it booted with.
 */
export function observeTerminalTheme(element: HTMLElement, onChange: () => void): () => void {
  const view = element.ownerDocument?.defaultView;
  const root = element.ownerDocument?.documentElement;
  if (!view || !root || typeof view.MutationObserver !== "function") {
    return () => undefined;
  }
  const observer = new view.MutationObserver(() => onChange());
  observer.observe(root, { attributes: true, attributeFilter: ["class", "style"] });
  return () => observer.disconnect();
}
