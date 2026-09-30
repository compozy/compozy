import { AlertCircle, BookOpen } from "lucide-react";

import {
  Empty,
  Button,
  Eyebrow,
  Item,
  ItemDescription,
  ItemFooter,
  ItemHeader,
  ItemTitle,
  ListGroup,
  Pill,
  SearchInput,
  Skeleton,
  SkeletonRows,
  Spinner,
  Time,
} from "@compozy/ui";

import { knowledgeMemoryKey, knowledgeTypeLabel } from "../lib/knowledge-formatters";
import { groupKnowledgeMemoriesByScope } from "../lib/knowledge-list";
import { knowledgeTypeFor } from "../lib/knowledge-type-tone";
import type { KnowledgeMemoryItem } from "../types";

interface KnowledgeListPanelProps {
  memories: KnowledgeMemoryItem[];
  selectedMemoryKey: string | null;
  onSelectMemory: (memoryKey: string) => void;
  searchQuery: string;
  onSearchChange: (query: string) => void;
  isLoading?: boolean;
  errorMessage?: string | null;
  searchMode?: boolean;
  searchInfo?: string | null;
  totalCount?: number;
  hasMore?: boolean;
  isLoadingMore?: boolean;
  onLoadMore?: () => void;
  onRetry?: () => void;
}

interface KnowledgeListItemProps {
  memory: KnowledgeMemoryItem;
  isSelected: boolean;
  onSelect: () => void;
}

function KnowledgeListItem({ memory, isSelected, onSelect }: KnowledgeListItemProps) {
  const memoryKey = knowledgeMemoryKey(memory);
  return (
    <Item
      as="button"
      className="rounded-none border-x-0 border-t-0 border-b border-line-soft px-4 py-3"
      data-state={isSelected ? "selected" : undefined}
      data-testid={`memory-item-${memoryKey}`}
      indicator={isSelected ? "rail" : "none"}
      onClick={onSelect}
      selectable
      selected={isSelected}
    >
      <ItemHeader>
        <ItemTitle className="min-w-0 flex-1 text-small-body text-fg">{memory.name}</ItemTitle>
        <Eyebrow className="text-subtle shrink-0">
          <Time iso={memory.mod_time} />
        </Eyebrow>
      </ItemHeader>
      {memory.description ? (
        <ItemDescription className="basis-full truncate text-xs text-muted">
          {memory.description}
        </ItemDescription>
      ) : null}
      <ItemFooter className="justify-start gap-2">
        <span
          className="text-xs text-subtle"
          data-knowledge-type={knowledgeTypeFor(memory.type)}
          data-testid={`type-badge-${memory.type}`}
        >
          {knowledgeTypeLabel(memory.type)}
        </span>
        {memory.staleness_banner ? (
          <Pill data-testid="staleness-badge" size="xs" tone="warning">
            Outdated
          </Pill>
        ) : null}
      </ItemFooter>
    </Item>
  );
}

function KnowledgeListPanel({
  memories,
  selectedMemoryKey,
  onSelectMemory,
  searchQuery,
  onSearchChange,
  isLoading = false,
  errorMessage = null,
  searchMode = false,
  searchInfo = null,
  totalCount,
  hasMore = false,
  isLoadingMore = false,
  onLoadMore,
  onRetry,
}: KnowledgeListPanelProps) {
  const isEmpty = memories.length === 0;

  return (
    <aside className="flex min-h-0 flex-1 flex-col" data-testid="knowledge-list-panel">
      <div className="border-b border-line p-3">
        <SearchInput
          aria-label="Search knowledge"
          data-testid="knowledge-search-input"
          onChange={onSearchChange}
          placeholder="Search knowledge"
          value={searchQuery}
        />
        {searchInfo ? (
          <Eyebrow className="text-subtle mt-2 block" data-testid="knowledge-search-info">
            {searchInfo}
          </Eyebrow>
        ) : null}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {isEmpty ? (
          <KnowledgeListEmptyState
            errorMessage={errorMessage}
            isLoading={isLoading}
            searching={searchMode || searchQuery.trim() !== ""}
          />
        ) : (
          <div data-testid="knowledge-list-groups">
            <KnowledgeListGroups
              memories={memories}
              onSelectMemory={onSelectMemory}
              selectedMemoryKey={selectedMemoryKey}
            />
            {errorMessage ? (
              <KnowledgeListRetryRow errorMessage={errorMessage} onRetry={onRetry} />
            ) : null}
            {!searchMode && !errorMessage && hasMore && onLoadMore ? (
              <KnowledgeListLoadMore
                isLoadingMore={isLoadingMore}
                loadedCount={memories.length}
                onLoadMore={onLoadMore}
                totalCount={totalCount ?? memories.length}
              />
            ) : null}
          </div>
        )}
      </div>
    </aside>
  );
}

interface KnowledgeListEmptyStateProps {
  isLoading: boolean;
  errorMessage: string | null;
  searching: boolean;
}

/** No rows yet: first-load skeleton, then a blocking error, then the empty copy. */
function KnowledgeListEmptyState({
  isLoading,
  errorMessage,
  searching,
}: KnowledgeListEmptyStateProps) {
  if (isLoading) {
    return (
      <div aria-busy="true" data-testid="knowledge-list-loading" role="status">
        <SkeletonRows className="min-h-full" count={6} rowClassName="border-b border-line p-4">
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between gap-3">
              <Skeleton className="h-3.5 w-2/5" />
              <Skeleton className="h-3 w-16" />
            </div>
            <Skeleton className="h-3 w-3/4" />
            <Skeleton className="h-3 w-20" />
          </div>
        </SkeletonRows>
        <span className="sr-only">Loading knowledge</span>
      </div>
    );
  }
  if (errorMessage) {
    return (
      <div
        className="flex min-h-full items-center justify-center p-4"
        data-testid="knowledge-list-error"
      >
        <Empty
          className="max-w-sm"
          description={errorMessage}
          icon={AlertCircle}
          title="Couldn't load knowledge"
        />
      </div>
    );
  }
  return (
    <div
      className="flex min-h-full items-center justify-center p-4"
      data-testid="knowledge-list-empty"
    >
      <Empty
        className="max-w-sm"
        description={
          searching
            ? "Try a different word."
            : "Agents save what they learn here. Use Create to add your own."
        }
        icon={BookOpen}
        title={searching ? "No matches" : "No knowledge yet"}
      />
    </div>
  );
}

interface KnowledgeListGroupsProps {
  memories: KnowledgeMemoryItem[];
  selectedMemoryKey: string | null;
  onSelectMemory: (memoryKey: string) => void;
}

/** A single scope renders flat; several scopes render under group headers. */
function KnowledgeListGroups({
  memories,
  selectedMemoryKey,
  onSelectMemory,
}: KnowledgeListGroupsProps) {
  const groups = groupKnowledgeMemoriesByScope(memories);
  const renderItem = (memory: KnowledgeMemoryItem) => (
    <KnowledgeListItem
      isSelected={knowledgeMemoryKey(memory) === selectedMemoryKey}
      key={knowledgeMemoryKey(memory)}
      memory={memory}
      onSelect={() => onSelectMemory(knowledgeMemoryKey(memory))}
    />
  );

  if (groups.length === 1) {
    return groups[0].memories.map(renderItem);
  }
  return groups.map(group => (
    <ListGroup
      count={group.memories.length}
      data-testid={`knowledge-group-${group.scope}`}
      headerProps={{ "data-testid": `knowledge-group-header-${group.scope}` }}
      key={group.scope}
      label={group.label}
    >
      {group.memories.map(renderItem)}
    </ListGroup>
  ));
}

function KnowledgeListRetryRow({
  errorMessage,
  onRetry,
}: {
  errorMessage: string;
  onRetry?: () => void;
}) {
  return (
    <div
      className="flex items-center justify-between gap-3 border-t border-line px-4 py-3 text-xs text-danger"
      data-testid="knowledge-list-pagination-error"
      role="alert"
    >
      <span>{errorMessage}</span>
      {onRetry ? (
        <Button onClick={onRetry} size="sm" type="button" variant="neutral">
          Retry loading knowledge
        </Button>
      ) : null}
    </div>
  );
}

interface KnowledgeListLoadMoreProps {
  loadedCount: number;
  totalCount: number;
  isLoadingMore: boolean;
  onLoadMore: () => void;
}

function KnowledgeListLoadMore({
  loadedCount,
  totalCount,
  isLoadingMore,
  onLoadMore,
}: KnowledgeListLoadMoreProps) {
  const label = isLoadingMore ? "Loading more knowledge" : "Load more knowledge";
  return (
    <div className="flex items-center justify-between gap-3 border-t border-line px-4 py-3">
      <span className="text-xs tabular-nums text-subtle">
        {loadedCount} of {totalCount}
      </span>
      <Button
        aria-busy={isLoadingMore}
        aria-label={label}
        disabled={isLoadingMore}
        onClick={onLoadMore}
        size="sm"
        type="button"
        variant="neutral"
      >
        {isLoadingMore ? <Spinner aria-hidden="true" className="size-3" /> : null}
        {label}
      </Button>
    </div>
  );
}

export { KnowledgeListPanel };
export type { KnowledgeListPanelProps };
