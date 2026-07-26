import { useEffect, useRef } from "react";
import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@agh/ui";
import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import {
  AutomationApiError,
  parseAutomationEnabled,
  parseAutomationScope,
  parseAutomationSource,
} from "@/systems/automation";
import type {
  AutomationScopeFilter,
  AutomationSource,
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
} from "@/systems/automation";
import { useSettingsAutomation } from "@/systems/settings";
import type { SettingsAutomationSection } from "@/systems/settings";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

export type JobEditorState =
  | { draft: CreateAutomationJobRequest; mode: "create" }
  | { draft: CreateAutomationJobRequest; id: string; mode: "edit" };

export type TriggerEditorState =
  | { draft: CreateAutomationTriggerRequest; mode: "create" }
  | { draft: CreateAutomationTriggerRequest; id: string; mode: "edit" };

/** Pre-target seed for opening the create sheet aimed at one Loop (§9.14 CTAs). */
export interface AutomationCreateSeed {
  /** When set, the page opens the create sheet in Run-loop mode for this Loop. */
  loop?: string;
}

export interface AutomationRouteSearch {
  create?: "loop";
  enabled?: boolean;
  event?: string;
  loop?: string;
  q?: string;
  scope?: AutomationScopeFilter;
  source?: AutomationSource;
  view?: ListingViewMode;
}

export function automationListLoopFilter(search: AutomationRouteSearch): string | undefined {
  return search.create === "loop" ? undefined : search.loop;
}

export function automationUnavailableMessage(
  kind: "jobs" | "triggers",
  runtime: SettingsAutomationSection["runtime"] | null,
  error: Error | null
): string | null {
  if (runtime && !runtime.available) {
    const noun = kind === "jobs" ? "Jobs" : "Triggers";
    return `${noun} are unavailable because the automation runtime is disabled. Enable automation and restart the daemon before using this surface.`;
  }

  if (error instanceof AutomationApiError && error.status === 503) {
    const noun = kind === "jobs" ? "jobs" : "triggers";
    return `Automation runtime is unavailable, so ${noun} cannot be loaded.`;
  }

  return null;
}

/** List-level error: runtime-unavailable wins, then the query error — only when no rows loaded. */
export function automationListError(
  runtimeUnavailableMessage: string | null,
  queryError: Error | null,
  itemCount: number
): Error | null {
  if (itemCount > 0) return null;
  if (runtimeUnavailableMessage) return new Error(runtimeUnavailableMessage);
  return queryError;
}

/**
 * Consumes the one-shot `?create=loop&loop=` deep-link shared by the Jobs and
 * Triggers routes. Waits for the active workspace before opening the create
 * editor (the loop-target draft binds workspace scope), then strips the
 * consumed params so a cancel or reload does not re-open the dialog and the
 * list is not silently filtered by `loop`.
 */
export function useAutomationCreateSeed(
  kind: "jobs" | "triggers",
  seed: AutomationCreateSeed,
  activeWorkspaceId: string | null | undefined,
  openLoopCreate: (loop: string) => void
): void {
  const navigate = useNavigate();
  const consumedLoopRef = useRef<string | null>(null);
  useEffect(() => {
    if (!seed.loop) {
      consumedLoopRef.current = null;
      return;
    }
    if (consumedLoopRef.current === seed.loop || !activeWorkspaceId) return;
    consumedLoopRef.current = seed.loop;
    openLoopCreate(seed.loop);
    void navigate({
      replace: true,
      search: current => ({
        ...(current as AutomationRouteSearch),
        create: undefined,
        loop: undefined,
      }),
      to: kind === "jobs" ? "/jobs" : "/triggers",
    });
  }, [activeWorkspaceId, kind, navigate, openLoopCreate, seed.loop]);
}

/**
 * Shared catalog base for the Jobs and Triggers list routes. Owns URL-driven
 * search/filter state, the active workspace context, and the runtime health
 * gate. Detail selection lives in the `$jobId`/`$triggerId` child routes.
 */
export function useAutomationPageBase(
  kind: "jobs" | "triggers",
  search: AutomationRouteSearch = {}
) {
  const navigate = useNavigate();
  const { activeWorkspace, activeWorkspaceId, workspaces } = useActiveWorkspace();
  const settingsQuery = useSettingsAutomation();

  const scopeFilter = search.scope ?? "all";
  const searchQuery = search.q ?? "";
  const view: ListingViewMode = search.view ?? "rows";
  const sourceFilter = search.source ?? null;
  const enabledFilter = search.enabled ?? null;
  const eventFilter = search.event ?? null;

  const scopedWorkspaceId =
    scopeFilter === "workspace" ? (activeWorkspaceId ?? undefined) : undefined;

  const listFilters = {
    limit: 50,
    enabled: search.enabled,
    loop: automationListLoopFilter(search),
    q: searchQuery,
    scope: scopeFilter === "all" ? undefined : scopeFilter,
    source: search.source,
    workspace_id: scopedWorkspaceId,
  };

  const updateSearch = (updates: Partial<AutomationRouteSearch>) => {
    void navigate({
      search: current => ({ ...(current as AutomationRouteSearch), ...updates }),
      to: kind === "jobs" ? "/jobs" : "/triggers",
    });
  };

  const setSearchQuery = (q: string) => updateSearch({ q: normalizeListingSearchValue(q) });
  const setScopeFilter = (scope: AutomationScopeFilter | null) =>
    updateSearch({ scope: scope && scope !== "all" ? scope : undefined });
  const setSourceFilter = (source: AutomationSource | null) =>
    updateSearch({ source: source ?? undefined });
  const setEnabledFilter = (enabled: boolean | null) =>
    updateSearch({ enabled: enabled ?? undefined });
  const setEventFilter = (event: string | null) =>
    updateSearch({ event: event && event.trim() !== "" ? event.trim() : undefined });
  const setView = (nextView: ListingViewMode) =>
    updateSearch({ view: nextView === "rows" ? undefined : nextView });
  const clearFilters = () =>
    updateSearch({
      enabled: undefined,
      event: undefined,
      q: undefined,
      scope: undefined,
      source: undefined,
    });

  const hasActiveFilters =
    searchQuery.trim() !== "" ||
    scopeFilter !== "all" ||
    sourceFilter !== null ||
    enabledFilter !== null ||
    eventFilter !== null;

  return {
    activeWorkspace,
    activeWorkspaceId,
    automationRuntime: settingsQuery.data?.runtime ?? null,
    clearFilters,
    enabledFilter,
    eventFilter,
    hasActiveFilters,
    listFilters,
    scopeFilter,
    searchQuery,
    setEnabledFilter,
    setEventFilter,
    setScopeFilter,
    setSearchQuery,
    setSourceFilter,
    setView,
    sourceFilter,
    view,
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  };
}

export function validateJobsSearch(search: Record<string, unknown>): AutomationRouteSearch {
  return {
    create: search.create === "loop" ? "loop" : undefined,
    enabled: parseAutomationEnabled(search.enabled),
    loop: normalizeListingSearchValue(search.loop),
    q: normalizeListingSearchValue(search.q),
    scope: parseAutomationScope(search.scope),
    source: parseAutomationSource(search.source),
    view: parseListingView(search.view),
  };
}

export function validateTriggersSearch(search: Record<string, unknown>): AutomationRouteSearch {
  return {
    create: search.create === "loop" ? "loop" : undefined,
    enabled: parseAutomationEnabled(search.enabled),
    event: normalizeListingSearchValue(search.event),
    loop: normalizeListingSearchValue(search.loop),
    q: normalizeListingSearchValue(search.q),
    scope: parseAutomationScope(search.scope),
    source: parseAutomationSource(search.source),
    view: parseListingView(search.view),
  };
}
