import type { ReactNode } from "react";

import type { ListingViewMode } from "@compozy/ui";

import type { AutomationJob } from "../types";
import {
  AutomationCatalogShell,
  type AutomationCatalogPagination,
} from "./automation-catalog-shell";
import { AutomationJobCard } from "./automation-job-card";
import { AutomationJobRow } from "./automation-job-row";
import type { ProfileListingScope } from "@/systems/profiles";

export interface AutomationJobsCatalogProps {
  jobs: AutomationJob[];
  view: ListingViewMode;
  isLoading: boolean;
  errorMessage: string | null;
  hasActiveFilters: boolean;
  pagination: AutomationCatalogPagination;
  runDisabled: boolean;
  runPendingIds: ReadonlySet<string>;
  onClearFilters: () => void;
  onCreate: () => void;
  onRun: (id: string) => void;
  unfilteredEmptyPanel?: ReactNode;
  /** Owner tags in aggregate mode; names the listing scope in the empty state. */
  profileScope: ProfileListingScope;
}

/** Jobs catalog body: rows or cards with shared empty/loading/error/load-more. */
function AutomationJobsCatalog({
  jobs,
  view,
  isLoading,
  errorMessage,
  hasActiveFilters,
  pagination,
  runDisabled,
  runPendingIds,
  onClearFilters,
  onCreate,
  onRun,
  profileScope,
  unfilteredEmptyPanel,
}: AutomationJobsCatalogProps) {
  return (
    <AutomationCatalogShell
      errorMessage={errorMessage}
      hasActiveFilters={hasActiveFilters}
      isLoading={isLoading}
      itemCount={jobs.length}
      kind="jobs"
      onClearFilters={onClearFilters}
      onCreate={onCreate}
      pagination={pagination}
      profileScope={profileScope}
      unfilteredEmptyPanel={unfilteredEmptyPanel}
      view={view}
    >
      {jobs.map(job =>
        view === "cards" ? (
          <AutomationJobCard
            isRunPending={runPendingIds.has(job.id)}
            job={job}
            key={job.id}
            owner={profileScope.aggregate ? profileScope.ownerOf(job) : undefined}
            onRun={onRun}
            runDisabled={runDisabled}
          />
        ) : (
          <AutomationJobRow
            isRunPending={runPendingIds.has(job.id)}
            job={job}
            key={job.id}
            profileOwner={profileScope.aggregate ? profileScope.ownerOf(job) : undefined}
            onRun={onRun}
            runDisabled={runDisabled}
          />
        )
      )}
    </AutomationCatalogShell>
  );
}

export { AutomationJobsCatalog };
