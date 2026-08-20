import { useEffect } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import {
  chordFromKeyboardEvent,
  shortcutLabel,
  WindowManagerSettingsError,
  type WindowManagerSettingsSection,
} from "@/systems/os";

import {
  globalShortcutRecorderLogic,
  type GlobalShortcutConflict,
} from "../stores/global-shortcut-recorder-store";
import type { WindowManagerBindingMutations } from "./use-window-manager-binding-mutations";

export interface GlobalShortcutRecorderModel {
  recording: string | null;
  announcement: string;
  error: string | null;
  conflict: GlobalShortcutConflict | null;
  saving: boolean;
  start: (commandId: string) => void;
  overwrite: () => void;
  dismissConflict: () => void;
}

export function useGlobalShortcutRecorder(
  section: WindowManagerSettingsSection,
  mutations: WindowManagerBindingMutations
): GlobalShortcutRecorderModel {
  const store = useStore(globalShortcutRecorderLogic);
  const recording = useSelector(store, snapshot => snapshot.context.recording);
  const announcement = useSelector(store, snapshot => snapshot.context.announcement);
  const error = useSelector(store, snapshot => snapshot.context.error);
  const conflict = useSelector(store, snapshot => snapshot.context.conflict);
  const { commit } = mutations;

  useEffect(() => {
    if (recording === null) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        store.trigger.cancelled({ announcement: "Recording cancelled." });
        return;
      }
      const chord = chordFromKeyboardEvent(event);
      if (chord === null) {
        if (!["Meta", "Control", "Alt", "Shift"].includes(event.key)) {
          event.preventDefault();
          store.trigger.announced({
            announcement: "A shortcut needs at least one modifier key.",
          });
        }
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      const commandId = recording;
      const desired = { ...section.config.globalShortcuts, [commandId]: chord };
      store.trigger.recordingStopped();
      store.trigger.errorCleared();
      void commit({ globalShortcuts: desired })
        .then(() => {
          store.trigger.conflictDismissed();
          store.trigger.announced({
            announcement: `Global hotkey set to ${shortcutLabel(chord)}.`,
          });
        })
        .catch((cause: unknown) => {
          if (cause instanceof WindowManagerSettingsError && cause.code === "shortcut_conflict") {
            store.trigger.conflictSet({
              announcement: `${shortcutLabel(chord)} is already used by another command.`,
              conflict: { commandId, chord, owner: cause.owner ?? "", desired },
            });
            return;
          }
          store.trigger.errorSet({
            error: cause instanceof Error ? cause.message : "Unable to save the global hotkey.",
          });
        });
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [commit, recording, section.config.globalShortcuts, store]);

  return {
    recording,
    announcement,
    error,
    conflict,
    saving: mutations.saving,
    start: commandId => {
      store.trigger.started({ announcement: "Press the keys you want.", commandId });
    },
    overwrite: () => {
      if (conflict === null) return;
      store.trigger.errorCleared();
      void commit({ globalShortcuts: conflict.desired, overwrite: true })
        .then(() => {
          store.trigger.conflictDismissed();
          store.trigger.announced({
            announcement: `Global hotkey moved to ${conflict.commandId}.`,
          });
        })
        .catch((cause: unknown) => {
          store.trigger.errorSet({
            error: cause instanceof Error ? cause.message : "Unable to move the global hotkey.",
          });
        });
    },
    dismissConflict: () => store.trigger.conflictDismissed(),
  };
}
