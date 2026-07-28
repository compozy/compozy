import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import {
  deleteWindowManagerLayoutProfile,
  putWindowManagerLayoutProfile,
} from "../adapters/window-manager-layouts-api";
import { settingsKeys } from "../lib/query-keys";
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

interface ProfileDraft {
  aspect: WindowManagerLayoutAspect;
  displayName: string;
  id: string;
  overflow: WindowManagerLayoutOverflow;
  scope: WindowManagerLayoutScopeKind;
}

interface WindowManagerLayoutProfileStoreContext extends ProfileDraft {
  error: Error | null;
  operation: number;
  pendingDelete: WindowManagerLayoutResourceRecord | null;
  pendingLoad: WindowManagerLayoutResourceRecord | null;
  phase: WindowManagerLayoutProfileEditorPhase;
  revision: number;
  selected: WindowManagerLayoutResourceRecord | null;
}

interface SaveProfileExecutionInput {
  document: WindowManagerLayoutDocument;
  draft: ProfileDraft;
  expectedVersion: number;
  workspaceId: string;
}

type WindowManagerLayoutProfileEvents = {
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
  draftChanged: { patch: Partial<ProfileDraft> };
  loadCancelled: {};
  loadConfirmed: {};
  loadRequested: { draftDirty: boolean; record: WindowManagerLayoutResourceRecord };
  profileSelected: { record: WindowManagerLayoutResourceRecord };
  saveFailed: { error: Error; operation: number; revision: number };
  saveRequested: {
    document: WindowManagerLayoutDocument;
    execute: (input: SaveProfileExecutionInput) => Promise<WindowManagerLayoutResourceRecord>;
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

type WindowManagerLayoutProfileEmitted = {
  profileDeleted: { record: WindowManagerLayoutResourceRecord };
  profileLoadAccepted: { record: WindowManagerLayoutResourceRecord };
  profileSaved: {
    previous: WindowManagerLayoutResourceRecord | null;
    record: WindowManagerLayoutResourceRecord;
  };
};

const emptyProfileDraft: ProfileDraft = {
  aspect: "any",
  displayName: "",
  id: "",
  overflow: "stack",
  scope: "workspace",
};

function contextFromRecord(record: WindowManagerLayoutResourceRecord): ProfileDraft {
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
  WindowManagerLayoutProfileStoreContext,
  WindowManagerLayoutProfileEvents,
  WindowManagerLayoutProfileEmitted
>({
  context: (): WindowManagerLayoutProfileStoreContext => ({
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
    draftChanged: (context, event: { patch: Partial<ProfileDraft> }) => ({
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
      const draft: ProfileDraft = {
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

export function useWindowManagerLayoutProfiles({
  workspaceId,
  document,
  profiles,
  draftDirty,
  onLoad,
}: {
  workspaceId: string;
  document: WindowManagerLayoutDocument;
  profiles: readonly WindowManagerLayoutResourceRecord[];
  draftDirty: boolean;
  onLoad: (document: WindowManagerLayoutDocument) => void;
}) {
  const store = useStore(windowManagerLayoutProfileEditorLogic);
  const context = useSelector(store, snapshot => snapshot.context);
  const queryClient = useQueryClient();
  const selectedKey = context.selected ? layoutProfileResourceKey(context.selected) : null;

  useEffect(() => {
    const deleted = store.on("profileDeleted", event => {
      queryClient.setQueryData<WindowManagerLayoutResourceRecord[]>(
        settingsKeys.windowManagerLayoutProfiles(workspaceId),
        current =>
          (current ?? []).filter(
            item => layoutProfileResourceKey(item) !== layoutProfileResourceKey(event.record)
          )
      );
    });
    const loaded = store.on("profileLoadAccepted", event => {
      onLoad({ ...structuredClone(event.record.spec.document), workspaceId });
    });
    const saved = store.on("profileSaved", event => {
      const previousKey = event.previous ? layoutProfileResourceKey(event.previous) : null;
      queryClient.setQueryData<WindowManagerLayoutResourceRecord[]>(
        settingsKeys.windowManagerLayoutProfiles(workspaceId),
        current => [
          ...(current ?? []).filter(item => {
            const key = layoutProfileResourceKey(item);
            return key !== layoutProfileResourceKey(event.record) && key !== previousKey;
          }),
          event.record,
        ]
      );
    });
    return () => {
      deleted.unsubscribe();
      loaded.unsubscribe();
      saved.unsubscribe();
    };
  }, [onLoad, queryClient, store, workspaceId]);

  const saveProfile = () =>
    store.trigger.saveRequested({
      document,
      workspaceId,
      execute: input =>
        putWindowManagerLayoutProfile(
          {
            version: 1,
            id: input.draft.id.trim(),
            displayName: input.draft.displayName.trim(),
            aspectVariant: input.draft.aspect,
            participantSlots: Object.keys(input.document.windows),
            overflowPolicy: input.draft.overflow,
            document: input.document,
          },
          input.draft.scope,
          input.workspaceId,
          input.expectedVersion
        ),
    });
  const selectProfile = (record: WindowManagerLayoutResourceRecord) =>
    store.trigger.profileSelected({ record });
  const requestLoad = (record: WindowManagerLayoutResourceRecord) =>
    store.trigger.loadRequested({ draftDirty, record });
  const confirmLoad = () => store.trigger.loadConfirmed();
  const requestDelete = () => {
    if (context.selected) store.trigger.deleteRequested({ record: context.selected });
  };
  const confirmDelete = () =>
    store.trigger.deleteConfirmed({
      execute: record => deleteWindowManagerLayoutProfile(workspaceId, record.id, record.version),
    });

  return {
    aspect: context.aspect,
    cancelDelete: () => store.trigger.deleteCancelled(),
    cancelLoad: () => store.trigger.loadCancelled(),
    confirmDelete,
    confirmLoad,
    displayName: context.displayName,
    error: context.error,
    id: context.id,
    overflow: context.overflow,
    pendingDelete: context.pendingDelete,
    pendingLoad: context.pendingLoad,
    phase: context.phase,
    profiles,
    requestDelete,
    requestLoad,
    saveProfile,
    scope: context.scope,
    selectProfile,
    selected: context.selected,
    selectedKey,
    setAspect: (aspect: WindowManagerLayoutAspect) =>
      store.trigger.draftChanged({ patch: { aspect } }),
    setDisplayName: (displayName: string) => store.trigger.draftChanged({ patch: { displayName } }),
    setId: (id: string) => store.trigger.draftChanged({ patch: { id } }),
    setOverflow: (overflow: WindowManagerLayoutOverflow) =>
      store.trigger.draftChanged({ patch: { overflow } }),
    setScope: (scope: WindowManagerLayoutScopeKind) =>
      store.trigger.draftChanged({ patch: { scope } }),
    startNew: () => store.trigger.startNew(),
  };
}

export type WindowManagerLayoutProfilesModel = ReturnType<typeof useWindowManagerLayoutProfiles>;
