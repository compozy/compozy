import { Plus, Zap } from "lucide-react";

import { Button, ListingPage, useTopbarSlot } from "@compozy/ui";

import { useAutomationsPage } from "../automation/use-automation-page";
import {
  AutomationCard,
  AutomationCatalogShell,
  AutomationDeleteAction,
  AutomationEditorDialog,
  AutomationRow,
  automationDetailSearchFrom,
  type AutomationItemControls,
  type AutomationsRouteSearch,
} from "@/systems/automation";
import { AutomationListFooter } from "./automations-catalog-parts";
import {
  AutomationsCatalogBanner,
  AutomationsCatalogToolbar,
  AutomationsEmptySuggestions,
  AutomationsFirstRunActions,
} from "./automations-catalog-sections";

export function AutomationsCatalogLocation({ search }: { search: AutomationsRouteSearch }) {
  const page = useAutomationsPage(search);
  const firstRun = page.firstRun;
  const unavailable = page.unavailableMessage !== null;

  useTopbarSlot({
    glyph: <Zap />,
    count: page.total ?? "—",
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
    // Search, filters and the view toggle are static controls: they stay while the list loads, so
    // the topbar does not grow when the rows arrive. Counts read "—" until both lists answer.
    toolbar: page.loadError || firstRun ? undefined : <AutomationsCatalogToolbar page={page} />,
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
        banner={<AutomationsCatalogBanner page={page} />}
        data-testid="automations-shell"
      >
        <AutomationCatalogShell
          firstRunActions={<AutomationsFirstRunActions />}
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
          unfilteredEmptyPanel={<AutomationsEmptySuggestions page={page} />}
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
            loadedCount={page.loadedCount}
            nextRunAt={page.nextRunAt}
            total={page.total}
          />
        ) : null}
      </ListingPage>

      <AutomationDeleteAction
        consequence="Past runs stay."
        hideTrigger
        isPending={page.deletePending}
        name={page.deleteTarget?.name ?? ""}
        onConfirm={page.confirmDelete}
        onOpenChange={open => {
          if (!open) page.setDeleteTarget(null);
        }}
        open={page.deleteTarget !== null}
      />
      <AutomationEditorDialog {...page.editorDialogProps} />
    </>
  );
}
