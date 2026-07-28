import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";
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
import { windowManagerLayoutProfileEditorLogic } from "../stores/window-manager-layout-profile-editor-store";

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
