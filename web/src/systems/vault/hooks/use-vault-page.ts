import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useRef } from "react";

import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import { VaultApiError } from "../adapters/vault-api";
import {
  normalizeVaultPrefixForNamespace,
  type VaultNamespaceFilter,
  type VaultRouteSearch,
} from "../lib/vault-route-search";
import { useDeleteVaultSecret, usePutVaultSecret } from "./use-vault-actions";
import { useVaultSecrets } from "./use-vault";
import {
  VAULT_NAMESPACES,
  type VaultListFilter,
  type VaultNamespace,
  type VaultSecret,
} from "../types";
import { vaultPageLogic, type VaultDraft } from "./vault-page-logic";

export type { VaultDraft, VaultEditorState, VaultLastAction } from "./vault-page-logic";

export type VaultDeleteState = { mode: "closed" } | { mode: "open"; secret: VaultSecret };

function emptyDraft(): VaultDraft {
  return {
    ref: "vault:sessions/",
    kind: "",
    secretValue: "",
    overwriteConfirmed: false,
  };
}

/** Mirrors `vault.NormalizeRef` — the daemon compares refs after trimming only. */
export function normalizeVaultRef(ref: string): string {
  return ref.trim();
}

function errorMessage(error: unknown): string | null {
  if (error instanceof VaultApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

function filterFor(namespace: VaultNamespaceFilter, prefix: string): VaultListFilter {
  const filter: VaultListFilter = {};
  if (namespace !== "all") filter.namespace = namespace;
  const normalizedPrefix = prefix.trim();
  if (normalizedPrefix) filter.prefix = normalizedPrefix;
  return filter;
}

function countVaultSecrets(secrets: VaultSecret[]) {
  const byNamespace = Object.fromEntries(VAULT_NAMESPACES.map(item => [item, 0])) as Record<
    VaultNamespace,
    number
  >;
  for (const secret of secrets) {
    if (secret.namespace in byNamespace) byNamespace[secret.namespace as VaultNamespace] += 1;
  }
  return {
    total: secrets.length,
    sessions: byNamespace.sessions,
    providers: byNamespace.providers,
    byNamespace,
  };
}

export function useVaultPage(search: VaultRouteSearch = {}) {
  const navigate = useNavigate({ from: "/vault" });
  const prefix = search.q ?? "";
  const namespace: VaultNamespaceFilter = search.namespace ?? "all";
  const routeRef = search.ref?.trim() || null;
  const view: ListingViewMode = search.view ?? "rows";
  const pageStore = useStore(vaultPageLogic);
  const flow = useSelector(pageStore, snapshot => snapshot.context);

  const filter = filterFor(namespace, prefix);
  const query = useVaultSecrets(filter);
  // Collision detection and identity resolution must see the whole vault, not
  // only the active namespace/prefix projection. Keep that unbounded read cold
  // until an action actually needs cross-filter truth.
  const inventoryRequired =
    routeRef !== null ||
    flow.editor.mode === "create" ||
    flow.selectedRef !== null ||
    flow.deleteTargetRef !== null;
  const allSecretsQuery = useVaultSecrets({}, { enabled: inventoryRequired });
  const putMutation = usePutVaultSecret();
  const deleteMutation = useDeleteVaultSecret();

  const secrets = query.data ?? [];
  const counts = countVaultSecrets(secrets);
  const secretInventory = allSecretsQuery.data ?? secrets;
  const selectedSecret = flow.selectedRef
    ? (secretInventory.find(secret => secret.ref === flow.selectedRef) ?? null)
    : null;
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
  const deleteTargetSecret = flow.deleteTargetRef
    ? (secretInventory.find(secret => secret.ref === flow.deleteTargetRef) ?? null)
    : null;
  const collisionInventoryReady = allSecretsQuery.isSuccess && !allSecretsQuery.isStale;
  const editorRef = flow.editor.mode === "create" ? normalizeVaultRef(flow.editor.draft.ref) : "";
  const editorRefExists =
    editorRef !== "" &&
    (allSecretsQuery.data ?? []).some(secret => normalizeVaultRef(secret.ref) === editorRef);
  const overwriteBlocked =
    flow.editor.mode === "create" &&
    (!collisionInventoryReady || (editorRefExists && !flow.editor.draft.overwriteConfirmed));
  const editorIsValid =
    flow.editor.mode === "create" &&
    flow.editor.draft.ref.trim().startsWith("vault:") &&
    flow.editor.draft.secretValue.trim() !== "" &&
    !overwriteBlocked;

  const updateSearch = (updater: (current: VaultRouteSearch) => VaultRouteSearch) => {
    void navigate({
      search: current => updater((current as VaultRouteSearch | undefined) ?? {}),
      to: "/vault",
    });
  };

  const openCreate = () => {
    putMutation.reset();
    pageStore.trigger.createOpened({ draft: emptyDraft() });
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
    const ref = normalizeVaultRef(flow.editor.draft.ref);
    const kind = flow.editor.draft.kind.trim();
    const request = {
      ref,
      secret_value: flow.editor.draft.secretValue,
      ...(kind ? { kind } : {}),
    };
    pageStore.trigger.putRequested({ execute: () => putMutation.mutateAsync(request) });
  };
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
  const replaceIsValid = Boolean(
    selectedSecret &&
    flow.replaceValue.trim() !== "" &&
    deleteTargetSecret === null &&
    flow.pendingDeleteAttempt === null &&
    flow.pendingPutAttempt === null
  );
  const replaceSecret = () => {
    if (!selectedSecret || !replaceIsValid) return;
    const kind = selectedSecret.kind?.trim();
    const request = {
      ref: selectedSecret.ref,
      secret_value: flow.replaceValue,
      ...(kind ? { kind } : {}),
    };
    pageStore.trigger.putRequested({ execute: () => putMutation.mutateAsync(request) });
  };
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

  const putError = errorMessage(putMutation.error);
  const selectionError =
    routeRef !== null && !allSecretsQuery.isFetching
      ? allSecretsQuery.isError
        ? (errorMessage(allSecretsQuery.error) ?? "Vault secret unavailable")
        : allSecretsQuery.isSuccess && selectedSecret === null
          ? "Vault secret not found"
          : null
      : null;
  return {
    counts,
    deleteError: errorMessage(deleteMutation.error),
    deleteIsPending: flow.pendingDeleteAttempt !== null,
    deleteTarget: deleteTargetSecret
      ? { mode: "open" as const, secret: deleteTargetSecret }
      : { mode: "closed" as const },
    dismissLastAction: () => pageStore.trigger.lastActionDismissed(),
    editor: flow.editor,
    editorError:
      flow.editor.mode === "create" ? (putError ?? errorMessage(allSecretsQuery.error)) : null,
    editorIsSaving: flow.editor.mode === "create" && flow.pendingPutAttempt !== null,
    editorIsValid,
    editorRefExists,
    filter,
    isLoading: query.isLoading,
    isRefetching: query.isFetching && !query.isLoading,
    lastAction: flow.lastAction,
    namespace,
    prefix,
    queryError: errorMessage(query.error),
    refetch: query.refetch,
    replaceError: selectedSecret ? putError : null,
    replaceIsPending:
      Boolean(selectedSecret) && flow.pendingPutAttempt !== null && flow.editor.mode === "closed",
    replaceIsValid,
    replaceSecret,
    replaceValue: flow.replaceValue,
    selectionError,
    secrets,
    selectedSecret,
    setNamespace: (next: VaultNamespaceFilter) =>
      updateSearch(current => ({
        ...current,
        q: normalizeVaultPrefixForNamespace(current.q, next === "all" ? undefined : next),
        namespace: next === "all" ? undefined : next,
      })),
    setPrefix: (next: string) =>
      updateSearch(current => ({ ...current, q: normalizeListingSearchValue(next) })),
    setReplaceValue: (value: string) => pageStore.trigger.replaceValueChanged({ value }),
    setView: (next: ListingViewMode) =>
      updateSearch(current => ({ ...current, view: next === "rows" ? undefined : next })),
    closeDelete,
    closeEditor,
    closeInspect,
    confirmDelete,
    openCreate,
    openDelete,
    openInspect,
    saveEditor,
    updateDraft,
    view,
  };
}
