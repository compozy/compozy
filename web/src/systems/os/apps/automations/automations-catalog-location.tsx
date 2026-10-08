import { AlertCircle, Plus, Zap } from "lucide-react";
import { Link } from "@tanstack/react-router";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  ListingPage,
  ListingToolbar,
  useTopbarSlot,
} from "@compozy/ui";

import { useAutomationsPage } from "../automation/use-automation-page";
import {
  AutomationCard,
  AutomationCatalogShell,
  AutomationEditorDialog,
  AutomationListFilters,
  AutomationRow,
  AutomationStartViews,
  AutomationSuggestionsPanel,
  automationDetailSearchFrom,
  type AutomationItemControls,
  type AutomationsRouteSearch,
} from "@/systems/automation";
import {
  AutomationListFooter,
  AutomationPartialAlert,
  AutomationDeleteDialog,
} from "./automations-catalog-parts";

export function AutomationsCatalogLocation({ search }: { search: AutomationsRouteSearch }) {
  const page = useAutomationsPage(search);
  const firstRun = page.firstRun;
  const unavailable = page.unavailableMessage !== null;

  useTopbarSlot({
    glyph: <Zap />,
    count: page.total,
    actions: (
      <Button
        data-testid="automations-create"
        onClick={() => page.create()}
        size="sm"
        type="button"
        variant="secondary"
      >
        <Plus aria-hidden="true" data-icon="inline-start" />
        New automation
      </Button>
    ),
    toolbar:
      page.isLoading || page.loadError || firstRun ? undefined : (
        <ListingToolbar>
          <ListingToolbar.Leading>
            <AutomationStartViews
              counts={page.counts}
              onChange={page.setStart}
              value={page.start}
            />
            <ListingToolbar.Search
              aria-label="Search automations"
              data-testid="automation-search-input"
              onChange={page.setSearchQuery}
              placeholder="Search automations"
              value={page.searchQuery}
            />
            <ListingToolbar.Filters>
              <AutomationListFilters
                handlers={{
                  onEnabledChange: page.setEnabledFilter,
                  onLoopChange: page.setLoopFilter,
                  onScopeChange: page.setScopeFilter,
                  onSourceChange: page.setSourceFilter,
                  onTargetChange: page.setTargetFilter,
                }}
                state={{
                  enabled: page.enabledFilter,
                  loop: page.loopFilter,
                  scope: page.scopeFilter,
                  source: page.sourceFilter,
                  target: page.targetFilter,
                }}
              />
            </ListingToolbar.Filters>
          </ListingToolbar.Leading>
          <ListingToolbar.Trailing>
            <ListingToolbar.ViewToggle onChange={page.setView} value={page.view} />
          </ListingToolbar.Trailing>
        </ListingToolbar>
      ),
  });

  const controls: AutomationItemControls = {
    detailSearch: automationDetailSearchFrom(search),
    unavailable,
    isRunPending: page.isRunPending,
    isTogglePending: page.isTogglePending,
    onToggleEnabled: page.toggleEnabled,
    onRunNow: page.runNow,
    onEdit: page.edit,
    onDelete: page.setDeleteTarget,
    onCopyLink: page.copyLink,
  };

  return (
    <>
      <ListingPage
        banner={
          page.unavailableMessage ? (
            <div className="border-b border-line px-9 py-3">
              <Alert data-testid="automations-runtime-alert" variant="warning">
                <AlertCircle aria-hidden="true" className="size-4" />
                <AlertTitle>Automations aren&apos;t available right now</AlertTitle>
                <AlertDescription>
                  {page.unavailableMessage}
                  <div className="mt-2">
                    <Button
                      nativeButton={false}
                      render={<Link to="/settings/automation" />}
                      size="sm"
                      variant="secondary"
                    >
                      Open Settings
                    </Button>
                  </div>
                </AlertDescription>
              </Alert>
            </div>
          ) : page.partialFailure ? (
            <AutomationPartialAlert failed={page.partialFailure} onRetry={page.retry} />
          ) : null
        }
        data-testid="automations-shell"
      >
        <AutomationCatalogShell
          firstRunActions={
            <div className="flex gap-2">
              <Button
                data-testid="automations-empty-start-schedule"
                nativeButton={false}
                render={<Link search={{ create: "1", start: "schedule" }} to="/automations" />}
                size="sm"
                variant="neutral"
              >
                On a schedule
              </Button>
              <Button
                data-testid="automations-empty-start-event"
                nativeButton={false}
                render={<Link search={{ create: "1", start: "event" }} to="/automations" />}
                size="sm"
                variant="neutral"
              >
                When something happens
              </Button>
            </div>
          }
          hasActiveFilters={page.hasActiveFilters}
          isLoading={page.isLoading}
          itemCount={page.items.length}
          loadError={
            page.loadError ? { message: page.loadError.message, onRetry: page.retry } : null
          }
          onClearFilters={page.clearFilters}
          pagination={{
            hasNextPage: page.canLoadMore,
            isFetchingNextPage: page.isFetchingMore,
            isPaused: page.isPaused,
            onLoadMore: page.loadMore,
          }}
          profileScope={page.profileScope}
          unfilteredEmptyPanel={
            page.suggestionsWorkspaceId ? (
              <AutomationSuggestionsPanel
                key={page.suggestionsWorkspaceId}
                workspaceID={page.suggestionsWorkspaceId}
              />
            ) : null
          }
          view={page.view}
        >
          {page.items.map(view =>
            page.view === "cards" ? (
              <AutomationCard controls={controls} key={`${view.kind}:${view.id}`} view={view} />
            ) : (
              <AutomationRow controls={controls} key={`${view.kind}:${view.id}`} view={view} />
            )
          )}
        </AutomationCatalogShell>
        {page.items.length > 0 ? (
          <AutomationListFooter
            enabledCount={page.enabledCount}
            nextRunAt={page.nextRunAt}
            total={page.total}
          />
        ) : null}
      </ListingPage>

      <AutomationDeleteDialog
        onConfirm={page.confirmDelete}
        onOpenChange={open => {
          if (!open) page.setDeleteTarget(null);
        }}
        pending={page.deletePending}
        target={page.deleteTarget}
      />
      <AutomationEditorDialog {...page.editorDialogProps} />
    </>
  );
}
