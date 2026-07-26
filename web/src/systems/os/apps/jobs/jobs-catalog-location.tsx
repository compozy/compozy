import { AlertCircle, Clock3, Plus } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Empty,
  ListingPage,
  ListingToolbar,
  useTopbarSlot,
} from "@agh/ui";
import {
  AutomationEditorDialog,
  AutomationJobsCatalog,
  AutomationListFilters,
  AutomationSuggestionsPanel,
} from "@/systems/automation";
import { useActiveWorkspace } from "@/systems/workspace";
import {
  useAutomationJobsPage,
  type AutomationRouteSearch,
} from "../automation/use-automation-page";

export function JobsCatalogLocation({ search }: { search: AutomationRouteSearch }) {
  const page = useAutomationJobsPage(
    search.create === "loop" && search.loop ? { loop: search.loop } : {},
    search
  );
  const { activeWorkspaceId } = useActiveWorkspace();

  useTopbarSlot({
    glyph: <Clock3 />,
    count: page.total,
    actions: (
      <Button data-testid="create-job-btn" onClick={page.handleCreate} size="sm" type="button">
        <Plus aria-hidden="true" className="size-3" />
        Job
      </Button>
    ),
    toolbar:
      page.isLoading || page.error ? undefined : (
        <ListingToolbar>
          <ListingToolbar.Leading>
            <ListingToolbar.Search
              aria-label="Search jobs"
              data-testid="automation-search-input"
              onChange={page.setSearchQuery}
              placeholder="Search jobs"
              value={page.searchQuery}
            />
            <ListingToolbar.Filters>
              <AutomationListFilters
                enabledFilter={page.enabledFilter}
                kind="jobs"
                onEnabledFilterChange={page.setEnabledFilter}
                onScopeFilterChange={page.setScopeFilter}
                onSourceFilterChange={page.setSourceFilter}
                scopeFilter={page.scopeFilter}
                sourceFilter={page.sourceFilter}
              />
            </ListingToolbar.Filters>
          </ListingToolbar.Leading>
          <ListingToolbar.Trailing>
            <ListingToolbar.ViewToggle onChange={page.setView} value={page.view} />
          </ListingToolbar.Trailing>
        </ListingToolbar>
      ),
  });

  if (page.error) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="jobs-error"
      >
        <Empty
          className="max-w-md"
          description={page.error.message ?? "Failed to load jobs"}
          icon={AlertCircle}
          title="Unable to load jobs"
        />
      </div>
    );
  }

  return (
    <>
      <ListingPage
        banner={
          page.runtimeUnavailableMessage ? (
            <div className="border-b border-line px-9 py-3">
              <Alert data-testid="jobs-runtime-alert" variant="warning">
                <AlertCircle aria-hidden="true" className="size-4" />
                <AlertTitle>Automation runtime unavailable</AlertTitle>
                <AlertDescription>{page.runtimeUnavailableMessage}</AlertDescription>
              </Alert>
            </div>
          ) : null
        }
        data-testid="jobs-shell"
      >
        {!page.isLoading && activeWorkspaceId && search.scope !== "global" ? (
          <AutomationSuggestionsPanel key={activeWorkspaceId} workspaceID={activeWorkspaceId} />
        ) : null}
        <AutomationJobsCatalog
          errorMessage={page.errorMessage}
          hasActiveFilters={page.hasActiveFilters}
          isLoading={page.isLoading}
          jobs={page.jobs}
          onClearFilters={page.clearFilters}
          onCreate={page.handleCreate}
          onRun={page.onRunJob}
          pagination={{
            hasNextPage: page.hasNextPage,
            isFetchingNextPage: page.isFetchingNextPage,
            onLoadMore: page.loadMore,
          }}
          runDisabled={page.runDisabled}
          runPendingIds={page.runPendingIds}
          view={page.view}
        />
      </ListingPage>

      <AutomationEditorDialog {...page.editorDialogProps} />
    </>
  );
}
