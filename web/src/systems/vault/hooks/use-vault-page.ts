import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import {
  normalizeVaultPrefixForNamespace,
  type VaultNamespaceFilter,
  type VaultRouteSearch,
} from "../lib/vault-route-search";
import type { VaultSecret } from "../types";
import { useVaultSecrets } from "./use-vault";
import { usePutVaultSecret } from "./use-vault-actions";
import {
  useVaultDelete,
  useVaultEditor,
  useVaultInspect,
  useVaultLastActionToast,
  useVaultPageFlow,
  useVaultRouteSelection,
  type VaultSearchUpdater,
} from "./use-vault-page-actions";
import {
  findVaultSecret,
  vaultErrorMessage,
  vaultFilterFor,
  vaultSelectionError,
} from "./vault-page-model";

export type { VaultDraft, VaultEditorState, VaultLastAction } from "./vault-page-logic";
export { normalizeVaultRef } from "./vault-page-model";

export type VaultDeleteState = { mode: "closed" } | { mode: "open"; secret: VaultSecret };

function vaultSearchSetters(updateSearch: VaultSearchUpdater) {
  return {
    setNamespace: (next: VaultNamespaceFilter) => {
      const namespace = next === "all" ? undefined : next;
      updateSearch(current => ({
        ...current,
        q: normalizeVaultPrefixForNamespace(current.q, namespace),
        namespace,
      }));
    },
    setPrefix: (next: string) =>
      updateSearch(current => ({ ...current, q: normalizeListingSearchValue(next) })),
    setView: (next: ListingViewMode) =>
      updateSearch(current => ({ ...current, view: next === "rows" ? undefined : next })),
  };
}

export function useVaultPage(search: VaultRouteSearch = {}) {
  const navigate = useNavigate({ from: "/vault" });
  const prefix = search.q ?? "";
  const namespace: VaultNamespaceFilter = search.namespace ?? "all";
  const routeRef = search.ref?.trim() || null;
  const view: ListingViewMode = search.view ?? "rows";
  const pageFlow = useVaultPageFlow();
  const { flow } = pageFlow;

  const filter = vaultFilterFor(namespace, prefix);
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

  const secrets = query.data ?? [];
  const secretInventory = allSecretsQuery.data ?? secrets;
  const selectedSecret = findVaultSecret(secretInventory, flow.selectedRef);
  const deleteTargetSecret = findVaultSecret(secretInventory, flow.deleteTargetRef);
  useVaultLastActionToast(flow.lastAction, pageFlow.pageStore);
  useVaultRouteSelection(routeRef, pageFlow);

  const updateSearch: VaultSearchUpdater = updater => {
    void navigate({
      search: current => updater((current as VaultRouteSearch | undefined) ?? {}),
      to: "/vault",
    });
  };

  const editor = useVaultEditor(pageFlow, putMutation, allSecretsQuery);
  const inspect = useVaultInspect(pageFlow, {
    putMutation,
    selectedSecret,
    deleteTargetOpen: deleteTargetSecret !== null,
    updateSearch,
  });
  const deletion = useVaultDelete(pageFlow, deleteTargetSecret, updateSearch);

  return {
    counts: { total: secrets.length },
    ...deletion,
    ...editor,
    ...inspect,
    ...vaultSearchSetters(updateSearch),
    filter,
    isLoading: query.isLoading,
    isRefetching: query.isFetching && !query.isLoading,
    namespace,
    prefix,
    queryError: vaultErrorMessage(query.error),
    refetch: query.refetch,
    selectionError: vaultSelectionError(routeRef, allSecretsQuery, selectedSecret),
    secrets,
    selectedSecret,
    view,
  };
}
