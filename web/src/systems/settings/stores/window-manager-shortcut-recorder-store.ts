import { createStoreLogic } from "@xstate/store";

import type { WindowManagerShortcutMap } from "@/systems/os";

export type ShortcutRecorderMode = "replace" | "alternate";

export interface ShortcutRecorderConflict {
  commandId: string;
  chord: string;
  owner: string;
  ownerTitle: string;
  desired: WindowManagerShortcutMap;
}

export interface ShortcutRecorderContext {
  announcement: string;
  conflict: ShortcutRecorderConflict | null;
  error: string | null;
  recording: string | null;
  recordingMode: ShortcutRecorderMode | null;
}

export const shortcutRecorderLogic = createStoreLogic({
  context: (): ShortcutRecorderContext => ({
    announcement: "",
    conflict: null,
    error: null,
    recording: null,
    recordingMode: null,
  }),
  on: {
    started(
      context,
      event: { announcement: string; commandId: string; mode: ShortcutRecorderMode }
    ): ShortcutRecorderContext {
      return {
        ...context,
        announcement: event.announcement,
        conflict: null,
        error: null,
        recording: event.commandId,
        recordingMode: event.mode,
      };
    },
    cancelled(context, event: { announcement: string }): ShortcutRecorderContext {
      return {
        ...context,
        announcement: event.announcement,
        recording: null,
        recordingMode: null,
      };
    },
    recordingStopped(context): ShortcutRecorderContext {
      return { ...context, recording: null, recordingMode: null };
    },
    announced(context, event: { announcement: string }): ShortcutRecorderContext {
      return { ...context, announcement: event.announcement };
    },
    conflictSet(
      context,
      event: { announcement: string; conflict: ShortcutRecorderConflict }
    ): ShortcutRecorderContext {
      return { ...context, announcement: event.announcement, conflict: event.conflict };
    },
    errorSet(context, event: { error: string }): ShortcutRecorderContext {
      return { ...context, error: event.error };
    },
    errorCleared(context): ShortcutRecorderContext {
      return { ...context, error: null };
    },
    conflictDismissed(context): ShortcutRecorderContext {
      return { ...context, conflict: null };
    },
  },
});
