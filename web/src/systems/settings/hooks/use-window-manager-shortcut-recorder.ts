import { useEffect, useState } from "react";

import {
  chordFromKeyboardEvent,
  findShortcutConflicts,
  shortcutLabel,
  WindowManagerSettingsError,
  type ShortcutConflict,
  type WindowManagerSettingsSection,
  type WindowManagerShortcutMap,
} from "@/systems/os";

import { withCommandReset } from "../lib/window-manager-shortcut-rows";
import type { WindowManagerBindingMutations } from "./use-window-manager-binding-mutations";

type RecorderMode = "replace" | "alternate";

/** A chord the daemon refused, named as it named it. */
export interface ShortcutRecorderConflict {
  commandId: string;
  chord: string;
  /** Command id holding the chord today. */
  owner: string;
  /** That command's registry title, or its id when the registry lacks it. */
  ownerTitle: string;
  /** The map to re-send once the operator accepts the transfer. */
  desired: WindowManagerShortcutMap;
}

export interface ShortcutRecorderModel {
  recording: string | null;
  recordingMode: RecorderMode | null;
  announcement: string;
  /** Blocked assignment awaiting an explicit overwrite (US-022.AC-2). */
  conflict: ShortcutRecorderConflict | null;
  /** Surface-local shadows — a chord a focused editor wins locally. */
  conflicts: readonly ShortcutConflict[];
  error: string | null;
  saving: boolean;
  start: (commandId: string, mode?: RecorderMode) => void;
  cancel: () => void;
  overwrite: () => void;
  dismissConflict: () => void;
  reset: (commandId: string) => void;
  resetAll: () => void;
  /** Writes a whole override map — the shipped presets' path. */
  applyShortcuts: (next: WindowManagerShortcutMap) => void;
}

function sameBinding(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

/**
 * Records chords straight onto the daemon keymap.
 *
 * Nothing here decides whether a chord is free: the daemon owns that judgement
 * and answers a contested capture by naming the command that holds it, so the
 * block the operator reads is the runtime's own (ADR-006). Overwrite re-sends
 * the same map with the transfer authorized, and the loser comes back unbound
 * in the echoed section rather than being predicted here.
 */
export function useWindowManagerShortcutRecorder(
  section: WindowManagerSettingsSection,
  mutations: WindowManagerBindingMutations
): ShortcutRecorderModel {
  const [recording, setRecording] = useState<string | null>(null);
  const [recordingMode, setRecordingMode] = useState<RecorderMode | null>(null);
  const [announcement, setAnnouncement] = useState("");
  const [conflict, setConflict] = useState<ShortcutRecorderConflict | null>(null);
  const [error, setError] = useState<string | null>(null);
  const overrides = section.config.shortcuts;
  const defaults = section.config.shortcutDefaults;
  const { commit } = mutations;
  const titleFor = (commandId: string) =>
    section.commands.find(command => command.id === commandId)?.title ?? commandId;

  const apply = async (desired: WindowManagerShortcutMap, commandId: string, chord: string) => {
    setError(null);
    try {
      await commit({ shortcuts: desired });
      setConflict(null);
      setAnnouncement(`Shortcut set to ${shortcutLabel(chord)}.`);
    } catch (cause) {
      if (cause instanceof WindowManagerSettingsError && cause.code === "shortcut_conflict") {
        const owner = cause.owner ?? "";
        setConflict({ commandId, chord, owner, ownerTitle: titleFor(owner), desired });
        setAnnouncement(`${shortcutLabel(chord)} is already used by ${titleFor(owner)}.`);
        return;
      }
      setError(cause instanceof Error ? cause.message : "Unable to save the keyboard shortcut.");
    }
  };

  useEffect(() => {
    if (recording === null || recordingMode === null) return undefined;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        // Escape belongs to the recorder while it is armed. Left to travel, the
        // shell's own handler reads it as "leave this surface" and moves focus
        // to the desktop (`use-os-shortcuts.ts`), and a dialog host would close
        // — so cancelling a capture would also cost the operator their place.
        event.preventDefault();
        event.stopPropagation();
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
      const commandId = recording;
      const current = overrides[commandId] ?? defaults[commandId] ?? [];
      const binding = recordingMode === "alternate" ? [...current, chord] : [chord];
      const deduplicated = [...new Set(binding)];
      const next: Record<string, readonly string[]> = { ...overrides };
      // Only real departures from the shipped default are stored; matching the
      // default again drops the override rather than freezing today's value.
      if (sameBinding(deduplicated, defaults[commandId] ?? [])) delete next[commandId];
      else next[commandId] = deduplicated;
      setRecording(null);
      setRecordingMode(null);
      void apply(next, commandId, chord);
    };

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
    // Intentionally re-subscribed every render: the handler writes the override
    // map it closes over, and a captured stale map would silently undo an edit
    // that landed while the recorder was armed.
  });

  return {
    recording,
    recordingMode,
    announcement,
    conflict,
    // Shadowing is client knowledge — which chords a focused surface claims
    // locally — so it stays here while blocking stays with the daemon.
    conflicts: findShortcutConflicts(overrides, defaults).filter(
      entry => entry.kind === "shadowed"
    ),
    error,
    saving: mutations.saving,
    start: (commandId, mode = "replace") => {
      setConflict(null);
      setError(null);
      setRecording(commandId);
      setRecordingMode(mode);
      setAnnouncement(
        mode === "alternate" ? "Press an alternate shortcut." : "Press the keys you want."
      );
    },
    cancel: () => {
      setRecording(null);
      setRecordingMode(null);
    },
    overwrite: () => {
      if (conflict === null) return;
      const { desired, commandId, chord, ownerTitle } = conflict;
      setError(null);
      void commit({ shortcuts: desired, overwrite: true })
        .then(() => {
          setConflict(null);
          setAnnouncement(
            `${shortcutLabel(chord)} moved to ${titleFor(commandId)}; ${ownerTitle} is unbound.`
          );
        })
        .catch((cause: unknown) => {
          setError(cause instanceof Error ? cause.message : "Unable to move the shortcut.");
        });
    },
    dismissConflict: () => setConflict(null),
    reset: commandId => {
      setError(null);
      // A preset may have bound this command through an aggregate range key, so
      // dropping the command's own entry is not enough to restore its default.
      void commit({ shortcuts: withCommandReset(overrides, commandId) })
        .then(() => setAnnouncement("Shortcut restored to its default."))
        .catch((cause: unknown) => {
          setError(cause instanceof Error ? cause.message : "Unable to restore the shortcut.");
        });
    },
    resetAll: () => {
      setError(null);
      void commit({ shortcuts: {} })
        .then(() => setAnnouncement("Every shortcut restored to its default."))
        .catch((cause: unknown) => {
          setError(cause instanceof Error ? cause.message : "Unable to restore the shortcuts.");
        });
    },
    applyShortcuts: next => {
      setError(null);
      void commit({ shortcuts: next })
        .then(() => setAnnouncement("Shortcuts updated."))
        .catch((cause: unknown) => {
          setError(cause instanceof Error ? cause.message : "Unable to save the shortcuts.");
        });
    },
  };
}
