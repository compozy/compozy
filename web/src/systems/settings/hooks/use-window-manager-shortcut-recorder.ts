import { useEffect, useState } from "react";

import {
  chordFromKeyboardEvent,
  findShortcutConflicts,
  type ShortcutConflict,
  type ShortcutMap,
  type WindowManagerActionId,
  type WindowManagerShortcutMap,
} from "@/systems/os";

type RecorderMode = "replace" | "alternate";

export interface ShortcutRecorderModel {
  recording: WindowManagerActionId | null;
  recordingMode: RecorderMode | null;
  announcement: string;
  conflicts: readonly ShortcutConflict[];
  blocked: ReadonlySet<WindowManagerActionId>;
  shadowed: ReadonlySet<WindowManagerActionId>;
  start: (actionId: WindowManagerActionId, mode?: RecorderMode) => void;
  cancel: () => void;
  reset: (actionId: WindowManagerActionId) => void;
  resetAll: () => void;
}

function sameBinding(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

/** Records primary and alternate chords without creating redundant default overrides. */
export function useWindowManagerShortcutRecorder(
  overrides: WindowManagerShortcutMap,
  defaults: ShortcutMap,
  onChange: (next: WindowManagerShortcutMap) => void
): ShortcutRecorderModel {
  const [recording, setRecording] = useState<WindowManagerActionId | null>(null);
  const [recordingMode, setRecordingMode] = useState<RecorderMode | null>(null);
  const [announcement, setAnnouncement] = useState("");

  useEffect(() => {
    if (recording === null || recordingMode === null) return undefined;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setRecording(null);
        setRecordingMode(null);
        setAnnouncement("Recording cancelled.");
        return;
      }
      const chord = chordFromKeyboardEvent(event);
      if (chord === null) {
        if (!["Meta", "Control", "Alt", "Shift"].includes(event.key)) {
          event.preventDefault();
          setAnnouncement("A shortcut needs at least one modifier key.");
        }
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      const current = overrides[recording] ?? defaults[recording] ?? [];
      const binding = recordingMode === "alternate" ? [...current, chord] : [chord];
      const deduplicated = [...new Set(binding)];
      const next = { ...overrides };
      if (sameBinding(deduplicated, defaults[recording] ?? [])) delete next[recording];
      else next[recording] = deduplicated;
      onChange(next);
      setRecording(null);
      setRecordingMode(null);
      setAnnouncement(`Shortcut set to ${chord.split("+").join(" ")}.`);
    };

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [defaults, onChange, overrides, recording, recordingMode]);

  const conflicts = findShortcutConflicts(overrides, defaults);
  const blocked = new Set<WindowManagerActionId>();
  const shadowed = new Set<WindowManagerActionId>();
  for (const conflict of conflicts) {
    const bucket = conflict.kind === "blocked" ? blocked : shadowed;
    for (const actionId of conflict.actionIds) bucket.add(actionId);
  }

  return {
    recording,
    recordingMode,
    announcement,
    conflicts,
    blocked,
    shadowed,
    start: (actionId, mode = "replace") => {
      setRecording(actionId);
      setRecordingMode(mode);
      setAnnouncement(
        mode === "alternate" ? "Press an alternate shortcut." : "Press the keys you want."
      );
    },
    cancel: () => {
      setRecording(null);
      setRecordingMode(null);
    },
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
