import { useEffect, useState } from "react";

import {
  WINDOW_MANAGER_ACTIONS,
  chordFromKeyboardEvent,
  findShortcutConflicts,
  type ShortcutConflict,
  type WindowManagerActionId,
} from "@/systems/os";

export interface ShortcutRecorderModel {
  recording: WindowManagerActionId | null;
  announcement: string;
  conflicts: readonly ShortcutConflict[];
  /** Actions whose chord the daemon would reject on save. */
  blocked: ReadonlySet<WindowManagerActionId>;
  /** Actions that share a chord with another action's shipped default. */
  shadowed: ReadonlySet<WindowManagerActionId>;
  start: (actionId: WindowManagerActionId) => void;
  cancel: () => void;
  reset: (actionId: WindowManagerActionId) => void;
  resetAll: () => void;
}

/**
 * Resolved on first use, not at module load: the settings barrel and the OS
 * barrel import each other (the OS settings window renders these pages), so
 * reading an OS constant while this module initializes finds it undefined.
 */
let defaultChords: Map<string, string | undefined> | null = null;

function defaultChordFor(actionId: WindowManagerActionId): string | undefined {
  defaultChords ??= new Map(WINDOW_MANAGER_ACTIONS.map(action => [action.id, action.defaultChord]));
  return defaultChords.get(actionId);
}

/**
 * Records one chord at a time from real keypresses, applying the daemon's own
 * grammar as it reads them: a bare key or a modifier-only press is not a chord,
 * so nothing is stored that `CanonicalShortcuts` would then reject.
 *
 * Recording an action's shipped default removes the override rather than
 * storing a duplicate of it — only the differences are persisted.
 */
export function useWindowManagerShortcutRecorder(
  overrides: Readonly<Record<string, string>>,
  onChange: (next: Record<string, string>) => void
): ShortcutRecorderModel {
  const [recording, setRecording] = useState<WindowManagerActionId | null>(null);
  const [announcement, setAnnouncement] = useState("");

  useEffect(() => {
    if (recording === null) return undefined;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setRecording(null);
        setAnnouncement("Recording cancelled.");
        return;
      }
      const chord = chordFromKeyboardEvent(event);
      if (chord === null) {
        // Modifier-only presses are the user still building the combination;
        // a bare key is a real refusal worth explaining.
        if (!["Meta", "Control", "Alt", "Shift"].includes(event.key)) {
          event.preventDefault();
          setAnnouncement("A shortcut needs at least one modifier key.");
        }
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      const next = { ...overrides };
      if (chord === defaultChordFor(recording)) delete next[recording];
      else next[recording] = chord;
      onChange(next);
      setRecording(null);
      setAnnouncement(`Shortcut set to ${chord.split("+").join(" ")}.`);
    };

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [recording, overrides, onChange]);

  const conflicts = findShortcutConflicts(overrides);
  const blocked = new Set<WindowManagerActionId>();
  const shadowed = new Set<WindowManagerActionId>();
  for (const conflict of conflicts) {
    const bucket = conflict.kind === "override" ? blocked : shadowed;
    for (const actionId of conflict.actionIds) bucket.add(actionId);
  }

  return {
    recording,
    announcement,
    conflicts,
    blocked,
    shadowed,
    start: actionId => {
      setRecording(actionId);
      setAnnouncement("Press the keys you want.");
    },
    cancel: () => setRecording(null),
    reset: actionId => {
      const next = { ...overrides };
      delete next[actionId];
      onChange(next);
      setAnnouncement("Shortcut restored to its default.");
    },
    resetAll: () => {
      onChange({});
      setAnnouncement("Every shortcut restored to its default.");
    },
  };
}
