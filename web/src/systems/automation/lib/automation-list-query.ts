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

export function normalizeAutomationJobFilter(
  filters: AutomationJobStableFilter = {}
): AutomationJobStableFilter {
  return {
    scope: filters.scope,
    workspace_id: normalizeOptionalText(filters.workspace_id),
    source: filters.source,
    enabled: filters.enabled,
    q: normalizeOptionalText(filters.q),
    limit: filters.limit,
    loop: normalizeOptionalText(filters.loop),
  };
}

export function normalizeAutomationTriggerFilter(
  filters: AutomationTriggerStableFilter = {}
): AutomationTriggerStableFilter {
  return {
    scope: filters.scope,
    workspace_id: normalizeOptionalText(filters.workspace_id),
    source: filters.source,
    enabled: filters.enabled,
    event: normalizeOptionalText(filters.event),
    q: normalizeOptionalText(filters.q),
    limit: filters.limit,
    loop: normalizeOptionalText(filters.loop),
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
