import { AlertCircle, Plus, Zap } from "lucide-react";

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
  AutomationListFilters,
  AutomationTriggersCatalog,
} from "@/systems/automation";
import {
  useAutomationTriggersPage,
  type AutomationRouteSearch,
} from "../automation/use-automation-page";

export function TriggersCatalogLocation({ search }: { search: AutomationRouteSearch }) {
  const page = useAutomationTriggersPage(
    search.create === "loop" && search.loop ? { loop: search.loop } : {},
    search
  );

  useTopbarSlot({
    glyph: <Zap />,
    count: page.total,
    actions: (
      <Button data-testid="create-trigger-btn" onClick={page.handleCreate} size="sm" type="button">
        <Plus aria-hidden="true" className="size-3" />
        Trigger
      </Button>
    ),
    toolbar:
      page.isLoading || page.error ? undefined : (
        <ListingToolbar>
          <ListingToolbar.Leading>
            <ListingToolbar.Search
              aria-label="Search triggers"
              data-testid="automation-search-input"
              onChange={page.setSearchQuery}
              placeholder="Search triggers"
              value={page.searchQuery}
            />
            <ListingToolbar.Filters>
              <AutomationListFilters
                enabledFilter={page.enabledFilter}
                eventFilter={page.eventFilter}
                kind="triggers"
                onEnabledFilterChange={page.setEnabledFilter}
                onEventFilterChange={page.setEventFilter}
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
        data-testid="triggers-error"
      >
        <Empty
          className="max-w-md"
          description={page.error.message ?? "Failed to load triggers"}
          icon={AlertCircle}
          title="Unable to load triggers"
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
              <Alert data-testid="triggers-runtime-alert" variant="warning">
                <AlertCircle aria-hidden="true" className="size-4" />
                <AlertTitle>Automation runtime unavailable</AlertTitle>
                <AlertDescription>{page.runtimeUnavailableMessage}</AlertDescription>
              </Alert>
            </div>
          ) : null
        }
        data-testid="triggers-shell"
      >
        <AutomationTriggersCatalog
          errorMessage={page.errorMessage}
          hasActiveFilters={page.hasActiveFilters}
          isLoading={page.isLoading}
          onClearFilters={page.clearFilters}
          onCreate={page.handleCreate}
          pagination={{
            hasNextPage: page.hasNextPage,
            isFetchingNextPage: page.isFetchingNextPage,
            onLoadMore: page.loadMore,
          }}
          triggers={page.triggers}
          view={page.view}
        />
      </ListingPage>

      <AutomationEditorDialog {...page.editorDialogProps} />
    </>
  );
}
