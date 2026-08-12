import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";

import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue } from "@/lib/listing-search";

import {
  parseSandboxBackendFilter,
  parseSandboxPersistenceFilter,
  type SandboxBackendFilter,
  type SandboxPersistenceFilter,
} from "../lib/sandbox-list-filters";
import type { SandboxRouteSearch } from "../lib/sandbox-route-search";
import {
  emptySandboxDraft,
  sandboxEnvErrors,
  sandboxSecretEnvErrors,
  toSandboxDraft,
  toSandboxRequest,
  type SandboxDraft,
} from "../lib/sandbox-profile-draft";
import { sandboxPageLogic } from "./sandbox-page-store";
import {
  SettingsApiError,
  type SettingsSandboxEntry,
  useDeleteSettingsSandbox,
  usePutSettingsSandbox,
  useSettingsPage,
  useSettingsSandboxes,
} from "@/systems/settings";

export type { SandboxEditorState, SandboxLastAction } from "./sandbox-page-store";

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

function matchesNormalizedSandboxQuery(
  entry: SettingsSandboxEntry,
  normalizedQuery: string
): boolean {
  return (
    normalizedQuery.length === 0 ||
    [
      entry.name,
      entry.profile.backend,
      entry.profile.sync_mode ?? "",
      entry.profile.persistence ?? "",
      entry.profile.runtime_root ?? "",
      entry.source_metadata.effective_source.kind,
    ]
      .join(" ")
      .toLowerCase()
      .includes(normalizedQuery)
  );
}

function filterSandboxes(
  sandboxes: SettingsSandboxEntry[],
  query: string,
  backend: SandboxBackendFilter,
  persistence: SandboxPersistenceFilter
): SettingsSandboxEntry[] {
  const normalizedQuery = query.toLowerCase();
  return sandboxes.filter(
    entry =>
      matchesNormalizedSandboxQuery(entry, normalizedQuery) &&
      (backend === "all" || entry.profile.backend === backend) &&
      (persistence === "all" || (entry.profile.persistence ?? "") === persistence)
  );
}

export function useSandboxPage(search: SandboxRouteSearch = {}) {
  const navigate = useNavigate({ from: "/sandbox" });
  const queryText = search.q ?? "";
  const backend: SandboxBackendFilter = parseSandboxBackendFilter(search.backend) ?? "all";
  const persistence: SandboxPersistenceFilter =
    parseSandboxPersistenceFilter(search.persistence) ?? "all";
  const view: ListingViewMode = search.view ?? "rows";

  const query = useSettingsSandboxes();
  const putMutation = usePutSettingsSandbox();
  const deleteMutation = useDeleteSettingsSandbox();
  const page = useSettingsPage({ currentSlug: "sandboxes" });
  const store = useStore(sandboxPageLogic);
  const flow = useSelector(store, snapshot => snapshot.context);

  const envelope = query.data ?? null;
  const sandboxes = envelope?.sandboxes ?? [];
  const filtered = filterSandboxes(sandboxes, queryText, backend, persistence);
  const selectedEntry = flow.selectedName
    ? (sandboxes.find(entry => entry.name === flow.selectedName) ?? null)
    : null;
  const counts = {
    total: sandboxes.length,
    totalWorkspaces: sandboxes.reduce((total, entry) => total + entry.workspace_usage_count, 0),
  };
  const hasActiveFilters =
    queryText.trim().length > 0 || backend !== "all" || persistence !== "all";

  const updateSearch = (updater: (current: SandboxRouteSearch) => SandboxRouteSearch) => {
    void navigate({
      search: current => updater((current as SandboxRouteSearch | undefined) ?? {}),
      to: "/sandbox",
    });
  };
  const openCreate = () => {
    putMutation.reset();
    store.trigger.editorOpened({ editor: { mode: "create", draft: emptySandboxDraft() } });
  };
  const openEdit = (entry: SettingsSandboxEntry) => {
    putMutation.reset();
    store.trigger.editorOpened({
      editor: { mode: "edit", name: entry.name, draft: toSandboxDraft(entry), entry },
    });
  };
  const closeEditor = () => {
    if (flow.pendingRequest) return;
    store.trigger.editorClosed();
    putMutation.reset();
  };
  const updateDraft = (updater: (draft: SandboxDraft) => SandboxDraft) => {
    if (flow.editor.mode !== "closed") {
      store.trigger.draftChanged({ draft: updater(flow.editor.draft) });
    }
  };
  const editorName = flow.editor.mode === "closed" ? "" : flow.editor.draft.name.trim();
  const editorIsValid =
    flow.editor.mode !== "closed" &&
    editorName.length > 0 &&
    flow.editor.draft.backend.trim().length > 0 &&
    (flow.editor.mode !== "create" ||
      !sandboxes.some(entry => entry.name.toLowerCase() === editorName.toLowerCase())) &&
    Object.keys(sandboxEnvErrors(flow.editor.draft)).length === 0 &&
    Object.keys(sandboxSecretEnvErrors(flow.editor.draft)).length === 0;
  const saveEditor = () => {
    if (flow.editor.mode === "closed" || !editorIsValid) return;
    const name = flow.editor.draft.name.trim();
    const body = toSandboxRequest(flow.editor.draft);
    store.trigger.saveRequested({
      execute: () => putMutation.mutateAsync({ name, body }),
      name,
    });
  };
  const openDelete = (entry: SettingsSandboxEntry) => {
    if (flow.pendingRequest) return;
    deleteMutation.reset();
    store.trigger.deleteOpened({ entry });
  };
  const closeDelete = () => {
    if (flow.pendingRequest) return;
    store.trigger.deleteClosed();
    deleteMutation.reset();
  };
  const confirmDelete = () => {
    if (flow.deleteTarget.mode === "closed") return;
    store.trigger.deleteRequested({ execute: name => deleteMutation.mutateAsync(name) });
  };

  return {
    isLoading: query.isLoading,
    isRefetching: query.isFetching && !query.isLoading,
    queryError: errorMessage(query.error),
    error: query.error,
    envelope,
    sandboxes,
    filtered,
    counts,
    query: queryText,
    backend,
    persistence,
    view,
    hasActiveFilters,
    setQuery: (next: string) =>
      updateSearch(current => ({ ...current, q: normalizeListingSearchValue(next) })),
    setBackend: (next: SandboxBackendFilter) =>
      updateSearch(current => ({ ...current, backend: next === "all" ? undefined : next })),
    setPersistence: (next: SandboxPersistenceFilter) =>
      updateSearch(current => ({
        ...current,
        persistence: next === "all" ? undefined : next,
      })),
    setView: (next: ListingViewMode) =>
      updateSearch(current => ({ ...current, view: next === "rows" ? undefined : next })),
    clearFilters: () => updateSearch(current => ({ view: current.view })),
    selectedEntry,
    openInspect: (entry: SettingsSandboxEntry) =>
      store.trigger.profileInspectOpened({ name: entry.name }),
    closeInspect: () => store.trigger.profileInspectClosed(),
    refetch: () => query.refetch(),
    restart: page.restart,
    editor: flow.editor,
    editorIsValid,
    editorError: errorMessage(putMutation.error),
    editorWarnings: putMutation.data?.warnings,
    editorIsSaving: flow.pendingRequest?.kind === "save",
    openCreate,
    openEdit,
    closeEditor,
    updateDraft,
    saveEditor,
    deleteTarget: flow.deleteTarget,
    deleteError: errorMessage(deleteMutation.error),
    deleteIsPending: flow.pendingRequest?.kind === "delete",
    openDelete,
    closeDelete,
    confirmDelete,
    lastAction: flow.lastAction,
    dismissLastAction: () => store.trigger.lastActionDismissed(),
  };
}

export { parseSandboxBackendFilter, parseSandboxPersistenceFilter };
