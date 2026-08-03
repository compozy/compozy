import { useEffect, useEffectEvent } from "react";

import { dispatchWindowManagerAction } from "../lib/window-manager-action-dispatch";
import {
  primaryShortcutModifier,
  resolveWindowManagerActions,
  shortcutMatches,
} from "../lib/window-manager-shortcuts";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import type { WindowManagerActionId } from "../lib/window-manager-command-registry";
import { windowManagerStore } from "../stores/window-manager-store";
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

function isGlobalTabAction(actionId: WindowManagerActionId): boolean {
  return actionId === "window.close" || actionId.startsWith("window.tab.");
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
    const editableTarget = isEditableTarget(event.target);
    const primaryModifier = primaryShortcutModifier(navigator.platform);
    if (config !== null && windowManagerCommandsAvailable(state)) {
      const action = resolveWindowManagerActions(config.shortcuts).find(
        candidate => candidate.chord && shortcutMatches(event, candidate.chord, primaryModifier)
      );
      if (
        action &&
        (!editableTarget || isGlobalTabAction(action.id)) &&
        (!action.needsFocusedWindow || state.focusedId !== null)
      ) {
        event.preventDefault();
        dispatchWindowManagerAction(action.id, {
          manager,
          state,
          openDesktops: handlers.onDesktops,
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
      // ⌘[ pops the active tab's own stack (D7). The chord is fixed: the Go
      // rebinding allow-list deliberately excludes navigation pops.
      if (
        !editableTarget &&
        event.metaKey &&
        !event.ctrlKey &&
        !event.altKey &&
        !event.shiftKey &&
        event.code === "BracketLeft" &&
        state.focusedId !== null
      ) {
        event.preventDefault();
        void manager.popWindowRoute(state.focusedId).completion;
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
