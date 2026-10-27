import type { InfiniteData } from "@tanstack/react-query";

import type {
  AutomationJob,
  AutomationJobListFilter,
  AutomationJobsListResponse,
  AutomationJobStableFilter,
  AutomationTrigger,
  AutomationTriggerListFilter,
  AutomationTriggersListResponse,
  AutomationTriggerStableFilter,
} from "../types";

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function normalizedWorkspaceIds(values?: readonly string[]): string[] | undefined {
  const ids = new Set<string>();
  for (const value of values ?? []) {
    const id = value.trim();
    if (id !== "") ids.add(id);
  }
  return ids.size > 0 ? [...ids].sort() : undefined;
}

export function normalizeAutomationJobFilter(
  filters: AutomationJobStableFilter = {}
): AutomationJobStableFilter {
  return {
    scope: filters.scope,
    workspace_id: normalizeOptionalText(filters.workspace_id),
    all_workspaces: filters.all_workspaces || undefined,
    // The Project filter narrows the aggregate only; sorted so equal filters share a key.
    workspace_ids: filters.all_workspaces
      ? normalizedWorkspaceIds(filters.workspace_ids)
      : undefined,
    source: filters.source,
    target: filters.target,
    enabled: filters.enabled,
    q: normalizeOptionalText(filters.q),
    limit: filters.limit,
    loop: normalizeOptionalText(filters.loop),
    // Profile scope rides the filter, so it lands in both request and key.
    profile: normalizeOptionalText(filters.profile),
    all_profiles: filters.all_profiles,
  };
}

export function normalizeAutomationTriggerFilter(
  filters: AutomationTriggerStableFilter = {}
): AutomationTriggerStableFilter {
  return {
    scope: filters.scope,
    workspace_id: normalizeOptionalText(filters.workspace_id),
    all_workspaces: filters.all_workspaces || undefined,
    // The Project filter narrows the aggregate only; sorted so equal filters share a key.
    workspace_ids: filters.all_workspaces
      ? normalizedWorkspaceIds(filters.workspace_ids)
      : undefined,
    source: filters.source,
    target: filters.target,
    enabled: filters.enabled,
    event: normalizeOptionalText(filters.event),
    q: normalizeOptionalText(filters.q),
    limit: filters.limit,
    loop: normalizeOptionalText(filters.loop),
    // Profile scope rides the filter, so it lands in both request and key.
    profile: normalizeOptionalText(filters.profile),
    all_profiles: filters.all_profiles,
  };
}

export function automationJobRequest(
  filters: AutomationJobStableFilter,
  cursor?: string
): AutomationJobListFilter {
  return cursor ? { ...filters, cursor } : filters;
}

export function automationTriggerRequest(
  filters: AutomationTriggerStableFilter,
  cursor?: string
): AutomationTriggerListFilter {
  return cursor ? { ...filters, cursor } : filters;
}

function flattenUnique<T extends { id: string }>(pages: readonly T[][]): T[] {
  const seen = new Set<string>();
  return pages.flatMap(page =>
    page.filter(item => {
      if (seen.has(item.id)) return false;
      seen.add(item.id);
      return true;
    })
  );
}

export function flattenAutomationJobPages(
  data: InfiniteData<AutomationJobsListResponse, unknown> | undefined
): AutomationJob[] {
  return flattenUnique(data?.pages.map(page => page.jobs) ?? []);
}

export function flattenAutomationTriggerPages(
  data: InfiniteData<AutomationTriggersListResponse, unknown> | undefined
): AutomationTrigger[] {
  return flattenUnique(data?.pages.map(page => page.triggers) ?? []);
}

/**
 * The filters a search edit leaves alone, as the placeholder rule compares them: while an edited
 * search loads, the previous rows stay on screen, but only within the same scope, profile and
 * facets. Stored in the query `meta` so the rule never depends on where `q` sits in the key.
 */
export function automationListPopulation(
  filters: AutomationJobStableFilter | AutomationTriggerStableFilter
): string {
  const { q: _search, ...population } = filters;
  return JSON.stringify(population);
}
