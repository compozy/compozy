import { useState } from "react";

import {
  cn,
  ContextBox,
  type ContextBoxEntry,
  Disclosure,
  MonoId,
  PAGE_CONTENT_GUTTER,
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

import { useKnowledgeDetailDialogs } from "../hooks/use-knowledge-detail-dialogs";

import { KnowledgeDecisionsSection } from "./knowledge-decisions-section";
import {
  KnowledgeContentSection,
  KnowledgeDetailDialogs,
  KnowledgeDetailEmpty,
  KnowledgeDetailError,
  KnowledgeDetailHeader,
  KnowledgeDetailSkeleton,
} from "./knowledge-detail-sections";

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

/** Runs a dialog action and closes the dialog on success; failures keep it open with its error. */
async function closeOnSuccess(action: () => Promise<void>, close: () => void) {
  try {
    await action();
    close();
  } catch {
    // Error state is surfaced through the dialog's error prop and the dialog stays open.
  }
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
  revertingDecisionId,
  revertError,
}: KnowledgeDetailPanelProps) {
  const { isLoading, isDeletePending, isEditPending = false, isDecisionsLoading = false } = status;
  const dialogs = useKnowledgeDetailDialogs(memory);
  const [showSource, setShowSource] = useState(false);

  if (isLoading) {
    return <KnowledgeDetailSkeleton />;
  }
  if (error) {
    return <KnowledgeDetailError error={error} />;
  }
  if (!memory) {
    return <KnowledgeDetailEmpty />;
  }

  const resolvedScope: KnowledgeScope = scope ?? memory.scope;
  const canEdit = onEdit !== undefined;

  return (
    <div
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-y-auto")}
      data-testid="knowledge-detail-panel"
    >
      <KnowledgeDetailHeader
        canEdit={canEdit}
        deleteDisabled={isDeletePending}
        editDisabled={isEditPending || content === undefined}
        memory={memory}
        onDeleteClick={() => dialogs.setDeleteOpen(true)}
        onEditClick={() => dialogs.setEditOpen(true)}
      />

      <div className="flex flex-col gap-6 py-5">
        <ContextBox data-testid="knowledge-detail-context" entries={buildPrimaryEntries(memory)} />

        {content ? (
          <KnowledgeContentSection
            content={content}
            onToggleSource={() => setShowSource(previous => !previous)}
            showSource={showSource}
          />
        ) : null}

        <Disclosure
          data-testid="knowledge-detail-more"
          label="Details"
          size="md"
          keepMounted
          triggerProps={{ "data-testid": "knowledge-detail-more-toggle" }}
          contentProps={{ className: "pt-3" }}
        >
          <ContextBox
            data-testid="knowledge-detail-facts"
            entries={buildDetailEntries(memory, resolvedScope, projectName)}
          />
        </Disclosure>

        <KnowledgeDecisionsSection
          decisions={decisions}
          error={decisionsError}
          isLoading={isDecisionsLoading}
          onRevertDecision={onRevertDecision}
          revertError={revertError}
          revertingDecisionId={revertingDecisionId}
        />
      </div>

      <KnowledgeDetailDialogs
        canEdit={canEdit}
        confirmDeleteOpen={dialogs.confirmDeleteOpen}
        content={content}
        deleteError={deleteError}
        editError={editError}
        editOpen={dialogs.editOpen}
        isDeletePending={isDeletePending}
        isEditPending={isEditPending}
        memory={memory}
        onConfirmDelete={() =>
          closeOnSuccess(
            () => onDelete(memory),
            () => dialogs.setDeleteOpen(false)
          )
        }
        onConfirmEdit={async input => {
          if (!onEdit) return;
          await closeOnSuccess(
            () => onEdit(memory, input),
            () => dialogs.setEditOpen(false)
          );
        }}
        onDeleteOpenChange={dialogs.setDeleteOpen}
        onEditOpenChange={dialogs.setEditOpen}
        scope={resolvedScope}
      />
    </div>
  );
}

export { KnowledgeDetailPanel };
export type { KnowledgeDetailPanelProps };
