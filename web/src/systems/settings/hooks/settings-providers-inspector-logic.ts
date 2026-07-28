import { createStoreLogic } from "@xstate/store";

import type {
  ProviderDraft,
  SettingsMutationResult,
  SettingsProviderEntry,
} from "@/systems/settings";

export type ProviderInspectorState =
  | { mode: "closed" }
  | { mode: "inspect"; entry: SettingsProviderEntry }
  | {
      cameFrom: "inspect" | "external";
      draft: ProviderDraft;
      entry: SettingsProviderEntry;
      mode: "edit";
    }
  | { draft: ProviderDraft; mode: "create" };

export type ProviderLastAction =
  | { kind: "saved"; name: string; result: SettingsMutationResult }
  | { hadFallback: boolean; kind: "deleted"; name: string; result: SettingsMutationResult };

export type ProviderInspectorFlow = {
  deleteTarget: SettingsProviderEntry | null;
  draftRevision: number;
  inspector: ProviderInspectorState;
  lastAction: ProviderLastAction | null;
  nextAttempt: number;
  pendingDeleteAttempt: number | null;
  pendingSave: { attempt: number; revision: number } | null;
  saveError: string | null;
};

/** Per-page provider CRUD workflow with event-scoped mutation executors. */
export const settingsProvidersInspectorLogic = createStoreLogic({
  context: (): ProviderInspectorFlow => ({
    deleteTarget: null,
    draftRevision: 0,
    inspector: { mode: "closed" },
    lastAction: null,
    nextAttempt: 0,
    pendingDeleteAttempt: null,
    pendingSave: null,
    saveError: null,
  }),
  on: {
    createOpened: (context, event: { draft: ProviderDraft }) => {
      if (context.pendingDeleteAttempt !== null || context.pendingSave !== null) return;
      return {
        deleteTarget: null,
        draftRevision: context.draftRevision + 1,
        inspector: { draft: event.draft, mode: "create" },
        lastAction: null,
        nextAttempt: context.nextAttempt,
        pendingDeleteAttempt: null,
        pendingSave: null,
        saveError: null,
      };
    },
    deleteCancelled: context =>
      context.pendingDeleteAttempt === null ? { ...context, deleteTarget: null } : undefined,
    deleteOpened: (context, event: { entry: SettingsProviderEntry }) =>
      context.pendingDeleteAttempt === null && context.pendingSave === null
        ? { ...context, deleteTarget: event.entry }
        : undefined,
    deleteRequested: (
      context,
      event: { execute: (name: string) => Promise<SettingsMutationResult> },
      enqueue
    ) => {
      if (!context.deleteTarget || context.pendingDeleteAttempt !== null) return;
      const attempt = context.nextAttempt + 1;
      const name = context.deleteTarget.name;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.deleteSucceeded({ attempt, result: await event.execute(name) });
        } catch {
          trigger.deleteFailed({ attempt });
        }
      });
      return { ...context, nextAttempt: attempt, pendingDeleteAttempt: attempt };
    },
    deleteFailed: (context, event: { attempt: number }) => {
      if (context.pendingDeleteAttempt !== event.attempt) return;
      return { ...context, pendingDeleteAttempt: null };
    },
    deleteSucceeded: (context, event: { attempt: number; result: SettingsMutationResult }) => {
      if (!context.deleteTarget || context.pendingDeleteAttempt !== event.attempt) return;
      const target = context.deleteTarget;
      const inspector =
        context.inspector.mode === "inspect" && context.inspector.entry.name === target.name
          ? { mode: "closed" as const }
          : context.inspector;
      return {
        ...context,
        deleteTarget: null,
        inspector,
        lastAction: {
          hadFallback: Boolean(target.fallback),
          kind: "deleted",
          name: target.name,
          result: event.result,
        },
        pendingDeleteAttempt: null,
      };
    },
    draftChanged: (context, event: { draft: ProviderDraft }) => {
      if (context.inspector.mode !== "edit" && context.inspector.mode !== "create") return;
      return {
        ...context,
        draftRevision: context.draftRevision + 1,
        inspector: { ...context.inspector, draft: event.draft },
        saveError: null,
      };
    },
    editCancelled: context => {
      if (context.inspector.mode !== "edit" || context.pendingSave !== null) return;
      return {
        ...context,
        inspector:
          context.inspector.cameFrom === "inspect"
            ? { entry: context.inspector.entry, mode: "inspect" }
            : { mode: "closed" },
        draftRevision: context.draftRevision + 1,
        pendingSave: null,
        saveError: null,
      };
    },
    editorDismissed: context =>
      context.pendingSave === null
        ? {
            ...context,
            draftRevision: context.draftRevision + 1,
            inspector: { mode: "closed" },
            saveError: null,
          }
        : undefined,
    inspectOpened: (context, event: { entry: SettingsProviderEntry }) =>
      context.pendingDeleteAttempt === null && context.pendingSave === null
        ? {
            ...context,
            draftRevision: context.draftRevision + 1,
            inspector: { entry: event.entry, mode: "inspect" },
            saveError: null,
          }
        : undefined,
    lastActionDismissed: context => ({ ...context, lastAction: null }),
    saveRequested: (
      context,
      event: {
        execute: () => Promise<SettingsMutationResult>;
        name: string;
      },
      enqueue
    ) => {
      if (
        (context.inspector.mode !== "edit" && context.inspector.mode !== "create") ||
        context.pendingSave !== null
      ) {
        return;
      }
      const attempt = context.nextAttempt + 1;
      const revision = context.draftRevision;
      enqueue.effect(async ({ trigger }) => {
        try {
          trigger.saveSucceeded({
            attempt,
            name: event.name,
            revision,
            result: await event.execute(),
          });
        } catch (error) {
          trigger.saveFailed({ attempt, error: saveErrorMessage(error), revision });
        }
      });
      return {
        ...context,
        nextAttempt: attempt,
        pendingSave: { attempt, revision },
        saveError: null,
      };
    },
    saveFailed: (context, event: { attempt: number; error: string; revision: number }) => {
      if (
        context.pendingSave?.attempt !== event.attempt ||
        context.pendingSave.revision !== event.revision
      ) {
        return;
      }
      return { ...context, pendingSave: null, saveError: event.error };
    },
    saveSucceeded: (
      context,
      event: { attempt: number; name: string; result: SettingsMutationResult; revision: number }
    ) => {
      if (
        context.pendingSave?.attempt !== event.attempt ||
        context.pendingSave.revision !== event.revision
      ) {
        return;
      }
      if (context.draftRevision !== event.revision) {
        return {
          ...context,
          lastAction: { kind: "saved", name: event.name, result: event.result },
          pendingSave: null,
        };
      }
      return {
        ...context,
        inspector: { mode: "closed" },
        lastAction: { kind: "saved", name: event.name, result: event.result },
        pendingSave: null,
        saveError: null,
      };
    },
    switchToEdit: (context, event: { draft: ProviderDraft }) => {
      if (context.inspector.mode !== "inspect") return;
      return {
        ...context,
        draftRevision: context.draftRevision + 1,
        inspector: {
          cameFrom: "inspect",
          draft: event.draft,
          entry: context.inspector.entry,
          mode: "edit",
        },
        saveError: null,
      };
    },
  },
});

function saveErrorMessage(error: unknown): string {
  return error instanceof Error && error.message.trim() !== ""
    ? error.message
    : "Failed to save provider.";
}
