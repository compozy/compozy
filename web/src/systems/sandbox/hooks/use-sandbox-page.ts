import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@agh/ui";

import { useSettingsPage } from "@/systems/settings/hooks/use-settings-page";
import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import {
  parseSandboxBackendFilter,
  parseSandboxPersistenceFilter,
  type SandboxBackendFilter,
  type SandboxPersistenceFilter,
} from "../lib/sandbox-list-filters";
import {
  emptySandboxDraft,
  sandboxEnvErrors,
  toSandboxDraft,
  toSandboxRequest,
  type SandboxDraft,
} from "../lib/sandbox-profile-draft";
import {
  SettingsApiError,
  useDeleteSettingsSandbox,
  usePutSettingsSandbox,
  useSettingsSandboxes,
  type SettingsSandboxEntry,
  type SettingsMutationResult,
} from "@/systems/settings";

export interface SandboxRouteSearch {
  q?: string;
  backend?: string;
  persistence?: string;
  view?: ListingViewMode;
}

export function validateSandboxSearch(search: Record<string, unknown>): SandboxRouteSearch {
  return {
    q: normalizeListingSearchValue(search.q),
    backend: parseSandboxBackendFilter(search.backend),
    persistence: parseSandboxPersistenceFilter(search.persistence),
    view: parseListingView(search.view),
  };
}

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

function matchesQuery(entry: SettingsSandboxEntry, query: string): boolean {
  if (!query) return true;
  const haystack = [
    entry.name,
    entry.profile.backend,
    entry.profile.sync_mode ?? "",
    entry.profile.persistence ?? "",
    entry.profile.runtime_root ?? "",
    entry.source_metadata.effective_source.kind,
  ]
    .join(" ")
    .toLowerCase();
  return haystack.includes(query.toLowerCase());
}

function filterSandboxes(
  sandboxes: SettingsSandboxEntry[],
  query: string,
  backend: SandboxBackendFilter,
  persistence: SandboxPersistenceFilter
): SettingsSandboxEntry[] {
  return sandboxes.filter(entry => {
    if (!matchesQuery(entry, query)) return false;
    if (backend !== "all" && entry.profile.backend !== backend) return false;
    if (persistence !== "all" && (entry.profile.persistence ?? "") !== persistence) return false;
    return true;
  });
}

export type SandboxEditorState =
  | { mode: "closed" }
  | { mode: "create"; draft: SandboxDraft }
  | {
      mode: "edit";
      name: string;
      draft: SandboxDraft;
      entry: SettingsSandboxEntry;
    };

type DeleteState = { mode: "closed" } | { mode: "open"; entry: SettingsSandboxEntry };

export type SandboxLastAction =
  | { kind: "saved"; name: string; result: SettingsMutationResult }
  | {
      kind: "deleted";
      name: string;
      result: SettingsMutationResult;
      usageCount: number;
    };

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

  const [editor, setEditor] = useState<SandboxEditorState>({ mode: "closed" });
  const [deleteTarget, setDeleteTarget] = useState<DeleteState>({ mode: "closed" });
  const [lastAction, setLastAction] = useState<SandboxLastAction | null>(null);
  const [selectedName, setSelectedName] = useState<string | null>(null);

  const envelope = query.data ?? null;
  const sandboxes = envelope?.sandboxes ?? [];
  const filtered = filterSandboxes(sandboxes, queryText, backend, persistence);
  const selectedEntry = selectedName
    ? (sandboxes.find(entry => entry.name === selectedName) ?? null)
    : null;

  const counts = {
    total: sandboxes.length,
    totalWorkspaces: sandboxes.reduce((acc, entry) => acc + entry.workspace_usage_count, 0),
  };

  const hasActiveFilters =
    queryText.trim().length > 0 || backend !== "all" || persistence !== "all";

  const updateSearch = (updater: (current: SandboxRouteSearch) => SandboxRouteSearch) => {
    void navigate({
      search: current => updater((current as SandboxRouteSearch | undefined) ?? {}),
      to: "/sandbox",
    });
  };

  const setQuery = (next: string) => {
    updateSearch(current => ({
      ...current,
      q: normalizeListingSearchValue(next),
    }));
  };

  const setBackend = (next: SandboxBackendFilter) => {
    updateSearch(current => ({
      ...current,
      backend: next === "all" ? undefined : next,
    }));
  };

  const setPersistence = (next: SandboxPersistenceFilter) => {
    updateSearch(current => ({
      ...current,
      persistence: next === "all" ? undefined : next,
    }));
  };

  const setView = (nextView: ListingViewMode) => {
    updateSearch(current => ({
      ...current,
      view: nextView === "rows" ? undefined : nextView,
    }));
  };

  const clearFilters = () => {
    updateSearch(current => ({ view: current.view }));
  };

  const openInspect = (entry: SettingsSandboxEntry) => {
    setSelectedName(entry.name);
  };

  const closeInspect = () => {
    setSelectedName(null);
  };

  const openCreate = () => {
    putMutation.reset();
    setSelectedName(null);
    setEditor({ mode: "create", draft: emptySandboxDraft() });
  };

  const openEdit = (entry: SettingsSandboxEntry) => {
    putMutation.reset();
    setSelectedName(null);
    setEditor({ mode: "edit", name: entry.name, draft: toSandboxDraft(entry), entry });
  };

  const closeEditor = () => {
    setEditor({ mode: "closed" });
    putMutation.reset();
  };

  const updateDraft = (updater: (draft: SandboxDraft) => SandboxDraft) => {
    setEditor(current => {
      if (current.mode === "closed") return current;
      return { ...current, draft: updater(current.draft) };
    });
  };

  const editorName = editor.mode === "closed" ? "" : editor.draft.name.trim();
  const editorIsValid =
    editor.mode !== "closed" &&
    editorName.length > 0 &&
    editor.draft.backend.trim().length > 0 &&
    (editor.mode !== "create" ||
      !sandboxes.some(entry => entry.name.toLowerCase() === editorName.toLowerCase())) &&
    Object.keys(sandboxEnvErrors(editor.draft)).length === 0;

  const saveEditor = () => {
    if (editor.mode === "closed") return;
    const name = editor.draft.name.trim();
    if (!name) return;
    if (Object.keys(sandboxEnvErrors(editor.draft)).length > 0) return;
    const body = toSandboxRequest(editor.draft);
    putMutation.mutate(
      { name, body },
      {
        onSuccess: result => {
          setLastAction({ kind: "saved", name, result });
          setEditor({ mode: "closed" });
        },
      }
    );
  };

  const openDelete = (entry: SettingsSandboxEntry) => {
    deleteMutation.reset();
    setSelectedName(null);
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
          usageCount: target.workspace_usage_count,
        });
        setDeleteTarget({ mode: "closed" });
      },
    });
  };

  const dismissLastAction = () => setLastAction(null);

  const refetch = () => query.refetch();

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
    setQuery,
    setBackend,
    setPersistence,
    setView,
    clearFilters,
    selectedEntry,
    openInspect,
    closeInspect,
    refetch,
    restart: page.restart,
    editor,
    editorIsValid,
    editorError: errorMessage(putMutation.error),
    editorWarnings: putMutation.data?.warnings,
    editorIsSaving: putMutation.isPending,
    openCreate,
    openEdit,
    closeEditor,
    updateDraft,
    saveEditor,
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

export { parseSandboxBackendFilter, parseSandboxPersistenceFilter };
