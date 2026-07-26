import { AlertCircle, BookOpen, Pencil, Trash2 } from "lucide-react";
import { type Dispatch, type SetStateAction, useState } from "react";

import {
  Button,
  cn,
  CodeBlock,
  ContextBox,
  type ContextBoxEntry,
  Empty,
  PAGE_CONTENT_GUTTER,
  Pill,
  Section,
  Skeleton,
  StatusDot,
  Time,
} from "@agh/ui";

import {
  knowledgeAgentTierLabel,
  knowledgeScopeLabel,
  memoryScopeTone,
} from "@/systems/knowledge/lib/knowledge-formatters";
import { knowledgeTypeFor } from "@/systems/knowledge/lib/knowledge-type-tone";
import type {
  KnowledgeMemoryItem,
  KnowledgeScope,
  MemoryDecision,
} from "@/systems/knowledge/types";

import { KnowledgeDecisionsSection } from "./knowledge-decisions-section";
import { KnowledgeDeleteDialog } from "./knowledge-delete-dialog";
import { KnowledgeEditDialog } from "./knowledge-edit-dialog";
import { pillToneFromKnowledgeTone } from "./knowledge-pill-tone";

interface KnowledgeDetailPanelProps {
  memory: KnowledgeMemoryItem | undefined;
  content: string | undefined;
  scope?: KnowledgeScope;
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

function statusDotToneFromScope(scope: KnowledgeScope): "warning" | "accent" | "faint" {
  if (scope === "agent") return "warning";
  if (scope === "workspace") return "accent";
  return "faint";
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

function buildContextEntries(memory: KnowledgeMemoryItem): ContextBoxEntry[] {
  const knowledgeType = knowledgeTypeFor(memory.type);
  const entries: ContextBoxEntry[] = [
    {
      label: "Type",
      value: (
        <span className="font-mono text-fg" data-testid="context-type-value">
          {memory.type}
        </span>
      ),
    },
    {
      label: "Knowledge tier",
      value: (
        <span className="font-mono text-muted" data-testid="context-tier-value">
          {knowledgeType}
        </span>
      ),
    },
    {
      label: "Staleness",
      value: (
        <span data-testid="context-staleness-value">{memory.staleness_banner ?? "Active"}</span>
      ),
    },
  ];

  if (memory.scope === "agent" && memory.agent_tier) {
    entries.push({
      label: "Agent tier",
      value: (
        <span className="font-mono text-muted" data-testid="context-agent-tier-value">
          {knowledgeAgentTierLabel(memory.agent_tier)}
        </span>
      ),
    });
  }
  if (memory.agent_name) {
    entries.push({
      label: "Agent",
      value: (
        <span className="font-mono text-fg" data-testid="context-agent-value">
          {memory.agent_name}
        </span>
      ),
    });
  }
  if (memory.workspace_id) {
    entries.push({
      label: "Workspace",
      value: (
        <span className="font-mono text-muted" data-testid="context-workspace-value">
          {memory.workspace_id}
        </span>
      ),
    });
  }
  entries.push({
    label: "Modified",
    value: (
      <Time
        className="text-muted"
        data-testid="context-modified-value"
        iso={memory.mod_time}
        mode="absolute"
      />
    ),
  });
  entries.push({
    label: "Recalls",
    value: (
      <span className="font-mono text-fg" data-testid="context-recalls-value">
        {memory.recall_count}
      </span>
    ),
  });
  if (memory.last_recalled_at) {
    entries.push({
      label: "Last recalled",
      value: (
        <Time
          className="text-muted"
          data-testid="context-last-recalled-value"
          iso={memory.last_recalled_at}
          mode="absolute"
        />
      ),
    });
  }
  if (memory.superseded_by) {
    entries.push({
      label: "Superseded by",
      value: (
        <span className="font-mono text-fg" data-testid="context-superseded-value">
          {memory.superseded_by}
        </span>
      ),
    });
  }
  entries.push({
    label: "Injection",
    value: (
      <span className="font-mono text-muted" data-testid="context-injection-value">
        {memory.injection ? "true" : "false"}
      </span>
    ),
  });
  if (memory.system_managed) {
    entries.push({
      label: "System managed",
      value: (
        <span className="font-mono text-muted" data-testid="context-system-managed-value">
          true
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
  const scopeTone = pillToneFromKnowledgeTone(memoryScopeTone(resolvedScope));
  const contextEntries = buildContextEntries(memory);

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
      <div className="flex flex-col gap-2 pt-4" data-testid="knowledge-detail-header">
        <span
          className="lowercase text-small-body text-subtle"
          data-testid="knowledge-detail-filename"
        >
          {memory.filename}
        </span>
        <div className="flex flex-wrap items-center gap-1.5">
          <Pill mono data-testid="detail-scope-badge" tone={scopeTone}>
            <StatusDot
              aria-hidden="true"
              className="-ml-0.5"
              tone={statusDotToneFromScope(resolvedScope)}
            />
            {knowledgeScopeLabel(resolvedScope)}
          </Pill>
          <Pill mono data-testid="detail-age-badge" tone="neutral">
            <Time iso={memory.mod_time} />
          </Pill>
        </div>
      </div>

      <div className="flex flex-col gap-6 py-5">
        <ContextBox
          data-testid="knowledge-detail-context"
          entries={contextEntries}
          title="Overview"
        />

        {memory.description ? (
          <Section label="Description">
            <p className="text-small-body leading-relaxed text-muted">{memory.description}</p>
          </Section>
        ) : null}

        {content ? (
          <Section label="Content">
            <CodeBlock code={content} copyable data-testid="content-preview" showPrompt={false} />
          </Section>
        ) : null}

        <KnowledgeDecisionsSection
          decisions={decisions}
          error={decisionsError}
          isLoading={isDecisionsLoading}
          onRevertDecision={onRevertDecision}
          revertError={revertError}
          revertingDecisionId={revertingDecisionId}
        />
      </div>

      <footer className="mt-auto flex flex-wrap items-center gap-2 border-t border-line py-4">
        {onEdit ? (
          <Button
            data-testid="edit-memory-btn"
            disabled={isEditPending || content === undefined}
            onClick={() => setKnowledgeDialogOpen(setDialogState, memoryIdentity, "editOpen", true)}
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
          variant="outline"
        >
          <Trash2 className="size-3" />
          Delete
        </Button>
        {deleteError ? (
          <span className="text-xs text-danger" data-testid="knowledge-delete-error">
            {deleteError}
          </span>
        ) : null}
        {editError ? (
          <span className="text-xs text-danger" data-testid="knowledge-edit-error">
            {editError}
          </span>
        ) : null}
      </footer>

      <KnowledgeDeleteDialog
        error={deleteError}
        filename={memory.filename}
        isPending={isDeletePending}
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
          scope={resolvedScope}
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
        <Skeleton className="h-3 w-48" />
        <div className="flex gap-2">
          <Skeleton className="h-5 w-24" />
          <Skeleton className="h-5 w-20" />
        </div>
      </div>
      <div className="flex flex-col gap-6 py-5">
        <Skeleton className="h-44 w-full" />
        <div className="space-y-2.5">
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-20 w-full" />
        </div>
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
