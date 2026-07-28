import { createStoreLogic } from "@xstate/store";

import { layoutProfileResourceKey } from "../lib/window-manager-layout-profile-key";
import type {
  WindowManagerLayoutAspect,
  WindowManagerLayoutDocument,
  WindowManagerLayoutOverflow,
  WindowManagerLayoutResourceRecord,
  WindowManagerLayoutScopeKind,
} from "../lib/window-manager-layout-types";

export type WindowManagerLayoutProfileEditorPhase =
  | "baseline"
  | "draft"
  | "dirty"
  | "saving"
  | "conflict"
  | "error";

export interface WindowManagerLayoutProfileDraft {
  aspect: WindowManagerLayoutAspect;
  displayName: string;
  id: string;
  overflow: WindowManagerLayoutOverflow;
  scope: WindowManagerLayoutScopeKind;
}

export interface WindowManagerLayoutProfileEditorState extends WindowManagerLayoutProfileDraft {
  error: Error | null;
  operation: number;
  pendingDelete: WindowManagerLayoutResourceRecord | null;
  pendingLoad: WindowManagerLayoutResourceRecord | null;
  phase: WindowManagerLayoutProfileEditorPhase;
  revision: number;
  selected: WindowManagerLayoutResourceRecord | null;
}

export interface WindowManagerLayoutProfileSaveInput {
  document: WindowManagerLayoutDocument;
  draft: WindowManagerLayoutProfileDraft;
  expectedVersion: number;
  workspaceId: string;
}

type WindowManagerLayoutProfileEditorEvents = {
  deleteCancelled: {};
  deleteConfirmed: {
    execute: (record: WindowManagerLayoutResourceRecord) => Promise<unknown>;
  };
  deleteFailed: { error: Error; operation: number; revision: number };
  deleteRequested: { record: WindowManagerLayoutResourceRecord };
  deleteSucceeded: {
    operation: number;
    record: WindowManagerLayoutResourceRecord;
    revision: number;
  };
  draftChanged: { patch: Partial<WindowManagerLayoutProfileDraft> };
  loadCancelled: {};
  loadConfirmed: {};
  loadRequested: { draftDirty: boolean; record: WindowManagerLayoutResourceRecord };
  profileSelected: { record: WindowManagerLayoutResourceRecord };
  saveFailed: { error: Error; operation: number; revision: number };
  saveRequested: {
    document: WindowManagerLayoutDocument;
    execute: (
      input: WindowManagerLayoutProfileSaveInput
    ) => Promise<WindowManagerLayoutResourceRecord>;
    workspaceId: string;
  };
  saveSucceeded: {
    operation: number;
    previous: WindowManagerLayoutResourceRecord | null;
    record: WindowManagerLayoutResourceRecord;
    revision: number;
  };
  startNew: {};
};

type WindowManagerLayoutProfileEditorEmitted = {
  profileDeleted: { record: WindowManagerLayoutResourceRecord };
  profileLoadAccepted: { record: WindowManagerLayoutResourceRecord };
  profileSaved: {
    previous: WindowManagerLayoutResourceRecord | null;
    record: WindowManagerLayoutResourceRecord;
  };
};

const emptyProfileDraft: WindowManagerLayoutProfileDraft = {
  aspect: "any",
  displayName: "",
  id: "",
  overflow: "stack",
  scope: "workspace",
};

function contextFromRecord(
  record: WindowManagerLayoutResourceRecord
): WindowManagerLayoutProfileDraft {
  return {
    aspect: record.spec.aspectVariant,
    displayName: record.spec.displayName,
    id: record.id,
    overflow: record.spec.overflowPolicy,
    scope: record.scope.kind,
  };
}

function isConflictError(error: Error): boolean {
  return Reflect.get(error, "status") === 409;
}

export const windowManagerLayoutProfileEditorLogic = createStoreLogic<
  WindowManagerLayoutProfileEditorState,
  WindowManagerLayoutProfileEditorEvents,
  WindowManagerLayoutProfileEditorEmitted
>({
  context: (): WindowManagerLayoutProfileEditorState => ({
    ...emptyProfileDraft,
    error: null,
    operation: 0,
    pendingDelete: null,
    pendingLoad: null,
    phase: "baseline",
    revision: 0,
    selected: null,
  }),
  on: {
    deleteCancelled: context =>
      context.phase === "saving" ? undefined : { ...context, pendingDelete: null },
    deleteConfirmed: (context, event, enqueue) => {
      if (!context.pendingDelete || context.phase === "saving") return;
      const operation = context.operation + 1;
      const record = context.pendingDelete;
      const revision = context.revision;
      enqueue.effect(async ({ trigger }) => {
        try {
          await event.execute(record);
          trigger.deleteSucceeded({ operation, record, revision });
        } catch (cause) {
          trigger.deleteFailed({
            error:
              cause instanceof Error ? cause : new Error("Unable to delete the layout profile."),
            operation,
            revision,
          });
        }
      });
      return {
        ...context,
        error: null,
        operation,
        phase: "saving",
      };
    },
    deleteFailed: (context, event: { error: Error; operation: number; revision: number }) => {
      if (context.operation !== event.operation) return;
      return {
        ...context,
        error: event.error,
        phase: isConflictError(event.error) ? "conflict" : "error",
      };
    },
    deleteRequested: (context, event: { record: WindowManagerLayoutResourceRecord }) => ({
      ...context,
      pendingDelete: event.record,
    }),
    deleteSucceeded: (context, event, enqueue) => {
      if (context.operation !== event.operation) return;
      enqueue.emit.profileDeleted({ record: event.record });
      if (context.revision !== event.revision) {
        const deletedSelected =
          context.selected !== null &&
          layoutProfileResourceKey(context.selected) === layoutProfileResourceKey(event.record);
        return {
          ...context,
          pendingDelete: null,
          selected: deletedSelected ? null : context.selected,
        };
      }
      return {
        ...emptyProfileDraft,
        error: null,
        operation: context.operation,
        pendingDelete: null,
        pendingLoad: null,
        phase: "baseline",
        revision: context.revision + 1,
        selected: null,
      };
    },
    draftChanged: (context, event: { patch: Partial<WindowManagerLayoutProfileDraft> }) => ({
      ...context,
      ...event.patch,
      revision: context.revision + 1,
      error: null,
      phase: "dirty",
    }),
    loadCancelled: context => ({ ...context, pendingLoad: null }),
    loadConfirmed: (context, _event, enqueue) => {
      const record = context.pendingLoad;
      if (!record) return;
      enqueue.emit.profileLoadAccepted({ record });
      return {
        ...context,
        ...contextFromRecord(record),
        pendingLoad: null,
        phase: "draft",
        revision: context.revision + 1,
        selected: record,
      };
    },
    loadRequested: (context, event, enqueue) => {
      if (event.draftDirty) return { ...context, pendingLoad: event.record };
      enqueue.emit.profileLoadAccepted({ record: event.record });
      return {
        ...context,
        ...contextFromRecord(event.record),
        pendingLoad: null,
        phase: "draft",
        revision: context.revision + 1,
        selected: event.record,
      };
    },
    profileSelected: (context, event: { record: WindowManagerLayoutResourceRecord }) => ({
      ...context,
      ...contextFromRecord(event.record),
      error: null,
      phase: "draft",
      revision: context.revision + 1,
      selected: event.record,
    }),
    saveFailed: (context, event: { error: Error; operation: number; revision: number }) => {
      if (context.operation !== event.operation) return;
      return {
        ...context,
        error: event.error,
        phase: isConflictError(event.error) ? "conflict" : "error",
      };
    },
    saveRequested: (context, event, enqueue) => {
      if (
        context.phase === "saving" ||
        context.id.trim() === "" ||
        context.displayName.trim() === ""
      ) {
        return;
      }
      const operation = context.operation + 1;
      const previous = context.selected;
      const revision = context.revision;
      const draft: WindowManagerLayoutProfileDraft = {
        aspect: context.aspect,
        displayName: context.displayName,
        id: context.id,
        overflow: context.overflow,
        scope: context.scope,
      };
      const expectedVersion =
        previous !== null && previous.id === context.id.trim() ? previous.version : 0;
      enqueue.effect(async ({ trigger }) => {
        try {
          const record = await event.execute({
            document: event.document,
            draft,
            expectedVersion,
            workspaceId: event.workspaceId,
          });
          trigger.saveSucceeded({ operation, previous, record, revision });
        } catch (cause) {
          trigger.saveFailed({
            error: cause instanceof Error ? cause : new Error("Unable to save the layout profile."),
            operation,
            revision,
          });
        }
      });
      return { ...context, error: null, operation, phase: "saving" };
    },
    saveSucceeded: (context, event, enqueue) => {
      if (context.operation !== event.operation) return;
      enqueue.emit.profileSaved({ previous: event.previous, record: event.record });
      if (context.revision !== event.revision) {
        return { ...context, error: null, selected: event.record };
      }
      return {
        ...context,
        ...contextFromRecord(event.record),
        error: null,
        phase: "draft",
        revision: context.revision + 1,
        selected: event.record,
      };
    },
    startNew: context => ({
      ...context,
      ...emptyProfileDraft,
      error: null,
      pendingDelete: null,
      pendingLoad: null,
      phase: "draft",
      revision: context.revision + 1,
      selected: null,
    }),
  },
});
