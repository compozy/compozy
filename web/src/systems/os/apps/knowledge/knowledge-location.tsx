import { AlertCircle, BookOpen, Plus } from "lucide-react";

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
} from "@compozy/ui";
import { useKnowledgePage } from "./use-knowledge-page";
import {
  type KnowledgeAgentTier,
  KnowledgeCreateDialog,
  KnowledgeDetailPanel,
  KnowledgeListPanel,
  type KnowledgeScope,
} from "@/systems/knowledge";

import { useDesktop } from "../../hooks/use-desktop";
import { knowledgeRouteOptions } from "./knowledge-route-selection";

const EMPTY_SEARCH: Record<string, unknown> = {};

type KnowledgePageModel = ReturnType<typeof useKnowledgePage>;

export function KnowledgeLocation({ windowId }: { windowId: string }) {
  const search = useDesktop(state => state.windows[windowId]?.route.search ?? EMPTY_SEARCH);
  const page = useKnowledgePage(knowledgeRouteOptions(search));
  // Master–detail: the list stays on screen while a memory is selected, so the
  // head keeps the root identity and the detail pane titles the memory.
  useTopbarSlot({
    glyph: <BookOpen />,
    crumb: "Knowledge",
    actions: <KnowledgeCreateButton page={page} />,
    toolbar: <KnowledgeScopeToolbar page={page} />,
  });

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden" data-testid="knowledge-shell">
      <KnowledgeLocationBody page={page} />
    </div>
  );
}

function KnowledgeCreateButton({ page }: { page: KnowledgePageModel }) {
  return (
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
}

function KnowledgeScopeToolbar({ page }: { page: KnowledgePageModel }) {
  return (
    <ListingToolbar>
      <ListingToolbar.Leading>
        <PillGroup<KnowledgeScope>
          aria-label="Knowledge scope"
          data-testid="tab-pills"
          items={[
            { value: "profile", label: "Profile", testId: "tab-profile" },
            { value: "workspace", label: "Project", testId: "tab-workspace" },
            { value: "agent", label: "Agent", testId: "tab-agent" },
          ]}
          onChange={page.setActiveScope}
          value={page.activeScope}
        />
        {page.activeScope === "agent" ? <KnowledgeAgentControls page={page} /> : null}
      </ListingToolbar.Leading>
    </ListingToolbar>
  );
}

function KnowledgeAgentControls({ page }: { page: KnowledgePageModel }) {
  return (
    <div className="flex items-center gap-2" data-testid="agent-scope-controls">
      <Input
        aria-label="Agent name"
        className="h-7 w-44"
        data-testid="agent-name-input"
        onChange={event => page.setAgentName(event.target.value)}
        placeholder="Agent name"
        value={page.agentName}
      />
      <PillGroup<KnowledgeAgentTier>
        aria-label="Agent tier"
        data-testid="agent-tier-pills"
        items={[
          { value: "workspace", label: "This project", testId: "tier-workspace" },
          { value: "global", label: "All projects", testId: "tier-global" },
        ]}
        onChange={page.setAgentTier}
        value={page.agentTier}
      />
    </div>
  );
}

/** Guard, then first-load skeleton, then a blocking error, then the split view. */
function KnowledgeLocationBody({ page }: { page: KnowledgePageModel }) {
  if (page.guard) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="knowledge-guard"
      >
        <Empty
          className="max-w-md"
          description={page.guard.description}
          icon={BookOpen}
          title={page.guard.title}
        />
      </div>
    );
  }

  if (page.isLoading) {
    return (
      <SplitPane
        data-testid="knowledge-loading"
        detail={
          <div className="space-y-4 p-5">
            <Skeleton className="h-5 w-48" />
            <Skeleton className="h-3 w-3/4" />
            <Skeleton className="h-28 w-full" />
          </div>
        }
        list={
          <SkeletonRows className="p-4" count={6} rowClassName="border-b border-line-soft py-3" />
        }
      />
    );
  }

  if (page.error) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="knowledge-error"
      >
        <Empty
          action={
            <Button onClick={page.retryKnowledgeList} size="sm" type="button" variant="ghost">
              Retry loading knowledge
            </Button>
          }
          className="max-w-md"
          description={page.error.message ?? "Try again in a moment."}
          icon={AlertCircle}
          title="Couldn't load knowledge"
        />
      </div>
    );
  }

  return <KnowledgeSplitView page={page} />;
}

function KnowledgeSplitView({ page }: { page: KnowledgePageModel }) {
  return (
    <>
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
            projectName={page.selectedProjectName}
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
        scope={page.createScope}
        destinationLabel={page.createDestinationLabel}
      />
    </>
  );
}
