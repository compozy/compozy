import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useStore } from "@xstate/store-react";

import type { ListingViewMode } from "@compozy/ui";
import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { normalizeListingSearchValue } from "@/lib/listing-search";

import {
  AutomationApiError,
  automationListLoopFilter,
  automationRouteHasActiveFilters,
  automationsStartView,
  type AutomationDoes,
  type AutomationScope,
  type AutomationSource,
  type AutomationsRouteSearch,
} from "@/systems/automation";

import { automationCreateSeedLogic } from "./automation-create-seed-store";
import { type SettingsAutomationSection, useSettingsAutomation } from "@/systems/settings";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

/** One-shot editor deep link: `?create=1&start=…` or `?create=loop&start=…&loop=…`. */
export interface AutomationCreateSeed {
  /** Start preselection; `event`/`webhook` open the event editor, otherwise schedules. */
  start?: "schedule" | "event" | "webhook";
  /** When set, the editor opens with Does = Start a Loop for this Loop. */
  loop?: string;
}

/** The seed a route search carries, or null when it opens no editor. */
export function automationCreateSeedOf(
  search: AutomationsRouteSearch
): AutomationCreateSeed | null {
  if (search.create === "loop")
    return search.loop ? { start: search.start, loop: search.loop } : null;
  return search.create === "1" ? { start: search.start } : null;
}

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
 * Consumes the one-shot create deep link. Waits for the active workspace before
 * opening the editor (drafts bind workspace scope), then strips the consumed
 * params so a cancel or reload does not re-open the dialog and the list is not
 * silently filtered by `loop` or `start`.
 */
export function useAutomationCreateSeed(
  seed: AutomationCreateSeed | null,
  activeWorkspaceId: string | null | undefined,
  open: (seed: AutomationCreateSeed) => void
): void {
  const navigate = useNavigate();
  const store = useStore(automationCreateSeedLogic);
  const key = seed ? `${seed.start ?? ""}:${seed.loop ?? ""}` : null;
  const start = seed?.start;
  const loop = seed?.loop;
  useEffect(() => {
    store.trigger.seedObserved({
      activeWorkspaceId,
      key,
      consume: () => {
        open({ start, loop });
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
  }, [activeWorkspaceId, key, loop, navigate, open, start, store]);
}

/**
 * URL-driven listing state for `/automations`: Start view, search, facets and
 * display mode, plus the active workspace and the runtime health gate. The
 * list filters never carry `start`; a Start view only decides which list loads.
 */
export function useAutomationPageBase(search: AutomationsRouteSearch = {}) {
  const navigate = useNavigate();
  const { activeWorkspace, activeWorkspaceId, workspaces } = useActiveWorkspace();
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
