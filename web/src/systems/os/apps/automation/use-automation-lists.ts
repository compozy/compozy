import { useAutomationJobs, useAutomationTriggers } from "@/systems/automation";

type ListFilters = Parameters<typeof useAutomationJobs>[0];

/**
 * Both automation lists for the listing. They load in every Start view so the
 * view counts and the window total stay honest on a cold load; a Start view only
 * decides which kind renders. Task targets exist only on jobs, so `target=task`
 * never asks for triggers.
 */
export function useAutomationLists({
  filters,
  start,
  targetFilter,
}: {
  filters: ListFilters;
  start: "schedule" | "event" | null;
  targetFilter: string | null | undefined;
}) {
  const fetchTriggers = targetFilter !== "task";
  const showJobs = start !== "event";
  const showTriggers = start !== "schedule" && fetchTriggers;
  const jobsQuery = useAutomationJobs(filters);
  const triggersQuery = useAutomationTriggers(filters, { enabled: fetchTriggers });
  const jobsMore = Boolean(jobsQuery.hasNextPage);
  const triggersMore = fetchTriggers && Boolean(triggersQuery.hasNextPage);

  return {
    fetchTriggers,
    showJobs,
    showTriggers,
    jobsQuery,
    triggersQuery,
    loadedTriggers: fetchTriggers ? triggersQuery.triggers : [],
    triggersError: fetchTriggers ? triggersQuery.error : null,
    /** Some list still has pages the loaded rows don't cover. */
    hasMorePages: jobsMore || triggersMore,
    canLoadMore: (showJobs && jobsMore) || (showTriggers && triggersMore),
    isFetchingMore: jobsQuery.isFetchingNextPage || triggersQuery.isFetchingNextPage,
    isPaused: jobsQuery.isPaused || triggersQuery.isPaused,
    loadMore: () => {
      if (showJobs && jobsMore) void jobsQuery.fetchNextPage();
      if (showTriggers && triggersMore) void triggersQuery.fetchNextPage();
    },
    retry: () => {
      void jobsQuery.refetch();
      if (fetchTriggers) void triggersQuery.refetch();
    },
  };
}
