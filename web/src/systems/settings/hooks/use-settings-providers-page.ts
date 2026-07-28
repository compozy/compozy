import { useState } from "react";

import { useSettingsPage } from "./use-settings-page";
import {
  SettingsApiError,
  useDeleteSettingsProvider,
  usePutSettingsProvider,
  useSettingsProviders,
  type ProviderDraft,
  type SettingsMutationResult,
  type SettingsProviderEntry,
} from "@/systems/settings";
import {
  applyProviderFilters,
  DEFAULT_PROVIDER_FILTERS,
  type ProviderFilterState,
} from "@/systems/settings/lib/providers-list-filters";
import {
  emptyProviderDraft,
  providerDraftFromEntry,
  providerDraftIsValid,
  providerDraftToRequest,
} from "@/systems/settings/lib/provider-draft";
import {
  deriveProviderStateLabel,
  type ProviderStateLabel,
} from "@/systems/settings/lib/provider-state";

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

export type ProviderInspectorState =
  | { mode: "closed" }
  | { mode: "inspect"; entry: SettingsProviderEntry }
  | {
      mode: "edit";
      entry: SettingsProviderEntry;
      draft: ProviderDraft;
      cameFrom: "inspect" | "external";
    }
  | { mode: "create"; draft: ProviderDraft };

type DeleteState = { mode: "closed" } | { mode: "open"; entry: SettingsProviderEntry };

export type ProviderLastAction =
  | { kind: "saved"; name: string; result: SettingsMutationResult }
  | { kind: "deleted"; name: string; result: SettingsMutationResult; hadFallback: boolean };

type LastAction = ProviderLastAction | null;

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

  const [inspector, setInspector] = useState<ProviderInspectorState>({ mode: "closed" });
  const [deleteTarget, setDeleteTarget] = useState<DeleteState>({ mode: "closed" });
  const [lastAction, setLastAction] = useState<LastAction>(null);
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
    setInspector({ mode: "inspect", entry });
  };

  const openCreate = () => {
    putMutation.reset();
    setInspector({ mode: "create", draft: emptyProviderDraft() });
  };

  const switchToEdit = () => {
    setInspector(current => {
      if (current.mode !== "inspect") return current;
      return {
        mode: "edit",
        entry: current.entry,
        draft: providerDraftFromEntry(current.entry),
        cameFrom: "inspect",
      };
    });
  };

  const cancelEdit = () => {
    putMutation.reset();
    setInspector(current => {
      if (current.mode === "edit" && current.cameFrom === "inspect") {
        return { mode: "inspect", entry: current.entry };
      }
      return { mode: "closed" };
    });
  };

  const closeInspector = () => {
    setInspector({ mode: "closed" });
    putMutation.reset();
  };

  const updateDraft = (updater: (draft: ProviderDraft) => ProviderDraft) => {
    setInspector(current => {
      if (current.mode !== "edit" && current.mode !== "create") return current;
      return { ...current, draft: updater(current.draft) };
    });
  };

  const inspectorIsValid = isProviderInspectorValid(inspector, providers);

  const saveInspector = () => {
    if (inspector.mode !== "edit" && inspector.mode !== "create") return;
    if (!inspectorIsValid) return;
    const name = inspector.draft.name.trim();
    const body = providerDraftToRequest(inspector.draft);
    putMutation.mutate(
      { name, body },
      {
        onSuccess: result => {
          setLastAction({ kind: "saved", name, result });
          setInspector({ mode: "closed" });
        },
      }
    );
  };

  const openDelete = (entry: SettingsProviderEntry) => {
    deleteMutation.reset();
    setDeleteTarget({ mode: "open", entry });
  };

  const closeDelete = () => {
    setDeleteTarget({ mode: "closed" });
    deleteMutation.reset();
  };

  const confirmDelete = () => {
    if (deleteTarget.mode === "closed") return;
    const target = deleteTarget.entry;
    deleteMutation.mutate(target.name, {
      onSuccess: result => {
        setLastAction({
          kind: "deleted",
          name: target.name,
          result,
          hadFallback: Boolean(target.fallback),
        });
        setDeleteTarget({ mode: "closed" });
        setInspector(current =>
          current.mode === "inspect" && current.entry.name === target.name
            ? { mode: "closed" }
            : current
        );
      },
    });
  };

  const dismissLastAction = () => setLastAction(null);

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
    inspector,
    inspectorIsValid,
    inspectorError: errorMessage(putMutation.error),
    inspectorWarnings: putMutation.data?.warnings,
    inspectorIsSaving: putMutation.isPending,
    openInspect,
    openCreate,
    switchToEdit,
    cancelEdit,
    closeInspector,
    updateDraft,
    saveInspector,
    deleteTarget,
    deleteError: errorMessage(deleteMutation.error),
    deleteIsPending: deleteMutation.isPending,
    openDelete,
    closeDelete,
    confirmDelete,
    lastAction,
    dismissLastAction,
  };
}
