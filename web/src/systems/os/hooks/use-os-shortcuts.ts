import { useEffect, useEffectEvent } from "react";

import { dispatchWindowManagerAction } from "../lib/window-manager-action-dispatch";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import type { ResolvedWindowManagerAction } from "../lib/window-manager-shortcuts";
import {
  primaryShortcutModifier,
  resolveWindowManagerActions,
  shortcutMatches,
} from "../lib/window-manager-shortcuts";
import { windowManagerStore } from "../stores/window-manager-store";
import { useOsShell } from "./use-os-shell";

export interface OsShortcutHandlers {
  onPalette: () => void;
  onPaletteSessions: () => void;
  onNewSession: () => void;
  onDesktops: () => void;
  onWorkspaces: () => void;
  onCycleWorkspace: (direction: "previous" | "next") => void;
  onCycleSession: (direction: "previous" | "next") => void;
  onFocusAttention: () => void;
  onToggleSidebar: () => void;
  onCheatsheet: () => void;
  onEscape: () => void;
  onToggleGlobalScope: () => void;
}

export interface OsShortcutOptions {
  /** Off while a blocking surface owns the shell (first-run setup). */
  enabled?: boolean;
}

/** AltGr aliases ⌃⌥ on some layouts — never steal keystrokes from text entry. */
function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false;
  const editable = target.closest("input, textarea, select, [contenteditable]");
  if (!(editable instanceof HTMLElement)) return false;
  return (
    editable.isContentEditable ||
    (editable.hasAttribute("contenteditable") &&
      editable.getAttribute("contenteditable") !== "false") ||
    editable instanceof HTMLInputElement ||
    editable instanceof HTMLTextAreaElement ||
    editable instanceof HTMLSelectElement
  );
}

function allowedInEditable(action: ResolvedWindowManagerAction, chordIndex: number): boolean {
  if (action.allowInEditable) return true;
  if (action.id !== "shortcuts.cheatsheet") return false;
  return action.chords[chordIndex]?.modifiers.has("meta") ?? false;
}

/**
 * One registry-driven listener. Config is read at event time, so Settings
 * changes apply without reload; Escape remains the sole hardcoded shell key.
 */
export function useOsShortcuts(
  handlers: OsShortcutHandlers,
  options: OsShortcutOptions = {}
): void {
  const enabled = options.enabled ?? true;
  const { projection, manager, coordinator } = useOsShell();
  const handleKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (event.repeat) return;
    if (event.key === "Escape") {
      if (event.defaultPrevented) return;
      handlers.onEscape();
      return;
    }
    const state = projection.get();
    const config = state.windowManagerConfig;
    if (config === null) return;
    const editableTarget = isEditableTarget(event.target);
    const primaryModifier = primaryShortcutModifier(navigator.platform);
    const commandsAvailable = windowManagerCommandsAvailable(state);
    for (const action of resolveWindowManagerActions(config.effectiveShortcuts)) {
      const chordIndex = action.chords.findIndex(chord =>
        shortcutMatches(event, chord, primaryModifier)
      );
      if (chordIndex < 0) continue;
      if (editableTarget && !allowedInEditable(action, chordIndex)) return;
      if (!commandsAvailable && !action.availabilityExempt) return;
      if (action.needsFocusedWindow && state.focusedId === null) return;
      event.preventDefault();
      dispatchWindowManagerAction(action.id, {
        manager,
        state,
        openPalette: handlers.onPalette,
        openPaletteSessions: handlers.onPaletteSessions,
        openNewSession: handlers.onNewSession,
        toggleGlobalScope: handlers.onToggleGlobalScope,
        openCheatsheet: handlers.onCheatsheet,
        openDesktops: handlers.onDesktops,
        openWorkspaces: handlers.onWorkspaces,
        cycleWorkspace: handlers.onCycleWorkspace,
        cycleSession: handlers.onCycleSession,
        focusAttention: handlers.onFocusAttention,
        toggleSidebar: handlers.onToggleSidebar,
        activateWindow: windowId => void coordinator.userFocus(windowId),
        openNewTab: stackTargetWindowId =>
          void coordinator
            .userOpen({
              app: "new-tab",
              ...(stackTargetWindowId ? { stackTargetWindowId } : {}),
            })
            .then(windowId => {
              if (windowId !== null) {
                windowManagerStore.trigger.paletteIntentRequested({
                  intent: { kind: "destination", windowId },
                });
              }
            }),
      });
      return;
    }
  });

  useEffect(() => {
    if (!enabled) return undefined;
    const listener = (event: KeyboardEvent) => handleKeyDown(event);
    document.addEventListener("keydown", listener);
    return () => document.removeEventListener("keydown", listener);
  }, [enabled]);
}
