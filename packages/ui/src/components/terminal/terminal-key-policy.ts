export interface TerminalKeyPolicyOptions {
  /** The emulator has a selection, so the primary copy chord copies it. */
  hasSelection: boolean;
  /** Apple platforms use ⌘ as the primary modifier; Ctrl always belongs to the shell there. */
  apple: boolean;
  /** The host binds this keystroke to one of its own commands from the terminal. */
  isHostShortcut?: (event: KeyboardEvent) => boolean;
}

export function isApplePlatform(platform: string): boolean {
  return /^(?:Mac|iPhone|iPad|iPod)/i.test(platform);
}

/**
 * Whether the emulator lets a key go to the host instead of sending it to the shell.
 *
 * The primary modifier (⌘ on Apple, Ctrl elsewhere) keeps zoom, paste, the palette and,
 * with a selection, copy. Everything else is the host's only when it binds the chord;
 * an unbound chord, and so ⌃K, ⌃V and ⌃0 on macOS, reaches the shell.
 */
export function terminalReleasesKey(
  event: KeyboardEvent,
  { hasSelection, apple, isHostShortcut }: TerminalKeyPolicyOptions
): boolean {
  const primary = apple ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey;
  if (primary && !event.altKey) {
    switch (event.key.toLowerCase()) {
      case "+":
      case "=":
      case "-":
      case "0":
      case "k":
      case "v":
        return true;
      case "c":
        if (hasSelection) return true;
        break;
      default:
        break;
    }
  }
  return isHostShortcut?.(event) ?? false;
}
