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
  type PillTone,
  SearchInput,
  Skeleton,
  SkeletonRows,
  Spinner,
  Time,
} from "@agh/ui";

import {
  knowledgeAgentTierShortLabel,
  knowledgeMemoryKey,
  knowledgeScopeShortLabel,
  memoryScopeTone,
} from "../lib/knowledge-formatters";
import { groupKnowledgeMemoriesByScope } from "../lib/knowledge-list";
import {
  KNOWLEDGE_TYPE_TONE,
  type KnowledgeTypeTone,
  knowledgeTypeFor,
} from "../lib/knowledge-type-tone";
import type { KnowledgeMemoryItem } from "../types";
import { pillToneFromKnowledgeTone } from "./knowledge-pill-tone";

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

function pillToneFromKnowledgeType(tone: KnowledgeTypeTone): PillTone {
  return tone === "faint" ? "neutral" : tone;
}

function KnowledgeListItem({ memory, isSelected, onSelect }: KnowledgeListItemProps) {
  const memoryKey = knowledgeMemoryKey(memory);
  const scope = memory.scope;
  const knowledgeType = knowledgeTypeFor(memory.type);
  const typeTone = pillToneFromKnowledgeType(KNOWLEDGE_TYPE_TONE[knowledgeType]);
  return (
    <Item
      as="button"
      className="rounded-none border-x-0 border-t-0 border-b border-line px-4 py-3"
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
      <ItemFooter className="justify-start gap-1.5">
        <Pill
          mono
          data-testid={`type-badge-${memory.type}`}
          data-knowledge-type={knowledgeType}
          tone={typeTone}
        >
          {memory.type}
        </Pill>
        <Pill
          mono
          data-testid={`scope-badge-${scope}`}
          tone={pillToneFromKnowledgeTone(memoryScopeTone(scope))}
        >
          {knowledgeScopeShortLabel(scope)}
        </Pill>
        {memory.scope === "agent" && memory.agent_tier ? (
          <Pill mono data-testid={`agent-tier-badge-${memory.agent_tier}`} tone="warning">
            {knowledgeAgentTierShortLabel(memory.agent_tier)}
          </Pill>
        ) : null}
        {memory.agent_name ? (
          <Pill mono data-testid="agent-name-badge" tone="neutral">
            {memory.agent_name}
          </Pill>
        ) : null}
        {memory.recall_count > 0 ? (
          <Pill mono data-testid="recall-count-badge" tone="info">
            ↻ {memory.recall_count}
          </Pill>
        ) : null}
        {memory.staleness_banner ? (
          <Pill mono data-testid="staleness-badge" tone="warning">
            stale
          </Pill>
        ) : null}
        {memory.system_managed ? (
          <Pill mono data-testid="system-managed-badge" tone="neutral">
            system
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
  const resolvedTotalCount = totalCount === undefined ? memories.length : totalCount;
  const groups = groupKnowledgeMemoriesByScope(memories);
  const isEmpty = memories.length === 0;

  return (
    <aside className="flex min-h-0 flex-1 flex-col" data-testid="knowledge-list-panel">
      <div className="border-b border-line p-3">
        <SearchInput
          aria-label="Recall knowledge"
          data-testid="knowledge-search-input"
          onChange={onSearchChange}
          placeholder="Recall knowledge..."
          value={searchQuery}
        />
        {searchInfo ? (
          <Eyebrow className="text-subtle mt-2 block" data-testid="knowledge-search-info">
            {searchInfo}
          </Eyebrow>
        ) : null}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {isLoading && isEmpty ? (
          <div aria-busy="true" data-testid="knowledge-list-loading" role="status">
            <SkeletonRows className="min-h-full" count={6} rowClassName="border-b border-line p-4">
              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between gap-3">
                  <Skeleton className="h-3.5 w-2/5" />
                  <Skeleton className="h-3 w-16" />
                </div>
                <Skeleton className="h-3 w-3/4" />
                <div className="flex gap-1.5">
                  <Skeleton className="h-5 w-14" />
                  <Skeleton className="h-5 w-20" />
                </div>
              </div>
            </SkeletonRows>
            <span className="sr-only">Loading knowledge</span>
          </div>
        ) : errorMessage && isEmpty ? (
          <div
            className="flex min-h-full items-center justify-center p-4"
            data-testid="knowledge-list-error"
          >
            <Empty
              className="max-w-sm"
              description={errorMessage}
              icon={AlertCircle}
              title="Unable to load knowledge"
            />
          </div>
        ) : isEmpty ? (
          <div
            className="flex min-h-full items-center justify-center p-4"
            data-testid="knowledge-list-empty"
          >
            <Empty
              className="max-w-sm"
              description={
                searchMode
                  ? "No memories matched this recall query."
                  : searchQuery.trim() !== ""
                    ? "Try a different search term or adjust the scope filter."
                    : "No knowledge items found"
              }
              icon={BookOpen}
              title="No knowledge items found"
            />
          </div>
        ) : (
          <div data-testid="knowledge-list-groups">
            {groups.map(group => (
              <ListGroup
                count={group.memories.length}
                data-testid={`knowledge-group-${group.scope}`}
                headerProps={{ "data-testid": `knowledge-group-header-${group.scope}` }}
                key={group.scope}
                label={group.label}
              >
                {group.memories.map(memory => (
                  <KnowledgeListItem
                    isSelected={knowledgeMemoryKey(memory) === selectedMemoryKey}
                    key={knowledgeMemoryKey(memory)}
                    memory={memory}
                    onSelect={() => onSelectMemory(knowledgeMemoryKey(memory))}
                  />
                ))}
              </ListGroup>
            ))}
            {errorMessage ? (
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
            ) : null}
            {!searchMode && !errorMessage && hasMore && onLoadMore ? (
              <div className="flex items-center justify-between gap-3 border-t border-line px-4 py-3">
                <span className="text-xs tabular-nums text-subtle">
                  {memories.length} of {resolvedTotalCount}
                </span>
                <Button
                  aria-busy={isLoadingMore}
                  aria-label={isLoadingMore ? "Loading more knowledge" : "Load more knowledge"}
                  disabled={isLoadingMore}
                  onClick={onLoadMore}
                  size="sm"
                  type="button"
                  variant="neutral"
                >
                  {isLoadingMore ? <Spinner aria-hidden="true" className="size-3" /> : null}
                  {isLoadingMore ? "Loading more knowledge" : "Load more knowledge"}
                </Button>
              </div>
            ) : null}
          </div>
        )}
      </div>
    </aside>
  );
}

export { KnowledgeListPanel };
export type { KnowledgeListPanelProps };
