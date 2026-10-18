import { AlertCircle, Inbox, ListFilter } from "lucide-react";

import {
  Button,
  Empty,
  Eyebrow,
  FiltersWithSearch,
  ListingPage,
  SearchInput,
  Spinner,
  StateGlyph,
  Switch,
} from "@compozy/ui";

import {
  INBOX_GROUPS,
  type InboxGroupDefinition,
  type InboxLaneFilterId,
} from "../lib/inbox-grouping";
import { useTasksInboxView } from "../hooks/use-tasks-inbox-view";
import type { TaskInboxItem, TaskInboxView, TaskPriority, TaskStatus } from "../types";
import { TasksInboxItem, type TasksInboxItemProps } from "./tasks-inbox-item";
import { TaskRowsLoadingSkeleton } from "./task-loading-skeletons";

export interface TasksInboxViewProps {
  inbox: TaskInboxView | null;
  laneFilter: InboxLaneFilterId;
  onLaneChange: (lane: InboxLaneFilterId) => void;
  statusFilter: TaskStatus | null;
  onStatusChange: (next: TaskStatus | null) => void;
  priorityFilter: TaskPriority | null;
  onPriorityChange: (next: TaskPriority | null) => void;
  unreadOnly: boolean;
  onToggleUnread: (next: boolean) => void;
  searchQuery: string;
  onSearchChange: (value: string) => void;
  isLoading?: boolean;
  errorMessage?: string | null;
  onApprove?: TasksInboxItemProps["onApprove"];
  onReject?: TasksInboxItemProps["onReject"];
  onRetry?: TasksInboxItemProps["onRetry"];
  onArchive?: TasksInboxItemProps["onArchive"];
  onDismiss?: TasksInboxItemProps["onDismiss"];
  onMarkRead?: TasksInboxItemProps["onMarkRead"];
  onOpen?: TasksInboxItemProps["onOpen"];
  pendingApproveIds?: ReadonlySet<string>;
  pendingRejectIds?: ReadonlySet<string>;
  pendingRetryIds?: ReadonlySet<string>;
  pendingArchiveIds?: ReadonlySet<string>;
  pendingDismissIds?: ReadonlySet<string>;
  pendingMarkReadIds?: ReadonlySet<string>;
  hasMore?: boolean;
  isLoadingMore?: boolean;
  onLoadMore?: () => void;
  onRetryQuery?: () => void;
}

type InboxItemActionProps = Omit<TasksInboxItemProps, "item" | "group">;

export function TasksInboxView({
  inbox,
  laneFilter,
  onLaneChange,
  statusFilter,
  onStatusChange,
  priorityFilter,
  onPriorityChange,
  unreadOnly,
  onToggleUnread,
  searchQuery,
  onSearchChange,
  isLoading = false,
  errorMessage = null,
  hasMore = false,
  isLoadingMore = false,
  onLoadMore,
  onRetryQuery,
  ...itemActionProps
}: TasksInboxViewProps) {
  const { filterChips, filterFields, groups, groupTotals, handleFiltersChange, hasItems } =
    useTasksInboxView({
      inbox,
      laneFilter,
      onLaneChange,
      statusFilter,
      onStatusChange,
      priorityFilter,
      onPriorityChange,
    });

  return (
    <ListingPage className="bg-canvas" data-testid="tasks-inbox-view">
      <div
        className="flex flex-wrap items-center gap-2 border-b border-line-soft pb-3"
        data-testid="tasks-inbox-toolbar"
      >
        <SearchInput
          containerClassName="w-64 max-w-full"
          data-testid="tasks-inbox-search"
          onChange={next => onSearchChange(next)}
          placeholder="Search inbox…"
          value={searchQuery}
        />
        <FiltersWithSearch<string>
          allowMultiple={false}
          fields={filterFields}
          filters={filterChips}
          onChange={handleFiltersChange}
          size="sm"
          trigger={
            <Button
              aria-label="Filter inbox"
              data-testid="tasks-inbox-filter-trigger"
              size="sm"
              type="button"
              variant="ghost"
            >
              <ListFilter aria-hidden="true" data-icon="inline-start" />
              Filter
            </Button>
          }
        />
        <label
          className="ml-auto inline-flex items-center gap-2"
          data-testid="tasks-inbox-unread-toggle"
          htmlFor="tasks-inbox-unread-only"
        >
          <Switch
            checked={unreadOnly}
            id="tasks-inbox-unread-only"
            onCheckedChange={onToggleUnread}
          />
          <Eyebrow className="text-muted">Unread only</Eyebrow>
        </label>
      </div>

      <div className="mt-4 flex min-h-0 flex-1 flex-col gap-6" data-testid="tasks-inbox-body">
        <TasksInboxContent
          errorMessage={errorMessage}
          groupTotals={groupTotals}
          groups={groups}
          hasItems={hasItems}
          inbox={inbox}
          isLoading={isLoading}
          itemActionProps={itemActionProps}
          onRetryQuery={onRetryQuery}
        />
        {errorMessage && inbox ? (
          <div
            className="flex items-center justify-between gap-3 border-t border-line-soft pt-3 text-caption text-danger"
            data-testid="tasks-inbox-pagination-error"
            role="alert"
          >
            <span>{errorMessage}</span>
            {onRetryQuery ? <TasksInboxRetryButton onRetryQuery={onRetryQuery} /> : null}
          </div>
        ) : null}
        {hasMore && onLoadMore && !errorMessage ? (
          <TasksInboxLoadMore isLoadingMore={isLoadingMore} onLoadMore={onLoadMore} />
        ) : null}
      </div>
    </ListingPage>
  );
}

type InboxViewModel = ReturnType<typeof useTasksInboxView>;

function TasksInboxContent({
  inbox,
  hasItems,
  groups,
  groupTotals,
  itemActionProps,
  isLoading,
  errorMessage,
  onRetryQuery,
}: Pick<InboxViewModel, "groups" | "groupTotals" | "hasItems"> & {
  inbox: TaskInboxView | null;
  itemActionProps: InboxItemActionProps;
  isLoading: boolean;
  errorMessage: string | null;
  onRetryQuery?: () => void;
}) {
  if (!inbox) {
    return (
      <TasksInboxUnloadedState
        errorMessage={errorMessage}
        isLoading={isLoading}
        onRetryQuery={onRetryQuery}
      />
    );
  }
  if (!hasItems) {
    return <TasksInboxEmptyState />;
  }
  return (
    <div className="flex flex-col gap-6" data-testid="tasks-inbox-groups">
      {INBOX_GROUPS.map(group => {
        const bucket = groups.get(group.id) ?? [];
        if (bucket.length === 0) {
          return null;
        }
        return (
          <GroupSection
            group={group}
            items={bucket}
            itemActionProps={itemActionProps}
            key={group.id}
            totalCount={groupTotals[group.id]}
          />
        );
      })}
    </div>
  );
}

function TasksInboxRetryButton({ onRetryQuery }: { onRetryQuery: () => void }) {
  return (
    <Button onClick={onRetryQuery} size="sm" type="button" variant="ghost">
      Retry loading inbox
    </Button>
  );
}

/** Body for an inbox that has not loaded yet: loading, failed, or not requested. */
function TasksInboxUnloadedState({
  isLoading,
  errorMessage,
  onRetryQuery,
}: {
  isLoading: boolean;
  errorMessage: string | null;
  onRetryQuery?: () => void;
}) {
  if (isLoading) {
    return <TaskRowsLoadingSkeleton label="Loading inbox" testId="tasks-inbox-loading" />;
  }
  if (errorMessage) {
    return (
      <Empty
        action={onRetryQuery ? <TasksInboxRetryButton onRetryQuery={onRetryQuery} /> : null}
        data-testid="tasks-inbox-error"
        description={errorMessage}
        icon={AlertCircle}
        title="Couldn't load the inbox"
      />
    );
  }
  return <TasksInboxEmptyState />;
}

function TasksInboxEmptyState() {
  return (
    <Empty
      className="mx-auto max-w-xl"
      data-testid="tasks-inbox-empty"
      description="Approvals and failed runs that need you show up here."
      icon={Inbox}
      title="You're all caught up"
    />
  );
}

function TasksInboxLoadMore({
  isLoadingMore,
  onLoadMore,
}: {
  isLoadingMore: boolean;
  onLoadMore: () => void;
}) {
  return (
    <div className="flex items-center justify-center border-t border-line-soft pt-3">
      <Button
        aria-busy={isLoadingMore}
        aria-label={isLoadingMore ? "Loading more inbox tasks" : "Load more inbox tasks"}
        data-testid="tasks-inbox-load-more"
        disabled={isLoadingMore}
        onClick={onLoadMore}
        size="sm"
        type="button"
        variant="ghost"
      >
        {isLoadingMore ? (
          <Spinner aria-hidden="true" className="size-3.5" data-icon="inline-start" />
        ) : null}
        {isLoadingMore ? "Loading more" : "Load more"}
      </Button>
    </div>
  );
}

interface GroupSectionProps {
  group: InboxGroupDefinition;
  items: TaskInboxItem[];
  itemActionProps: InboxItemActionProps;
  totalCount: number;
}

function GroupSection({ group, items, itemActionProps, totalCount }: GroupSectionProps) {
  const countLabel =
    items.length === totalCount ? `${items.length}` : `${items.length} of ${totalCount}`;
  return (
    <section className="flex flex-col gap-2" data-testid={`tasks-inbox-group-${group.id}`}>
      <header className="flex items-center gap-2">
        <StateGlyph data-testid={`tasks-inbox-group-dot-${group.id}`} state={group.glyph} />
        <Eyebrow>{group.label}</Eyebrow>
        <span
          className="font-mono text-badge tabular-nums text-faint"
          data-testid={`tasks-inbox-group-count-${group.id}`}
        >
          {countLabel}
        </span>
      </header>
      <div className="flex flex-col">
        {items.map(item => (
          <TasksInboxItem key={item.task.id} {...itemActionProps} group={group.id} item={item} />
        ))}
      </div>
    </section>
  );
}
