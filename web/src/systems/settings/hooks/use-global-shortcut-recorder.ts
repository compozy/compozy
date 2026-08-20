import { useEffect, useState } from "react";

import {
  chordFromKeyboardEvent,
  shortcutLabel,
  WindowManagerSettingsError,
  type WindowManagerGlobalShortcutMap,
  type WindowManagerSettingsSection,
} from "@/systems/os";

import type { WindowManagerBindingMutations } from "./use-window-manager-binding-mutations";

interface GlobalShortcutConflict {
  commandId: string;
  chord: string;
  owner: string;
  desired: WindowManagerGlobalShortcutMap;
}

export interface GlobalShortcutRecorderModel {
  recording: string | null;
  announcement: string;
  error: string | null;
  conflict: GlobalShortcutConflict | null;
  saving: boolean;
  start: (commandId: string) => void;
  cancel: () => void;
  overwrite: () => void;
  dismissConflict: () => void;
}

export function useGlobalShortcutRecorder(
  section: WindowManagerSettingsSection,
  mutations: WindowManagerBindingMutations
): GlobalShortcutRecorderModel {
  const [recording, setRecording] = useState<string | null>(null);
  const [announcement, setAnnouncement] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState<GlobalShortcutConflict | null>(null);
  const { commit } = mutations;

  useEffect(() => {
    if (recording === null) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        setRecording(null);
        setAnnouncement("Recording cancelled.");
        return;
      }
      const chord = chordFromKeyboardEvent(event);
      if (chord === null) return;
      event.preventDefault();
      event.stopPropagation();
      const commandId = recording;
      const desired = { ...section.config.globalShortcuts, [commandId]: chord };
      setRecording(null);
      setError(null);
      void commit({ globalShortcuts: desired })
        .then(() => {
          setConflict(null);
          setAnnouncement(`Global hotkey set to ${shortcutLabel(chord)}.`);
        })
        .catch((cause: unknown) => {
          if (cause instanceof WindowManagerSettingsError && cause.code === "shortcut_conflict") {
            setConflict({ commandId, chord, owner: cause.owner ?? "", desired });
            setAnnouncement(`${shortcutLabel(chord)} is already used by another command.`);
            return;
          }
          setError(cause instanceof Error ? cause.message : "Unable to save the global hotkey.");
        });
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [commit, recording, section.config.globalShortcuts]);

  return {
    recording,
    announcement,
    error,
    conflict,
    saving: mutations.saving,
    start: commandId => {
      setError(null);
      setConflict(null);
      setRecording(commandId);
      setAnnouncement("Press the keys you want.");
    },
    cancel: () => setRecording(null),
    overwrite: () => {
      if (conflict === null) return;
      setError(null);
      void commit({ globalShortcuts: conflict.desired, overwrite: true })
        .then(() => {
          setConflict(null);
          setAnnouncement(`Global hotkey moved to ${conflict.commandId}.`);
        })
        .catch((cause: unknown) => {
          setError(cause instanceof Error ? cause.message : "Unable to move the global hotkey.");
        });
    },
    dismissConflict: () => setConflict(null),
  };
}
