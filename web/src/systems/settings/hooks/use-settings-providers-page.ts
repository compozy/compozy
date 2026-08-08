import { useState } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { SettingsApiError } from "../adapters/settings-api";
import type { ProviderDraft, SettingsProviderEntry } from "../types";
import {
  applyProviderFilters,
  DEFAULT_PROVIDER_FILTERS,
  type ProviderFilterState,
} from "../lib/providers-list-filters";
import {
  emptyProviderDraft,
  providerDraftFromEntry,
  providerDraftIsValid,
  providerDraftToRequest,
} from "../lib/provider-draft";
import { useSettingsProviders } from "./use-settings-collections";
import { useDeleteSettingsProvider, usePutSettingsProvider } from "./use-settings-mutations";
import {
  deriveProviderStateLabel,
  type ProviderStateLabel,
} from "@/systems/settings/lib/provider-state";

import {
  settingsProvidersInspectorLogic,
  type ProviderInspectorState,
} from "./settings-providers-inspector-logic";
import { useSettingsPage } from "./use-settings-page";

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

export type {
  ProviderInspectorState,
  ProviderLastAction,
} from "./settings-providers-inspector-logic";
export type { ProviderDraft };

function isProviderInspectorValid(
  inspector: ProviderInspectorState,
  providers: readonly SettingsProviderEntry[]
): boolean {
  if (inspector.mode !== "edit" && inspector.mode !== "create") return false;
  return providerDraftIsValid(inspector.draft, {
    mode: inspector.mode,
    existingNames: providers.map(provider => provider.name),
  });
}

export function useSettingsProvidersPage() {
  const query = useSettingsProviders();
  const putMutation = usePutSettingsProvider();
  const deleteMutation = useDeleteSettingsProvider();
  const page = useSettingsPage({ currentSlug: "providers" });
  const inspectorStore = useStore(settingsProvidersInspectorLogic);
  const inspectorFlow = useSelector(inspectorStore, snapshot => snapshot.context);
  const [filters, setFilters] = useState<ProviderFilterState>(DEFAULT_PROVIDER_FILTERS);

  const envelope = query.data ?? null;
  const providers = envelope?.providers ?? [];
  const providerStates = providers.map(deriveProviderStateLabel);
  const counts = {
    total: providers.length,
    installed: providerStates.filter(state => state === "installed").length,
    binaryMissing: providerStates.filter(state => state === "binary-missing").length,
    needsSetup: providerStates.filter(state => state !== "installed" && state !== "binary-missing")
      .length,
  };
  const filteredProviders = applyProviderFilters(providers, filters);

  const setStatusFilter = (next: ProviderStateLabel | null) => {
    setFilters(current => ({ ...current, statusFilter: next }));
  };
  const setNameQuery = (next: string) => {
    setFilters(current => ({ ...current, nameQuery: next }));
  };

  const openInspect = (entry: SettingsProviderEntry) => {
    putMutation.reset();
    inspectorStore.trigger.inspectOpened({ entry });
  };
  const openCreate = () => {
    putMutation.reset();
    inspectorStore.trigger.createOpened({ draft: emptyProviderDraft() });
  };
  const switchToEdit = () => {
    const inspector = inspectorFlow.inspector;
    if (inspector.mode === "inspect") {
      inspectorStore.trigger.switchToEdit({ draft: providerDraftFromEntry(inspector.entry) });
    }
  };
  const cancelEdit = () => {
    if (inspectorFlow.pendingSave !== null) return;
    putMutation.reset();
    inspectorStore.trigger.editCancelled();
  };
  const closeInspector = () => {
    if (inspectorFlow.pendingSave !== null) return;
    inspectorStore.trigger.editorDismissed();
    putMutation.reset();
  };
  const updateDraft = (updater: (draft: ProviderDraft) => ProviderDraft) => {
    const inspector = inspectorFlow.inspector;
    if (inspector.mode === "edit" || inspector.mode === "create") {
      inspectorStore.trigger.draftChanged({ draft: updater(inspector.draft) });
    }
  };

  const inspectorIsValid = isProviderInspectorValid(inspectorFlow.inspector, providers);
  const saveInspector = () => {
    const inspector = inspectorFlow.inspector;
    if ((inspector.mode !== "edit" && inspector.mode !== "create") || !inspectorIsValid) return;
    const name = inspector.draft.name.trim();
    const body = providerDraftToRequest(inspector.draft);
    inspectorStore.trigger.saveRequested({
      execute: () => putMutation.mutateAsync({ name, body }),
      name,
    });
  };
  const openDelete = (entry: SettingsProviderEntry) => {
    deleteMutation.reset();
    inspectorStore.trigger.deleteOpened({ entry });
  };
  const closeDelete = () => {
    if (inspectorFlow.pendingDeleteAttempt !== null) return;
    inspectorStore.trigger.deleteCancelled();
    deleteMutation.reset();
  };
  const confirmDelete = () => {
    if (!inspectorFlow.deleteTarget) return;
    inspectorStore.trigger.deleteRequested({ execute: name => deleteMutation.mutateAsync(name) });
  };

  return {
    isLoading: query.isLoading,
    error: query.error,
    envelope,
    providers,
    filteredProviders,
    filters,
    setStatusFilter,
    setNameQuery,
    counts,
    restart: page.restart,
    inspector: inspectorFlow.inspector,
    inspectorIsValid,
    inspectorError: inspectorFlow.saveError,
    inspectorWarnings: putMutation.data?.warnings,
    inspectorIsSaving: inspectorFlow.pendingSave !== null,
    openInspect,
    openCreate,
    switchToEdit,
    cancelEdit,
    closeInspector,
    updateDraft,
    saveInspector,
    deleteTarget: inspectorFlow.deleteTarget
      ? { mode: "open" as const, entry: inspectorFlow.deleteTarget }
      : { mode: "closed" as const },
    deleteError: errorMessage(deleteMutation.error),
    deleteIsPending: inspectorFlow.pendingDeleteAttempt !== null,
    openDelete,
    closeDelete,
    confirmDelete,
    lastAction: inspectorFlow.lastAction,
    dismissLastAction: () => inspectorStore.trigger.lastActionDismissed(),
  };
}
