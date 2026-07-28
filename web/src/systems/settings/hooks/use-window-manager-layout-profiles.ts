import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

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
  /** Loading over unapplied edits discards them, so it asks first. */
  draftDirty: boolean;
  onLoad: (document: WindowManagerLayoutDocument) => void;
}) {
  const queryClient = useQueryClient();
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const selected =
    profiles.find(profile => layoutProfileResourceKey(profile) === selectedKey) ?? null;
  const [id, setId] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [aspect, setAspect] = useState<WindowManagerLayoutAspect>("any");
  const [overflow, setOverflow] = useState<WindowManagerLayoutOverflow>("stack");
  const [scope, setScope] = useState<WindowManagerLayoutScopeKind>("workspace");
  const [pendingLoad, setPendingLoad] = useState<WindowManagerLayoutResourceRecord | null>(null);
  const [deleteRequested, setDeleteRequested] = useState(false);

  const save = useMutation({
    mutationFn: () => {
      const normalizedId = id.trim();
      return putWindowManagerLayoutProfile(
        {
          version: 1,
          id: normalizedId,
          displayName: displayName.trim(),
          aspectVariant: aspect,
          participantSlots: Object.keys(document.windows),
          overflowPolicy: overflow,
          document,
        },
        scope,
        workspaceId,
        selected !== null && selected.id === normalizedId ? selected.version : 0
      );
    },
    onSuccess: record => {
      const previousKey = selected === null ? null : layoutProfileResourceKey(selected);
      setSelectedKey(layoutProfileResourceKey(record));
      queryClient.setQueryData<WindowManagerLayoutResourceRecord[]>(
        settingsKeys.windowManagerLayoutProfiles(workspaceId),
        current => [
          ...(current ?? []).filter(item => {
            const key = layoutProfileResourceKey(item);
            return key !== layoutProfileResourceKey(record) && key !== previousKey;
          }),
          record,
        ]
      );
    },
  });

  const remove = useMutation({
    mutationFn: () => {
      if (selected === null) throw new Error("Select a saved profile first.");
      return deleteWindowManagerLayoutProfile(workspaceId, selected.id, selected.version);
    },
    onSuccess: () => {
      if (selected === null) return;
      queryClient.setQueryData<WindowManagerLayoutResourceRecord[]>(
        settingsKeys.windowManagerLayoutProfiles(workspaceId),
        current =>
          (current ?? []).filter(
            item => layoutProfileResourceKey(item) !== layoutProfileResourceKey(selected)
          )
      );
      // The whole form belonged to the deleted record — clearing only the name
      // and id left its scope, screen shape and overflow behind as defaults for
      // the next profile.
      startNew();
      setDeleteRequested(false);
    },
  });

  /** Fills the editor form from a record without touching the layout draft. */
  const selectProfile = (record: WindowManagerLayoutResourceRecord) => {
    setSelectedKey(layoutProfileResourceKey(record));
    setId(record.id);
    setDisplayName(record.spec.displayName);
    setAspect(record.spec.aspectVariant);
    setOverflow(record.spec.overflowPolicy);
    setScope(record.scope.kind);
  };

  const loadIntoDraft = (record: WindowManagerLayoutResourceRecord) => {
    selectProfile(record);
    onLoad({ ...structuredClone(record.spec.document), workspaceId });
  };

  const requestLoad = (record: WindowManagerLayoutResourceRecord) => {
    if (draftDirty) {
      setPendingLoad(record);
      return;
    }
    loadIntoDraft(record);
  };

  const confirmLoad = () => {
    if (pendingLoad === null) return;
    loadIntoDraft(pendingLoad);
    setPendingLoad(null);
  };

  const cancelLoad = () => setPendingLoad(null);

  const requestDelete = () => {
    if (selected === null) return;
    setDeleteRequested(true);
  };

  const cancelDelete = () => setDeleteRequested(false);

  const confirmDelete = () => {
    if (selected === null) return;
    remove.mutate();
  };

  function startNew() {
    setSelectedKey(null);
    setId("");
    setDisplayName("");
    setAspect("any");
    setOverflow("stack");
    setScope("workspace");
  }

  return {
    aspect,
    cancelDelete,
    cancelLoad,
    confirmDelete,
    confirmLoad,
    displayName,
    error: save.error ?? remove.error,
    id,
    overflow,
    pendingDelete: deleteRequested && selected !== null ? selected : null,
    pendingLoad,
    profiles,
    remove,
    requestDelete,
    requestLoad,
    save,
    scope,
    selectProfile,
    selected,
    selectedKey,
    setAspect,
    setDisplayName,
    setId,
    setOverflow,
    setScope,
    startNew,
  };
}

export type WindowManagerLayoutProfilesModel = ReturnType<typeof useWindowManagerLayoutProfiles>;
