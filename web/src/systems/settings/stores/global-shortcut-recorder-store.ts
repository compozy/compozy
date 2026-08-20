import { createStoreLogic } from "@xstate/store";

import type { WindowManagerGlobalShortcutMap } from "@/systems/os";

export interface GlobalShortcutConflict {
  commandId: string;
  chord: string;
  owner: string;
  desired: WindowManagerGlobalShortcutMap;
}

export interface GlobalShortcutRecorderContext {
  announcement: string;
  conflict: GlobalShortcutConflict | null;
  error: string | null;
  recording: string | null;
}

export const globalShortcutRecorderLogic = createStoreLogic({
  context: (): GlobalShortcutRecorderContext => ({
    announcement: "",
    conflict: null,
    error: null,
    recording: null,
  }),
  on: {
    started(
      context,
      event: { announcement: string; commandId: string }
    ): GlobalShortcutRecorderContext {
      return {
        ...context,
        announcement: event.announcement,
        conflict: null,
        error: null,
        recording: event.commandId,
      };
    },
    cancelled(context, event: { announcement: string }): GlobalShortcutRecorderContext {
      return { ...context, announcement: event.announcement, recording: null };
    },
    recordingStopped(context): GlobalShortcutRecorderContext {
      return { ...context, recording: null };
    },
    announced(context, event: { announcement: string }): GlobalShortcutRecorderContext {
      return { ...context, announcement: event.announcement };
    },
    conflictSet(
      context,
      event: { announcement: string; conflict: GlobalShortcutConflict }
    ): GlobalShortcutRecorderContext {
      return { ...context, announcement: event.announcement, conflict: event.conflict };
    },
    errorSet(context, event: { error: string }): GlobalShortcutRecorderContext {
      return { ...context, error: event.error };
    },
    errorCleared(context): GlobalShortcutRecorderContext {
      return { ...context, error: null };
    },
    conflictDismissed(context): GlobalShortcutRecorderContext {
      return { ...context, conflict: null };
    },
  },
});
