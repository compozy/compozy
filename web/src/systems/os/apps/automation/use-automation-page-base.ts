import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useStore } from "@xstate/store-react";

import type { ListingViewMode } from "@compozy/ui";
import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { normalizeListingSearchValue } from "@/lib/listing-search";
import { notifyUser } from "@/lib/user-feedback";

import {
  AutomationApiError,
  automationListLoopFilter,
  automationRouteHasActiveFilters,
  automationsStartView,
  type AutomationDoes,
  type AutomationEditorSeed,
  type AutomationScope,
  type AutomationSource,
  type AutomationsRouteSearch,
} from "@/systems/automation";

import { automationCreateSeedLogic } from "./automation-create-seed-store";
import { type SettingsAutomationSection, useSettingsAutomation } from "@/systems/settings";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

const UNAVAILABLE_OFF =
  "Automations are turned off. Turn on automations in Settings, then restart CompozyOS.";
const UNAVAILABLE_TRANSIENT =
  "CompozyOS couldn't load your automations right now. Try again in a moment.";

/** Runtime-off or daemon 503: the one reason automations can't be read or operated. */
export function automationUnavailableMessage(
  runtime: SettingsAutomationSection["runtime"] | null,
  ...errors: (Error | null | undefined)[]
): string | null {
  if (runtime && !runtime.available) return UNAVAILABLE_OFF;
  if (errors.some(error => error instanceof AutomationApiError && error.status === 503)) {
    return UNAVAILABLE_TRANSIENT;
  }
  return null;
}

/**
 * Consumes the one-shot `?create=1|loop` deep link. Waits until the project or
 * Global lens is resolved (the draft binds its location). A Loop seed needs a
 * project, since the Loop lives in one: in Global it says so instead of
 * opening. Either way it strips the consumed params
 * so a cancel or reload does not re-open the dialog and the list is not
 * silently filtered by `loop` or `start`.
 */
export function useAutomationCreateSeed(
  seed: AutomationEditorSeed | null,
  workspace: { activeWorkspaceId: string | null | undefined; resolved: boolean },
  openCreate: (seed: AutomationEditorSeed) => void
): void {
  const navigate = useNavigate();
  const store = useStore(automationCreateSeedLogic);
  useEffect(() => {
    store.trigger.seedObserved({
      activeWorkspaceId: workspace.activeWorkspaceId,
      seed,
      workspaceResolved: workspace.resolved,
      consume: (consumed, outcome) => {
        if (outcome === "open") openCreate(consumed);
        else notifyUser({ message: "Pick a project to automate a Loop.", tone: "info" });
        void navigate({
          replace: true,
          search: current => ({
            ...(current as AutomationsRouteSearch),
            create: undefined,
            loop: undefined,
            start: undefined,
          }),
          to: "/automations",
        });
      },
    });
  }, [navigate, openCreate, seed, store, workspace.activeWorkspaceId, workspace.resolved]);
}

/**
 * URL-driven listing state for `/automations`: Start view, search, facets and
 * display mode, plus the active workspace and the runtime health gate. The
 * list filters never carry `start`; a Start view only decides which list loads.
 */
export function useAutomationPageBase(search: AutomationsRouteSearch = {}) {
  const navigate = useNavigate();
  const { activeWorkspace, activeWorkspaceId, pending, workspaces } = useActiveWorkspace();
  const settingsQuery = useSettingsAutomation();

  const start = automationsStartView(search) ?? null;
  const view: ListingViewMode = search.view ?? "rows";

  const updateSearch = (updates: Partial<AutomationsRouteSearch>) => {
    void navigate({
      search: current => ({ ...(current as AutomationsRouteSearch), ...updates }),
      to: "/automations",
    });
  };

  const searchInput = useDebouncedInput({
    externalValue: search.q ?? "",
    onCommit: q => updateSearch({ q: normalizeListingSearchValue(q) }),
  });
  const committedSearchQuery = normalizeListingSearchValue(searchInput.committedValue);

  const listFilters = {
    limit: 50,
    enabled: search.enabled,
    loop: automationListLoopFilter(search),
    q: committedSearchQuery,
    scope: search.scope,
    source: search.source,
    target: search.target,
    workspace_id: search.scope === "workspace" ? (activeWorkspaceId ?? undefined) : undefined,
  };

  const clearFilters = () => {
    searchInput.reset("");
    updateSearch({
      enabled: undefined,
      loop: search.create === "loop" ? search.loop : undefined,
      q: undefined,
      scope: undefined,
      source: undefined,
      start: undefined,
      target: undefined,
    });
  };

  return {
    activeWorkspace,
    activeWorkspaceId,
    /** The project or Global lens is known (no longer loading). */
    workspaceResolved: !pending,
    automationRuntime: settingsQuery.data?.runtime ?? null,
    clearFilters,
    enabledFilter: search.enabled ?? null,
    hasActiveFilters: automationRouteHasActiveFilters({ ...search, q: searchInput.draftValue }),
    listFilters,
    loopFilter: automationListLoopFilter(search) ?? null,
    scopeFilter: search.scope ?? null,
    searchQuery: searchInput.draftValue,
    setEnabledFilter: (enabled: boolean | null) => updateSearch({ enabled: enabled ?? undefined }),
    setLoopFilter: (loop: string | null) => updateSearch({ loop: loop ?? undefined }),
    setScopeFilter: (scope: AutomationScope | null) => updateSearch({ scope: scope ?? undefined }),
    setSearchQuery: searchInput.setDraftValue,
    setSourceFilter: (source: AutomationSource | null) =>
      updateSearch({ source: source ?? undefined }),
    setStart: (next: "schedule" | "event" | null) => updateSearch({ start: next ?? undefined }),
    setTargetFilter: (target: AutomationDoes | null) =>
      updateSearch({ target: target ?? undefined }),
    setView: (nextView: ListingViewMode) =>
      updateSearch({ view: nextView === "cards" ? "cards" : undefined }),
    sourceFilter: search.source ?? null,
    start,
    targetFilter: search.target ?? null,
    view,
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  };
}
