import { AlertCircle, BookOpen, Code, Pencil, Trash2 } from "lucide-react";

import {
  Button,
  cn,
  CodeBlock,
  Empty,
  PAGE_CONTENT_GUTTER,
  Section,
  Skeleton,
  StreamMarkdown,
} from "@compozy/ui";

import type { KnowledgeMemoryItem, KnowledgeScope } from "@/systems/knowledge/types";

import { KnowledgeDeleteDialog } from "./knowledge-delete-dialog";
import { KnowledgeEditDialog } from "./knowledge-edit-dialog";

interface KnowledgeDetailHeaderProps {
  memory: KnowledgeMemoryItem;
  canEdit: boolean;
  editDisabled: boolean;
  deleteDisabled: boolean;
  onEditClick: () => void;
  onDeleteClick: () => void;
}

function KnowledgeDetailHeader({
  memory,
  canEdit,
  editDisabled,
  deleteDisabled,
  onEditClick,
  onDeleteClick,
}: KnowledgeDetailHeaderProps) {
  return (
    <header
      className="flex flex-wrap items-start justify-between gap-3 pt-4"
      data-testid="knowledge-detail-header"
    >
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <h2 className="text-detail-h1 font-medium tracking-detail-h1 text-fg">{memory.name}</h2>
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
        {canEdit ? (
          <Button
            data-testid="edit-memory-btn"
            disabled={editDisabled}
            onClick={onEditClick}
            size="sm"
            type="button"
            variant="secondary"
          >
            <Pencil />
            Edit
          </Button>
        ) : null}
        <Button
          data-testid="delete-memory-btn"
          disabled={deleteDisabled}
          onClick={onDeleteClick}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Trash2 />
          Delete
        </Button>
      </div>
    </header>
  );
}

interface KnowledgeContentSectionProps {
  content: string;
  showSource: boolean;
  onToggleSource: () => void;
}

function KnowledgeContentSection({
  content,
  showSource,
  onToggleSource,
}: KnowledgeContentSectionProps) {
  return (
    <Section
      label="Content"
      right={
        <Button
          aria-pressed={showSource}
          data-testid="knowledge-content-source-toggle"
          onClick={onToggleSource}
          size="xs"
          type="button"
          variant="ghost"
        >
          <Code />
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
  );
}

interface KnowledgeDetailDialogsProps {
  memory: KnowledgeMemoryItem;
  content: string | undefined;
  scope: KnowledgeScope;
  canEdit: boolean;
  confirmDeleteOpen: boolean;
  editOpen: boolean;
  isDeletePending: boolean;
  isEditPending: boolean;
  deleteError?: string | null;
  editError?: string | null;
  onConfirmDelete: () => Promise<void>;
  onConfirmEdit: (input: { content: string; description?: string }) => Promise<void>;
  onDeleteOpenChange: (open: boolean) => void;
  onEditOpenChange: (open: boolean) => void;
}

function KnowledgeDetailDialogs({
  memory,
  content,
  scope,
  canEdit,
  confirmDeleteOpen,
  editOpen,
  isDeletePending,
  isEditPending,
  deleteError,
  editError,
  onConfirmDelete,
  onConfirmEdit,
  onDeleteOpenChange,
  onEditOpenChange,
}: KnowledgeDetailDialogsProps) {
  return (
    <>
      <KnowledgeDeleteDialog
        error={deleteError}
        isPending={isDeletePending}
        name={memory.name}
        onConfirm={onConfirmDelete}
        onOpenChange={onDeleteOpenChange}
        open={confirmDeleteOpen}
        scope={scope}
      />

      {canEdit ? (
        <KnowledgeEditDialog
          error={editError}
          filename={memory.filename}
          initialContent={content ?? ""}
          initialDescription={memory.description ?? ""}
          isPending={isEditPending}
          name={memory.name}
          onConfirm={onConfirmEdit}
          onOpenChange={onEditOpenChange}
          open={editOpen}
          type={memory.type}
        />
      ) : null}
    </>
  );
}

function KnowledgeDetailError({ error }: { error: Error }) {
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

function KnowledgeDetailEmpty() {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center py-10"
      data-testid="knowledge-detail-empty"
    >
      <Empty className="max-w-md" icon={BookOpen} title="Select a memory to view details" />
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
        <Skeleton className="h-5 w-48" />
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

export {
  KnowledgeContentSection,
  KnowledgeDetailDialogs,
  KnowledgeDetailEmpty,
  KnowledgeDetailError,
  KnowledgeDetailHeader,
  KnowledgeDetailSkeleton,
};
