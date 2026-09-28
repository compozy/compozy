import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useRef } from "react";

import { toast } from "@compozy/ui";

import { vaultSecretTitle } from "../lib/vault-secret-title";
import type { VaultRouteSearch } from "../lib/vault-route-search";
import type { VaultSecret } from "../types";
import { useDeleteVaultSecret, usePutVaultSecret } from "./use-vault-actions";
import { type VaultDraft, type VaultLastAction, vaultPageLogic } from "./vault-page-logic";
import {
  checkVaultEditor,
  emptyVaultDraft,
  normalizeVaultRef,
  vaultErrorMessage,
  vaultPutRequest,
} from "./vault-page-model";

type VaultSearchUpdater = (updater: (current: VaultRouteSearch) => VaultRouteSearch) => void;

function useVaultPageFlow() {
  const pageStore = useStore(vaultPageLogic);
  const flow = useSelector(pageStore, snapshot => snapshot.context);
  return { pageStore, flow };
}

type VaultPageFlowHandle = ReturnType<typeof useVaultPageFlow>;
type VaultPutMutation = ReturnType<typeof usePutVaultSecret>;

/** Announces a finished save/delete once, then clears it from the flow. */
function useVaultLastActionToast(
  lastAction: VaultLastAction | null,
  pageStore: VaultPageFlowHandle["pageStore"]
) {
  useEffect(() => {
    if (!lastAction) return;
    const title = vaultSecretTitle(lastAction.ref);
    if (lastAction.kind === "saved") toast.success(`Saved ${title}`);
    else toast(`Deleted ${title}`);
    pageStore.trigger.lastActionDismissed();
  }, [lastAction, pageStore]);
}

/** Opens the inspector for a deep-linked ref and closes it when the ref leaves the URL. */
function useVaultRouteSelection(routeRef: string | null, { pageStore, flow }: VaultPageFlowHandle) {
  const previousRouteRef = useRef<string | null | undefined>(undefined);
  useEffect(() => {
    const previous = previousRouteRef.current;
    previousRouteRef.current = routeRef;
    if (routeRef !== null) {
      if (flow.selectedRef !== routeRef) pageStore.trigger.inspectOpened({ ref: routeRef });
      return;
    }
    if (previous !== undefined && previous !== null && flow.selectedRef !== null) {
      pageStore.trigger.inspectClosed();
    }
  }, [flow.selectedRef, pageStore, routeRef]);
}

interface VaultInventoryQuery {
  data?: VaultSecret[];
  error: unknown;
  isStale: boolean;
  isSuccess: boolean;
}

function useVaultEditor(
  { pageStore, flow }: VaultPageFlowHandle,
  putMutation: VaultPutMutation,
  inventory: VaultInventoryQuery
) {
  const isCreating = flow.editor.mode === "create";
  const { editorRefExists, editorIsValid } = checkVaultEditor(
    flow.editor,
    inventory.data,
    inventory.isSuccess && !inventory.isStale
  );

  const openCreate = () => {
    putMutation.reset();
    pageStore.trigger.createOpened({ draft: emptyVaultDraft() });
  };
  const closeEditor = () => {
    if (flow.pendingPutAttempt !== null) return;
    pageStore.trigger.editorDismissed();
    putMutation.reset();
  };
  const updateDraft = (updater: (draft: VaultDraft) => VaultDraft) => {
    if (flow.editor.mode === "create") {
      pageStore.trigger.draftChanged({ draft: updater(flow.editor.draft) });
    }
  };
  const saveEditor = () => {
    if (flow.editor.mode !== "create" || !editorIsValid) return;
    const { draft } = flow.editor;
    const request = vaultPutRequest(normalizeVaultRef(draft.ref), draft.secretValue, draft.kind);
    pageStore.trigger.putRequested({ execute: () => putMutation.mutateAsync(request) });
  };

  return {
    editor: flow.editor,
    editorError: isCreating
      ? (vaultErrorMessage(putMutation.error) ?? vaultErrorMessage(inventory.error))
      : null,
    editorIsSaving: isCreating && flow.pendingPutAttempt !== null,
    editorIsValid,
    editorRefExists,
    openCreate,
    closeEditor,
    updateDraft,
    saveEditor,
  };
}

interface VaultInspectOptions {
  putMutation: VaultPutMutation;
  selectedSecret: VaultSecret | null;
  deleteTargetOpen: boolean;
  updateSearch: VaultSearchUpdater;
}

/** Inspector open/close plus the in-place value replacement for the selected secret. */
function useVaultInspect(
  { pageStore, flow }: VaultPageFlowHandle,
  { putMutation, selectedSecret, deleteTargetOpen, updateSearch }: VaultInspectOptions
) {
  const idle = flow.pendingDeleteAttempt === null && flow.pendingPutAttempt === null;
  const replaceIsValid =
    selectedSecret !== null && flow.replaceValue.trim() !== "" && !deleteTargetOpen && idle;

  const openInspect = (secret: VaultSecret) => {
    putMutation.reset();
    pageStore.trigger.inspectOpened({ ref: secret.ref });
    updateSearch(current => ({ ...current, ref: secret.ref }));
  };
  const closeInspect = () => {
    if (flow.pendingPutAttempt !== null) return;
    pageStore.trigger.inspectClosed();
    putMutation.reset();
    updateSearch(current => ({ ...current, ref: undefined }));
  };
  const replaceSecret = () => {
    if (!selectedSecret || !replaceIsValid) return;
    const request = vaultPutRequest(selectedSecret.ref, flow.replaceValue, selectedSecret.kind);
    pageStore.trigger.putRequested({ execute: () => putMutation.mutateAsync(request) });
  };

  return {
    openInspect,
    closeInspect,
    replaceError: selectedSecret ? vaultErrorMessage(putMutation.error) : null,
    replaceIsPending:
      selectedSecret !== null && flow.pendingPutAttempt !== null && flow.editor.mode === "closed",
    replaceIsValid,
    replaceSecret,
    replaceValue: flow.replaceValue,
    setReplaceValue: (value: string) => pageStore.trigger.replaceValueChanged({ value }),
  };
}

function useVaultDelete({ pageStore, flow }: VaultPageFlowHandle, target: VaultSecret | null) {
  const deleteMutation = useDeleteVaultSecret();

  const openDelete = (secret: VaultSecret) => {
    if (flow.pendingPutAttempt !== null) return;
    deleteMutation.reset();
    pageStore.trigger.deleteOpened({ ref: secret.ref });
  };
  const closeDelete = () => {
    if (flow.pendingDeleteAttempt !== null) return;
    pageStore.trigger.deleteCancelled();
    deleteMutation.reset();
  };
  const confirmDelete = () => {
    if (!flow.deleteTargetRef) return;
    pageStore.trigger.deleteRequested({ execute: ref => deleteMutation.mutateAsync(ref) });
  };

  return {
    deleteError: vaultErrorMessage(deleteMutation.error),
    deleteIsPending: flow.pendingDeleteAttempt !== null,
    deleteTarget: target ? { mode: "open" as const, secret: target } : { mode: "closed" as const },
    openDelete,
    closeDelete,
    confirmDelete,
  };
}

export {
  useVaultDelete,
  useVaultEditor,
  useVaultInspect,
  useVaultLastActionToast,
  useVaultPageFlow,
  useVaultRouteSelection,
};
export type { VaultSearchUpdater };
