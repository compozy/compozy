import { AlertCircle, BookOpen, Plus } from "lucide-react";

import {
  KnowledgeCreateDialog,
  KnowledgeDetailPanel,
  KnowledgeListPanel,
  type KnowledgeAgentTier,
  type KnowledgeScope,
} from "@/systems/knowledge";
import {
  Button,
  Empty,
  Input,
  ListingToolbar,
  PillGroup,
  Skeleton,
  SkeletonRows,
  SplitPane,
  useTopbarSlot,
} from "@agh/ui";
import { useKnowledgePage } from "./use-knowledge-page";

export function KnowledgeLocation() {
  const page = useKnowledgePage();

  const scopePills = (
    <PillGroup<KnowledgeScope>
      aria-label="Knowledge scope"
      data-testid="tab-pills"
      items={[
        { value: "global", label: "Global", testId: "tab-global" },
        { value: "workspace", label: "Workspace", testId: "tab-workspace" },
        { value: "agent", label: "Agent", testId: "tab-agent" },
      ]}
      onChange={page.setActiveScope}
      value={page.activeScope}
    />
  );

  const agentControls =
    page.activeScope === "agent" ? (
      <div className="flex items-center gap-2" data-testid="agent-scope-controls">
        <Input
          aria-label="Agent name"
          className="h-7 w-44"
          data-testid="agent-name-input"
          onChange={event => page.setAgentName(event.target.value)}
          placeholder="agent name"
          value={page.agentName}
        />
        <PillGroup<KnowledgeAgentTier>
          aria-label="Agent tier"
          data-testid="agent-tier-pills"
          items={[
            { value: "workspace", label: "Workspace", testId: "tier-workspace" },
            { value: "global", label: "Global", testId: "tier-global" },
          ]}
          onChange={page.setAgentTier}
          value={page.agentTier}
        />
      </div>
    ) : null;

  const createBtn = (
    <Button
      data-testid="create-memory-btn"
      disabled={!page.canCreateMemory}
      onClick={() => page.setCreateOpen(true)}
      size="sm"
      type="button"
    >
      <Plus className="size-3" />
      Create
    </Button>
  );

  const selected = page.selectedMemory;
  const clearSelection = () => {
    page.setSelectedMemoryKey(null);
  };

  useTopbarSlot({
    glyph: selected ? undefined : <BookOpen />,
    crumb: selected ? (
      <span data-testid="knowledge-detail-title">{selected.name}</span>
    ) : (
      "Knowledge"
    ),
    onBack: selected ? () => clearSelection() : undefined,
    crumbs: selected
      ? [{ id: "knowledge", label: "Knowledge", onSelect: () => clearSelection() }]
      : undefined,
    actions: createBtn,
    toolbar: (
      <ListingToolbar>
        <ListingToolbar.Leading>
          {scopePills}
          {agentControls}
        </ListingToolbar.Leading>
      </ListingToolbar>
    ),
  });

  if (page.guardMessage) {
    return (
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden" data-testid="knowledge-shell">
        <div
          className="flex min-h-0 flex-1 items-center justify-center py-10"
          data-testid="knowledge-guard"
        >
          <Empty
            className="max-w-md"
            description={page.guardMessage}
            icon={BookOpen}
            title="Select scope inputs"
          />
        </div>
      </div>
    );
  }

  if (page.isLoading) {
    return (
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden" data-testid="knowledge-shell">
        <div className="grid min-h-0 flex-1 grid-cols-[18rem_1fr]" data-testid="knowledge-loading">
          <SkeletonRows
            className="border-r border-line p-4"
            count={6}
            rowClassName="border-b border-line-soft py-3"
          />
          <div className="space-y-4 p-5">
            <Skeleton className="h-5 w-48" />
            <Skeleton className="h-3 w-3/4" />
            <Skeleton className="h-28 w-full" />
          </div>
        </div>
      </div>
    );
  }

  if (page.error) {
    return (
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden" data-testid="knowledge-shell">
        <div
          className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 py-10"
          data-testid="knowledge-error"
        >
          <Empty
            className="max-w-md"
            description={page.error.message ?? "Failed to load knowledge"}
            icon={AlertCircle}
            title="Unable to load knowledge"
          />
          <Button onClick={page.retryKnowledgeList} size="sm" type="button" variant="ghost">
            Retry loading knowledge
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden" data-testid="knowledge-shell">
      <SplitPane
        data-testid="knowledge-split-pane"
        detail={
          <KnowledgeDetailPanel
            content={page.selectedContent}
            decisions={page.decisions}
            decisionsError={page.decisionsError}
            deleteError={page.deleteError}
            editError={page.editError}
            error={page.contentError}
            memory={page.selectedMemory}
            onDelete={page.handleDelete}
            onEdit={page.handleEdit}
            onRevertDecision={page.handleRevertDecision}
            revertError={page.revertError}
            revertingDecisionId={page.revertingDecisionId}
            scope={page.selectedScope}
            status={{
              isDecisionsLoading: page.isDecisionsLoading,
              isDeletePending: page.isDeletePending,
              isEditPending: page.isEditPending,
              isLoading: page.isContentLoading,
            }}
          />
        }
        list={
          <KnowledgeListPanel
            memories={page.memories}
            errorMessage={page.listRetryError?.message ?? null}
            hasMore={page.hasMoreMemories}
            isLoadingMore={page.isLoadingMoreMemories}
            onLoadMore={page.loadMoreMemories}
            onRetry={page.retryKnowledgeList}
            onSearchChange={page.setSearchQuery}
            onSelectMemory={page.setSelectedMemoryKey}
            searchInfo={page.searchInfo}
            searchMode={page.searchActive}
            searchQuery={page.searchQuery}
            selectedMemoryKey={page.effectiveSelectedMemoryKey}
            totalCount={page.memoryCount}
          />
        }
      />
      <KnowledgeCreateDialog
        defaultType={page.createDefaultType}
        error={page.createError}
        isPending={page.isCreatePending}
        onConfirm={page.handleCreate}
        onOpenChange={page.setCreateOpen}
        open={page.createOpen}
        scope={page.activeScope}
      />
    </div>
  );
}
