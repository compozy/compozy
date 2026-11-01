import { AlertCircle } from "@compozy/ui/icons";
import { Link } from "@tanstack/react-router";

import { Alert, AlertDescription, AlertTitle, ListingToolbar, buttonVariants } from "@compozy/ui";

import type { useAutomationsPage } from "../automation/use-automation-page";
import {
  AutomationListFilters,
  AutomationStartViews,
  AutomationSuggestionsPanel,
} from "@/systems/automation";
import { WorkspaceBadge } from "@/systems/workspace";
import { AutomationPartialAlert } from "./automations-catalog-parts";

type AutomationsPage = ReturnType<typeof useAutomationsPage>;

/** Start views, search, filters and the cards/rows toggle of the automations listing. */
export function AutomationsCatalogToolbar({ page }: { page: AutomationsPage }) {
  return (
    <ListingToolbar>
      <ListingToolbar.Leading>
        <AutomationStartViews counts={page.counts} onChange={page.setStart} value={page.start} />
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
              onProjectsChange: page.setProjectFilter,
              onSourceChange: page.setSourceFilter,
              onTargetChange: page.setTargetFilter,
            }}
            projectField={page.projectFilterField}
            state={{
              enabled: page.enabledFilter,
              loop: page.loopFilter,
              projects: page.projectFilter,
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
  );
}

/** Runtime unavailability, a partial kind failure, or a failed refresh over loaded rows. */
export function AutomationsCatalogBanner({ page }: { page: AutomationsPage }) {
  if (page.unavailableMessage) {
    return (
      <div className="border-b border-line px-9 py-3">
        <Alert data-testid="automations-runtime-alert" variant="warning">
          <AlertCircle aria-hidden="true" className="size-4" />
          <AlertTitle>Automations aren&apos;t available right now</AlertTitle>
          <AlertDescription>
            {page.unavailableMessage}
            <div className="mt-2">
              <Link
                className={buttonVariants({ size: "sm", variant: "secondary" })}
                to="/settings/automation"
              >
                Open Settings
              </Link>
            </div>
          </AlertDescription>
        </Alert>
      </div>
    );
  }
  if (page.partialFailure) {
    return <AutomationPartialAlert failed={page.partialFailure} onRetry={page.retry} />;
  }
  if (page.loadError && page.items.length > 0) {
    return <AutomationPartialAlert failed="refresh" onRetry={page.retry} />;
  }
  return null;
}

/** First-run starters: a schedule or an event. */
export function AutomationsFirstRunActions() {
  return (
    <div className="flex gap-2">
      <Link
        className={buttonVariants({ size: "sm", variant: "neutral" })}
        data-testid="automations-empty-start-schedule"
        search={{ create: 1, start: "schedule" }}
        to="/automations"
      >
        On a schedule
      </Link>
      <Link
        className={buttonVariants({ size: "sm", variant: "neutral" })}
        data-testid="automations-empty-start-event"
        search={{ create: 1, start: "event" }}
        to="/automations"
      >
        When something happens
      </Link>
    </div>
  );
}

/** Suggestions shown in the unfiltered empty listing, badged by project under All projects. */
export function AutomationsEmptySuggestions({ page }: { page: AutomationsPage }) {
  const source = page.suggestionsSource;
  if (!source) return null;
  const suggestionProject = page.suggestionProject;
  return (
    <AutomationSuggestionsPanel
      key={source.kind === "all" ? "all" : source.workspaceID}
      projectBadge={
        suggestionProject
          ? suggestion => <WorkspaceBadge identity={suggestionProject(suggestion)} />
          : undefined
      }
      source={source}
      timeZone={page.timeZone}
    />
  );
}
