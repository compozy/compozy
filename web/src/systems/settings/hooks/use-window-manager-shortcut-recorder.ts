import { useEffect, useEffectEvent } from "react";
import { useSelector, useStore } from "@xstate/store-react";

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
import {
  shortcutRecorderLogic,
  type ShortcutRecorderConflict,
  type ShortcutRecorderMode,
} from "../stores/window-manager-shortcut-recorder-store";
import type { WindowManagerBindingMutations } from "./use-window-manager-binding-mutations";

export type { ShortcutRecorderConflict };

export interface ShortcutRecorderModel {
  recording: string | null;
  recordingMode: ShortcutRecorderMode | null;
  announcement: string;
  conflict: ShortcutRecorderConflict | null;
  conflicts: readonly ShortcutConflict[];
  error: string | null;
  saving: boolean;
  start: (commandId: string, mode?: ShortcutRecorderMode) => void;
  cancel: () => void;
  overwrite: () => void;
  dismissConflict: () => void;
  reset: (commandId: string) => void;
  resetAll: () => void;
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
  const store = useStore(shortcutRecorderLogic);
  const activeRecording = useSelector(store, snapshot => snapshot.context.recording);
  const announcement = useSelector(store, snapshot => snapshot.context.announcement);
  const conflict = useSelector(store, snapshot => snapshot.context.conflict);
  const error = useSelector(store, snapshot => snapshot.context.error);
  const overrides = section.config.shortcuts;
  const defaults = section.config.shortcutDefaults;
  const { commit } = mutations;
  const titleFor = (commandId: string) =>
    section.commands.find(command => command.id === commandId)?.title ?? commandId;

  const apply = async (desired: WindowManagerShortcutMap, commandId: string, chord: string) => {
    store.trigger.errorCleared();
    try {
      await commit({ shortcuts: desired });
      store.trigger.conflictDismissed();
      store.trigger.announced({ announcement: `Shortcut set to ${shortcutLabel(chord)}.` });
    } catch (cause) {
      if (cause instanceof WindowManagerSettingsError && cause.code === "shortcut_conflict") {
        const owner = cause.owner ?? "";
        store.trigger.conflictSet({
          announcement: `${shortcutLabel(chord)} is already used by ${titleFor(owner)}.`,
          conflict: { commandId, chord, owner, ownerTitle: titleFor(owner), desired },
        });
        return;
      }
      store.trigger.errorSet({
        error: cause instanceof Error ? cause.message : "Unable to save the keyboard shortcut.",
      });
    }
  };

  const onKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (activeRecording === null) return;
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
        store.trigger.announced({ announcement: "A shortcut needs at least one modifier key." });
      }
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    const { commandId, mode } = activeRecording;
    const current =
      mode === "alternate"
        ? (section.config.effectiveShortcuts[commandId] ??
          overrides[commandId] ??
          defaults[commandId] ??
          [])
        : (overrides[commandId] ?? defaults[commandId] ?? []);
    const binding = mode === "alternate" ? [...current, chord] : [chord];
    const deduplicated = [...new Set(binding)];
    const next: Record<string, readonly string[]> = { ...overrides };
    if (sameBinding(deduplicated, defaults[commandId] ?? [])) delete next[commandId];
    else next[commandId] = deduplicated;
    store.trigger.recordingStopped();
    void apply(next, commandId, chord);
  });

  useEffect(() => {
    if (activeRecording === null) return undefined;

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [activeRecording]);

  return {
    recording: activeRecording?.commandId ?? null,
    recordingMode: activeRecording?.mode ?? null,
    announcement,
    conflict,
    conflicts: findShortcutConflicts(overrides, defaults).filter(
      entry => entry.kind === "shadowed"
    ),
    error,
    saving: mutations.saving,
    start: (commandId, mode = "replace") => {
      store.trigger.started({
        announcement:
          mode === "alternate" ? "Press an alternate shortcut." : "Press the keys you want.",
        commandId,
        mode,
      });
    },
    cancel: () => {
      store.trigger.cancelled({ announcement: "" });
    },
    overwrite: () => {
      if (conflict === null) return;
      const { desired, commandId, chord, ownerTitle } = conflict;
      store.trigger.errorCleared();
      void commit({ shortcuts: desired, overwrite: true })
        .then(() => {
          store.trigger.conflictDismissed();
          store.trigger.announced({
            announcement: `${shortcutLabel(chord)} moved to ${titleFor(commandId)}; ${ownerTitle} is unbound.`,
          });
        })
        .catch((cause: unknown) => {
          store.trigger.errorSet({
            error: cause instanceof Error ? cause.message : "Unable to move the shortcut.",
          });
        });
    },
    dismissConflict: () => store.trigger.conflictDismissed(),
    reset: commandId => {
      store.trigger.errorCleared();
      void commit({ shortcuts: withCommandReset(overrides, commandId) })
        .then(() => store.trigger.announced({ announcement: "Shortcut restored to its default." }))
        .catch((cause: unknown) => {
          store.trigger.errorSet({
            error: cause instanceof Error ? cause.message : "Unable to restore the shortcut.",
          });
        });
    },
    resetAll: () => {
      store.trigger.errorCleared();
      void commit({ shortcuts: {} })
        .then(() =>
          store.trigger.announced({ announcement: "Every shortcut restored to its default." })
        )
        .catch((cause: unknown) => {
          store.trigger.errorSet({
            error: cause instanceof Error ? cause.message : "Unable to restore the shortcuts.",
          });
        });
    },
    applyShortcuts: next => {
      store.trigger.errorCleared();
      void commit({ shortcuts: next })
        .then(() => store.trigger.announced({ announcement: "Shortcuts updated." }))
        .catch((cause: unknown) => {
          store.trigger.errorSet({
            error: cause instanceof Error ? cause.message : "Unable to save the shortcuts.",
          });
        });
    },
  };
}
