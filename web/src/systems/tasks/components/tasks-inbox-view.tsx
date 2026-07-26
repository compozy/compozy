import { AlertCircle, ListFilter, Search } from "lucide-react";

import {
  Button,
  Empty,
  Eyebrow,
  FiltersWithSearch,
  ListingPage,
  SearchInput,
  Spinner,
  StatusDot,
  Switch,
} from "@agh/ui";

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
  onApprove,
  onReject,
  onRetry,
  onArchive,
  onDismiss,
  onMarkRead,
  onOpen,
  pendingApproveIds,
  pendingRejectIds,
  pendingRetryIds,
  pendingArchiveIds,
  pendingDismissIds,
  pendingMarkReadIds,
  hasMore = false,
  isLoadingMore = false,
  onLoadMore,
  onRetryQuery,
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

  const itemActionProps: Omit<TasksInboxItemProps, "item" | "group"> = {
    onApprove,
    onReject,
    onRetry,
    onArchive,
    onDismiss,
    onMarkRead,
    onOpen,
    pendingApproveIds,
    pendingRejectIds,
    pendingRetryIds,
    pendingArchiveIds,
    pendingDismissIds,
    pendingMarkReadIds,
  };

  return (
    <ListingPage className="bg-canvas" data-testid="tasks-inbox-view">
      <div
        className="flex flex-wrap items-center gap-2 border-b border-line-soft pb-3"
        data-testid="tasks-inbox-toolbar"
      >
        <SearchInput
          className="h-8 w-64 max-w-full"
          data-testid="tasks-inbox-search"
          onChange={next => onSearchChange(next)}
          placeholder="Search inbox..."
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
              <ListFilter aria-hidden="true" className="size-3" />
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
        {isLoading && !inbox ? (
          <TaskRowsLoadingSkeleton label="Loading inbox" testId="tasks-inbox-loading" />
        ) : errorMessage && !inbox ? (
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3">
            <Empty
              data-testid="tasks-inbox-error"
              description={errorMessage}
              icon={AlertCircle}
              title="Unable to load inbox"
            />
            {onRetryQuery ? (
              <Button onClick={onRetryQuery} size="sm" type="button" variant="ghost">
                Retry loading inbox
              </Button>
            ) : null}
          </div>
        ) : !hasItems ? (
          <Empty
            className="mx-auto max-w-xl"
            data-testid="tasks-inbox-empty"
            description="Approval requests, failed runs, blockers, and archived items will appear here as work progresses."
            icon={Search}
            title="Nothing is waiting in the inbox"
          />
        ) : (
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
        )}
        {errorMessage && inbox ? (
          <div
            className="flex items-center justify-between gap-3 border-t border-line-soft pt-3 text-caption text-danger"
            data-testid="tasks-inbox-pagination-error"
            role="alert"
          >
            <span>{errorMessage}</span>
            {onRetryQuery ? (
              <Button onClick={onRetryQuery} size="sm" type="button" variant="ghost">
                Retry loading inbox
              </Button>
            ) : null}
          </div>
        ) : null}
        {hasMore && onLoadMore && !errorMessage ? (
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
              {isLoadingMore ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {isLoadingMore ? "Loading more" : "Load more"}
            </Button>
          </div>
        ) : null}
      </div>
    </ListingPage>
  );
}

interface GroupSectionProps {
  group: InboxGroupDefinition;
  items: TaskInboxItem[];
  itemActionProps: Omit<TasksInboxItemProps, "item" | "group">;
  totalCount: number;
}

function GroupSection({ group, items, itemActionProps, totalCount }: GroupSectionProps) {
  const countLabel =
    items.length === totalCount ? `${items.length}` : `${items.length} of ${totalCount}`;
  return (
    <section className="flex flex-col gap-2" data-testid={`tasks-inbox-group-${group.id}`}>
      <header className="flex items-center gap-2">
        <StatusDot
          data-testid={`tasks-inbox-group-dot-${group.id}`}
          label={group.label}
          tone={group.dotTone}
          variant={group.dotVariant}
        />
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
