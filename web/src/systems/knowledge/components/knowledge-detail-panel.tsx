import { AlertCircle, BookOpen, Code, Pencil, Trash2 } from "lucide-react";
import { type Dispatch, type SetStateAction, useState } from "react";

import {
  Button,
  cn,
  CodeBlock,
  ContextBox,
  type ContextBoxEntry,
  Empty,
  MonoId,
  PAGE_CONTENT_GUTTER,
  Section,
  Skeleton,
  StreamMarkdown,
  Time,
} from "@compozy/ui";

import {
  knowledgeAgentTierLabel,
  knowledgeScopeLabel,
  knowledgeTypeLabel,
} from "@/systems/knowledge/lib/knowledge-formatters";
import type {
  KnowledgeMemoryItem,
  KnowledgeScope,
  MemoryDecision,
} from "@/systems/knowledge/types";

import { KnowledgeDecisionsSection } from "./knowledge-decisions-section";
import { KnowledgeDeleteDialog } from "./knowledge-delete-dialog";
import { KnowledgeEditDialog } from "./knowledge-edit-dialog";
import { KnowledgeFold } from "./knowledge-fold";

interface KnowledgeDetailPanelProps {
  memory: KnowledgeMemoryItem | undefined;
  content: string | undefined;
  scope?: KnowledgeScope;
  /** Display name of the memory's project; the raw id is the fallback. */
  projectName?: string;
  status: {
    isLoading: boolean;
    isDeletePending: boolean;
    isEditPending?: boolean;
    isDecisionsLoading?: boolean;
  };
  error: Error | null;
  onDelete: (memory: KnowledgeMemoryItem) => Promise<void>;
  deleteError?: string | null;
  onEdit?: (
    memory: KnowledgeMemoryItem,
    input: { content: string; description?: string }
  ) => Promise<void>;
  editError?: string | null;
  decisions?: MemoryDecision[];
  decisionsError?: Error | null;
  onRevertDecision?: (decision: MemoryDecision) => Promise<void>;
  revertingDecisionId?: string | null;
  revertError?: string | null;
}

type KnowledgeDialogKey = "confirmDeleteOpen" | "editOpen";

interface KnowledgeDialogState {
  memoryIdentity: string;
  confirmDeleteOpen: boolean;
  editOpen: boolean;
}

function setKnowledgeDialogOpen(
  setDialogState: Dispatch<SetStateAction<KnowledgeDialogState>>,
  memoryIdentity: string,
  key: KnowledgeDialogKey,
  open: boolean
) {
  setDialogState(previous => ({ ...previous, memoryIdentity, [key]: open }));
}

function knowledgeDialogMemoryIdentity(memory: KnowledgeMemoryItem | undefined): string {
  if (!memory) return "";
  if (memory.key) return memory.key;

  return [
    memory.scope,
    memory.workspace_id ?? "",
    memory.agent_name ?? "",
    memory.agent_tier ?? "",
    memory.filename,
  ].join(":");
}

function buildPrimaryEntries(memory: KnowledgeMemoryItem): ContextBoxEntry[] {
  const entries: ContextBoxEntry[] = [
    {
      label: "Kind",
      value: (
        <span className="text-fg" data-testid="context-type-value">
          {knowledgeTypeLabel(memory.type)}
        </span>
      ),
    },
    {
      label: "Updated",
      value: (
        <Time className="text-muted" data-testid="context-modified-value" iso={memory.mod_time} />
      ),
    },
    {
      label: "Used",
      value: (
        <span className="text-muted" data-testid="context-recalls-value">
          {memory.recall_count === 0
            ? "Not used yet"
            : `${memory.recall_count} ${memory.recall_count === 1 ? "time" : "times"}`}
          {memory.last_recalled_at ? (
            <>
              {" · last "}
              <Time data-testid="context-last-recalled-value" iso={memory.last_recalled_at} />
            </>
          ) : null}
        </span>
      ),
    },
  ];
  if (memory.staleness_banner) {
    entries.push({
      label: "Status",
      value: (
        <span className="text-warning" data-testid="context-staleness-value">
          {memory.staleness_banner}
        </span>
      ),
    });
  }
  return entries;
}

function buildDetailEntries(
  memory: KnowledgeMemoryItem,
  scope: KnowledgeScope,
  projectName: string | undefined
): ContextBoxEntry[] {
  const entries: ContextBoxEntry[] = [
    {
      label: "File",
      value: (
        <MonoId data-testid="knowledge-detail-filename" preserveCase value={memory.filename} />
      ),
    },
    {
      label: "Saved in",
      value: (
        <span className="text-muted" data-testid="context-scope-value">
          {memory.scope === "agent" && memory.agent_tier
            ? knowledgeAgentTierLabel(memory.agent_tier)
            : knowledgeScopeLabel(scope)}
        </span>
      ),
    },
  ];
  if (memory.agent_name) {
    entries.push({
      label: "Agent",
      value: (
        <span className="text-fg" data-testid="context-agent-value">
          {memory.agent_name}
        </span>
      ),
    });
  }
  if (memory.workspace_id) {
    entries.push({
      label: "Project",
      value: projectName ? (
        <span className="text-muted" data-testid="context-workspace-value">
          {projectName}
        </span>
      ) : (
        <MonoId data-testid="context-workspace-value" preserveCase value={memory.workspace_id} />
      ),
    });
  }
  if (memory.superseded_by) {
    entries.push({
      label: "Replaced by",
      value: (
        <MonoId data-testid="context-superseded-value" preserveCase value={memory.superseded_by} />
      ),
    });
  }
  entries.push({
    label: "Loaded into every session",
    value: (
      <span className="text-muted" data-testid="context-injection-value">
        {memory.injection ? "Yes" : "No"}
      </span>
    ),
  });
  if (memory.system_managed) {
    entries.push({
      label: "Managed by",
      value: (
        <span className="text-muted" data-testid="context-system-managed-value">
          CompozyOS
        </span>
      ),
    });
  }
  return entries;
}

function KnowledgeDetailPanel({
  memory,
  content,
  scope,
  projectName,
  status,
  error,
  onDelete,
  deleteError,
  onEdit,
  editError,
  decisions,
  decisionsError = null,
  onRevertDecision,
  revertingDecisionId = null,
  revertError = null,
}: KnowledgeDetailPanelProps) {
  const { isLoading, isDeletePending, isEditPending = false, isDecisionsLoading = false } = status;
  const memoryIdentity = knowledgeDialogMemoryIdentity(memory);
  const [showSource, setShowSource] = useState(false);
  const [dialogState, setDialogState] = useState<KnowledgeDialogState>({
    memoryIdentity,
    confirmDeleteOpen: false,
    editOpen: false,
  });

  if (dialogState.memoryIdentity !== memoryIdentity) {
    setDialogState({ memoryIdentity, confirmDeleteOpen: false, editOpen: false });
  }

  const isCurrentDialogState = dialogState.memoryIdentity === memoryIdentity;
  const confirmDeleteOpen = isCurrentDialogState && dialogState.confirmDeleteOpen;
  const editOpen = isCurrentDialogState && dialogState.editOpen;

  if (isLoading) {
    return <KnowledgeDetailSkeleton />;
  }

  if (error) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="knowledge-detail-error"
      >
        <Empty
          className="max-w-md"
          description={error.message ?? "Failed to load memory details"}
          icon={AlertCircle}
          title="Failed to load memory details"
        />
      </div>
    );
  }

  if (!memory) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="knowledge-detail-empty"
      >
        <Empty className="max-w-md" icon={BookOpen} title="Select a memory to view details" />
      </div>
    );
  }

  const resolvedScope: KnowledgeScope = scope ?? memory.scope;

  const handleConfirmDelete = async () => {
    try {
      await onDelete(memory);
      setKnowledgeDialogOpen(setDialogState, memoryIdentity, "confirmDeleteOpen", false);
    } catch {
      // Error state is surfaced through `deleteError` and the dialog stays open.
    }
  };

  const handleConfirmEdit = async (input: { content: string; description?: string }) => {
    if (!onEdit) return;
    try {
      await onEdit(memory, input);
      setKnowledgeDialogOpen(setDialogState, memoryIdentity, "editOpen", false);
    } catch {
      // Error state is surfaced through `editError` and the dialog stays open.
    }
  };

  return (
    <div
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-y-auto")}
      data-testid="knowledge-detail-panel"
    >
      <header
        className="flex flex-wrap items-start justify-between gap-3 pt-4"
        data-testid="knowledge-detail-header"
      >
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <h2 className="text-item-title font-medium text-fg-strong">{memory.name}</h2>
          {memory.description ? (
            <p
              className="text-small-body leading-relaxed text-muted"
              data-testid="knowledge-detail-description"
            >
              {memory.description}
            </p>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {onEdit ? (
            <Button
              data-testid="edit-memory-btn"
              disabled={isEditPending || content === undefined}
              onClick={() =>
                setKnowledgeDialogOpen(setDialogState, memoryIdentity, "editOpen", true)
              }
              size="sm"
              type="button"
              variant="outline"
            >
              <Pencil className="size-3" />
              Edit
            </Button>
          ) : null}
          <Button
            data-testid="delete-memory-btn"
            disabled={isDeletePending}
            onClick={() =>
              setKnowledgeDialogOpen(setDialogState, memoryIdentity, "confirmDeleteOpen", true)
            }
            size="sm"
            type="button"
            variant="ghost"
          >
            <Trash2 className="size-3" />
            Delete
          </Button>
        </div>
      </header>

      <div className="flex flex-col gap-6 py-5">
        <ContextBox data-testid="knowledge-detail-context" entries={buildPrimaryEntries(memory)} />

        {content ? (
          <Section
            label="Content"
            right={
              <Button
                aria-pressed={showSource}
                data-testid="knowledge-content-source-toggle"
                onClick={() => setShowSource(previous => !previous)}
                size="xs"
                type="button"
                variant="ghost"
              >
                <Code className="size-3" />
                {showSource ? "View formatted" : "View source"}
              </Button>
            }
          >
            {showSource ? (
              <CodeBlock code={content} copyable data-testid="content-source" />
            ) : (
              <div data-testid="content-preview">
                <StreamMarkdown compact>{content}</StreamMarkdown>
              </div>
            )}
          </Section>
        ) : null}

        <KnowledgeFold
          data-testid="knowledge-detail-more"
          label="Details"
          toggleTestId="knowledge-detail-more-toggle"
        >
          <ContextBox
            data-testid="knowledge-detail-facts"
            entries={buildDetailEntries(memory, resolvedScope, projectName)}
          />
        </KnowledgeFold>

        <KnowledgeDecisionsSection
          decisions={decisions}
          error={decisionsError}
          isLoading={isDecisionsLoading}
          onRevertDecision={onRevertDecision}
          revertError={revertError}
          revertingDecisionId={revertingDecisionId}
        />
      </div>

      <KnowledgeDeleteDialog
        error={deleteError}
        isPending={isDeletePending}
        name={memory.name}
        onConfirm={handleConfirmDelete}
        onOpenChange={open =>
          setKnowledgeDialogOpen(setDialogState, memoryIdentity, "confirmDeleteOpen", open)
        }
        open={confirmDeleteOpen}
        scope={resolvedScope}
      />

      {onEdit ? (
        <KnowledgeEditDialog
          error={editError}
          filename={memory.filename}
          initialContent={content ?? ""}
          initialDescription={memory.description ?? ""}
          isPending={isEditPending}
          name={memory.name}
          onConfirm={handleConfirmEdit}
          onOpenChange={open =>
            setKnowledgeDialogOpen(setDialogState, memoryIdentity, "editOpen", open)
          }
          open={editOpen}
          type={memory.type}
        />
      ) : null}
    </div>
  );
}

function KnowledgeDetailSkeleton() {
  return (
    <div
      aria-busy="true"
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden")}
      data-testid="knowledge-detail-loading"
      role="status"
    >
      <div className="flex flex-col gap-2 pt-4">
        <Skeleton className="h-4 w-48" />
        <Skeleton className="h-3 w-3/4" />
      </div>
      <div className="flex flex-col gap-6 py-5">
        <Skeleton className="h-20 w-full" />
        <div className="space-y-2.5">
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-48 w-full" />
        </div>
      </div>
      <span className="sr-only">Loading knowledge details</span>
    </div>
  );
}

export { KnowledgeDetailPanel };
export type { KnowledgeDetailPanelProps };
