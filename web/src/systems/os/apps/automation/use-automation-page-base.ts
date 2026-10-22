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
  type AutomationSource,
  type AutomationsRouteSearch,
} from "@/systems/automation";

import { automationCreateSeedLogic } from "./automation-create-seed-store";
import { type SettingsAutomationSection, useSettingsAutomation } from "@/systems/settings";
import {
  automationBreadthParams,
  breadthQuery,
  toWorkspaceCommandSelectOptions,
  useActiveWorkspace,
  useProjectFilter,
  useWindowDataWorkspace,
} from "@/systems/workspace";

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

/** Page-scoped seed state; created before the editor so a save can settle it. */
export function useAutomationCreateSeedStore() {
  return useStore(automationCreateSeedLogic);
}

/**
 * Opens the editor for a `?create=1|loop` deep link once the project or User
 * lens is resolved (the draft binds its location). The params stay while the
 * editor is open — a late or repeated mount opens it again — and leave the
 * URL when the operator closes it. A Loop seed needs a project, since the
 * Loop lives in one: without a project it says so and strips the params.
 */
export function useAutomationCreateSeed(
  store: ReturnType<typeof useAutomationCreateSeedStore>,
  seed: AutomationEditorSeed | null,
  context: {
    activeWorkspaceId: string | null | undefined;
    editorOpen: boolean;
    resolved: boolean;
  },
  openCreate: (seed: AutomationEditorSeed) => void
): () => void {
  const navigate = useNavigate();
  const strip = () =>
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
  useEffect(() => {
    store.trigger.seedObserved({
      activeWorkspaceId: context.activeWorkspaceId,
      editorOpen: context.editorOpen,
      seed,
      workspaceResolved: context.resolved,
      open: openCreate,
      refuse: () => notifyUser({ message: "Pick a project to automate a Loop.", tone: "info" }),
      strip,
    });
  });
  return () => store.trigger.operatorClosed({ strip });
}

/**
 * URL-driven listing state for `/automations`: Start view, search, facets and
 * display mode, plus the active workspace and the runtime health gate. The
 * list filters never carry `start`; a Start view only decides which list loads.
 * The window's data scope decides breadth: a project lists only its own
 * automations (8A), User only those without a project, and All projects every
 * one, narrowed by the transient Project filter.
 */
export function useAutomationPageBase(search: AutomationsRouteSearch = {}) {
  const navigate = useNavigate();
  const { activeWorkspace, activeWorkspaceId, pending, workspaces } = useActiveWorkspace();
  const dataScope = useWindowDataWorkspace();
  const projectFilter = useProjectFilter(dataScope, workspaces);
  const breadth = dataScope
    ? automationBreadthParams(breadthQuery(dataScope, projectFilter.values))
    : null;
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
    source: search.source,
    target: search.target,
    ...breadth,
  };

  const clearFilters = () => {
    searchInput.reset("");
    projectFilter.setValues([]);
    updateSearch({
      enabled: undefined,
      loop: search.create === "loop" ? search.loop : undefined,
      q: undefined,
      source: undefined,
      start: undefined,
      target: undefined,
    });
  };

  return {
    activeWorkspace,
    activeWorkspaceId,
    /** What this window lists; `null` while the destination is unsettled (lists stay idle). */
    dataScope,
    /** The project, User, or All projects lens is known (no longer loading). */
    workspaceResolved: !pending,
    automationRuntime: settingsQuery.data?.runtime ?? null,
    /** The global automation time zone every listing sentence reads in. */
    timeZone: settingsQuery.data?.config?.timezone?.trim() || undefined,
    clearFilters,
    enabledFilter: search.enabled ?? null,
    hasActiveFilters:
      automationRouteHasActiveFilters({ ...search, q: searchInput.draftValue }) ||
      projectFilter.values.length > 0,
    listFilters,
    listsEnabled: breadth !== null,
    loopFilter: automationListLoopFilter(search) ?? null,
    projectFilter: projectFilter.values,
    /** The shared Project filter field; only under All projects. */
    projectFilterField: projectFilter.field,
    searchQuery: searchInput.draftValue,
    setEnabledFilter: (enabled: boolean | null) => updateSearch({ enabled: enabled ?? undefined }),
    setLoopFilter: (loop: string | null) => updateSearch({ loop: loop ?? undefined }),
    setProjectFilter: projectFilter.setValues,
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
