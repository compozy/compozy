import { createStoreLogic } from "@xstate/store";

export interface VaultDraft {
  ref: string;
  kind: string;
  secretValue: string;
  /** Confirms that an upsert may rotate the existing normalized ref. */
  overwriteConfirmed: boolean;
}

export type VaultLastAction = { kind: "saved" | "deleted"; ref: string };

export type VaultEditorState = { mode: "closed" } | { draft: VaultDraft; mode: "create" };

export type VaultPageFlow = {
  deleteTargetRef: string | null;
  editor: VaultEditorState;
  lastAction: VaultLastAction | null;
  nextAttempt: number;
  pendingDeleteAttempt: number | null;
  pendingPutAttempt: number | null;
  replaceValue: string;
  selectedRef: string | null;
};

type VaultPageEvents = {
  createOpened: { draft: VaultDraft };
  deleteCancelled: {};
  deleteFailed: { attempt: number };
  deleteOpened: { ref: string };
  deleteRequested: { execute: (ref: string) => Promise<unknown> };
  deleteSucceeded: { attempt: number };
  draftChanged: { draft: VaultDraft };
  editorDismissed: {};
  inspectClosed: {};
  inspectOpened: { ref: string };
  lastActionDismissed: {};
  putFailed: { attempt: number };
  putRequested: { execute: () => Promise<{ ref: string }> };
  putSucceeded: { attempt: number; ref: string };
  replaceValueChanged: { value: string };
};

/** Per-vault-page CRUD workflow. It fences stale mutation completions by attempt. */
export const vaultPageLogic = createStoreLogic<VaultPageFlow, VaultPageEvents>({
  context: (): VaultPageFlow => ({
    deleteTargetRef: null,
    editor: { mode: "closed" },
    lastAction: null,
    nextAttempt: 0,
    pendingDeleteAttempt: null,
    pendingPutAttempt: null,
    replaceValue: "",
    selectedRef: null,
  }),
  on: {
    createOpened: (context, event: { draft: VaultDraft }) =>
      context.pendingDeleteAttempt === null && context.pendingPutAttempt === null
        ? {
            ...context,
            deleteTargetRef: null,
            editor: { draft: event.draft, mode: "create" },
            pendingPutAttempt: null,
            replaceValue: "",
            selectedRef: null,
          }
        : undefined,
    deleteCancelled: context =>
      context.pendingDeleteAttempt === null ? { ...context, deleteTargetRef: null } : undefined,
    deleteFailed: (context, event: { attempt: number }) =>
      context.pendingDeleteAttempt === event.attempt
        ? { ...context, pendingDeleteAttempt: null }
        : undefined,
    deleteOpened: (context, event) =>
      context.pendingPutAttempt === null
        ? {
            ...context,
            deleteTargetRef: event.ref,
            pendingDeleteAttempt: null,
          }
        : undefined,
    deleteRequested: (context, event, enqueue) => {
      if (
        !context.deleteTargetRef ||
        context.pendingDeleteAttempt !== null ||
        context.pendingPutAttempt !== null
      ) {
        return;
      }
      const attempt = context.nextAttempt + 1;
      const ref = context.deleteTargetRef;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute(ref);
          trigger.deleteSucceeded({ attempt });
        } catch {
          trigger.deleteFailed({ attempt });
        }
      });
      return { ...context, nextAttempt: attempt, pendingDeleteAttempt: attempt };
    },
    deleteSucceeded: (context, event: { attempt: number }) => {
      if (!context.deleteTargetRef || context.pendingDeleteAttempt !== event.attempt) return;
      const ref = context.deleteTargetRef;
      return {
        ...context,
        deleteTargetRef: null,
        lastAction: { kind: "deleted", ref },
        pendingDeleteAttempt: null,
        replaceValue: context.selectedRef === ref ? "" : context.replaceValue,
        selectedRef: context.selectedRef === ref ? null : context.selectedRef,
      };
    },
    editorDismissed: context =>
      context.pendingPutAttempt === null ? { ...context, editor: { mode: "closed" } } : undefined,
    inspectClosed: context =>
      context.pendingPutAttempt === null
        ? { ...context, replaceValue: "", selectedRef: null }
        : undefined,
    inspectOpened: (context, event: { ref: string }) =>
      context.pendingDeleteAttempt === null && context.pendingPutAttempt === null
        ? {
            ...context,
            deleteTargetRef: null,
            editor: { mode: "closed" },
            pendingPutAttempt: null,
            replaceValue: "",
            selectedRef: event.ref,
          }
        : undefined,
    lastActionDismissed: context => ({ ...context, lastAction: null }),
    putFailed: (context, event: { attempt: number }) =>
      context.pendingPutAttempt === event.attempt
        ? { ...context, pendingPutAttempt: null }
        : undefined,
    putRequested: (context, event, enqueue) => {
      if (
        (context.editor.mode !== "create" && context.selectedRef === null) ||
        context.pendingPutAttempt !== null ||
        context.pendingDeleteAttempt !== null
      ) {
        return;
      }
      const attempt = context.nextAttempt + 1;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.putSucceeded({ attempt, ref: (await event.execute()).ref });
        } catch {
          trigger.putFailed({ attempt });
        }
      });
      return { ...context, nextAttempt: attempt, pendingPutAttempt: attempt };
    },
    replaceValueChanged: (context, event: { value: string }) => ({
      ...context,
      replaceValue: event.value,
    }),
    putSucceeded: (context, event: { attempt: number; ref: string }) => {
      if (context.pendingPutAttempt !== event.attempt) return;
      return {
        ...context,
        editor: { mode: "closed" },
        lastAction: { kind: "saved", ref: event.ref },
        pendingPutAttempt: null,
        replaceValue: "",
      };
    },
    draftChanged: (context, event: { draft: VaultDraft }) =>
      context.editor.mode === "create"
        ? {
            ...context,
            editor: {
              draft:
                event.draft.ref.trim() === context.editor.draft.ref.trim()
                  ? event.draft
                  : { ...event.draft, overwriteConfirmed: false },
              mode: "create",
            },
          }
        : undefined,
  },
});
