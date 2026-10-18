/** Which list failed when exactly one of the two loads failed (Business Rule 17). */
export type AutomationPartialFailure = "schedule" | "event" | null;

export interface AutomationStartCounts {
  all: number | null;
  schedule: number | null;
  event: number | null;
}

/** One list query as the listing reads it. */
export interface AutomationListLoad {
  /** The list renders in the current Start view. */
  shown: boolean;
  /** The query runs at all (`target=task` never asks for triggers). */
  fetched: boolean;
  error: Error | null;
  loaded: boolean;
  loading: boolean;
  total: number;
}

export interface AutomationListingStateInput {
  jobs: AutomationListLoad;
  triggers: AutomationListLoad;
  unavailableMessage: string | null;
  itemCount: number;
  hasActiveFilters: boolean;
}

export interface AutomationListingState {
  loadError: Error | null;
  partialFailure: AutomationPartialFailure;
  counts: AutomationStartCounts;
  isLoading: boolean;
  firstRun: boolean;
}

function viewCount(list: AutomationListLoad): number | null {
  if (!list.fetched) return 0;
  return list.error || !list.loaded ? null : list.total;
}

/**
 * Listing system state from the two list loads. Only a rendered kind can fail
 * the listing; the other kind's failure reads "—" in its Start-view count.
 */
export function deriveAutomationListingState({
  jobs,
  triggers,
  unavailableMessage,
  itemCount,
  hasActiveFilters,
}: AutomationListingStateInput): AutomationListingState {
  const jobsError = jobs.shown ? jobs.error : null;
  const triggersError = triggers.shown ? triggers.error : null;
  const shownKinds = Number(jobs.shown) + Number(triggers.shown);
  const failedKinds = Number(jobsError !== null) + Number(triggersError !== null);
  const loadError =
    unavailableMessage === null && shownKinds > 0 && failedKinds === shownKinds
      ? (jobsError ?? triggersError)
      : null;
  const healthy = unavailableMessage === null && loadError === null;
  const partialFailure: AutomationPartialFailure = !healthy
    ? null
    : jobsError
      ? "schedule"
      : triggersError
        ? "event"
        : null;

  const schedule = viewCount(jobs);
  const event = viewCount(triggers);
  const counts: AutomationStartCounts = {
    all: schedule === null || event === null ? null : schedule + event,
    schedule,
    event,
  };

  const isLoading =
    itemCount === 0 && ((jobs.shown && jobs.loading) || (triggers.shown && triggers.loading));
  const firstRun =
    !isLoading &&
    itemCount === 0 &&
    !hasActiveFilters &&
    partialFailure === null &&
    loadError === null;
  return { loadError, partialFailure, counts, isLoading, firstRun };
}
