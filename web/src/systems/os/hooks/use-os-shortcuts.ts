import { useEffect, useEffectEvent } from "react";

import { dispatchWindowManagerAction } from "../lib/window-manager-action-dispatch";
import { resolveWindowManagerActions, shortcutMatches } from "../lib/window-manager-shortcuts";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { useOsShell } from "./use-os-shell";

export interface OsShortcutHandlers {
  onPalette: () => void;
  onNewSession: () => void;
  onDesktops: () => void;
  onEscape: () => void;
}

export interface OsShortcutOptions {
  /** Off while a blocking surface owns the shell (first-run setup). */
  enabled?: boolean;
}

function isPlainMod(event: KeyboardEvent): boolean {
  return (event.metaKey || event.ctrlKey) && !event.altKey && !event.shiftKey;
}

/** AltGr aliases ⌃⌥ on some layouts — never steal keystrokes from text entry. */
function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement
  );
}

/**
 * The shell's global shortcut set (ADR-005): ⌘K palette (owned here — the old
 * RuntimeSelector registry is deleted), ⌘N new session, Esc focus return,
 * and the live window-manager action registry. Config overrides are read at
 * event time, so a successful Settings apply changes dispatch immediately.
 *
 * `enabled: false` unbinds the listener entirely: ⌘K and ⌘N must not fire behind
 * a blocking panel, and `inert` alone does not stop document-level keydown.
 */
export function useOsShortcuts(
  handlers: OsShortcutHandlers,
  options: OsShortcutOptions = {}
): void {
  const enabled = options.enabled ?? true;
  const { store, manager } = useOsShell();
  const handleKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (event.key === "Escape") {
      handlers.onEscape();
      return;
    }
    const state = store.getState();
    const config = state.windowManagerConfig;
    if (
      config !== null &&
      windowManagerCommandsAvailable(state) &&
      !isEditableTarget(event.target) &&
      !event.repeat
    ) {
      const action = resolveWindowManagerActions(config.shortcuts).find(
        candidate => candidate.chord && shortcutMatches(event, candidate.chord)
      );
      if (action && (!action.needsFocusedWindow || state.focusedId !== null)) {
        event.preventDefault();
        dispatchWindowManagerAction(action.id, {
          manager,
          state,
          openDesktops: handlers.onDesktops,
        });
        return;
      }
    }
    if (!isPlainMod(event)) return;
    const key = event.key.toLowerCase();
    if (key === "k") {
      event.preventDefault();
      handlers.onPalette();
      return;
    }
    if (key === "n") {
      event.preventDefault();
      handlers.onNewSession();
    }
  });

  useEffect(() => {
    if (!enabled) return undefined;
    const listener = (event: KeyboardEvent) => handleKeyDown(event);
    document.addEventListener("keydown", listener);
    return () => document.removeEventListener("keydown", listener);
  }, [enabled]);
}
