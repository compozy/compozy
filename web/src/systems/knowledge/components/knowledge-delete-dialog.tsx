import { Trash2 } from "lucide-react";

import { ConfirmDialog } from "@agh/ui";

import type { KnowledgeScope } from "../types";
import { knowledgeScopeLabel } from "../lib/knowledge-formatters";

interface KnowledgeDeleteDialogProps {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  filename: string;
  scope: KnowledgeScope;
  isPending: boolean;
  error?: string | null;
  onConfirm: () => Promise<void>;
}

function KnowledgeDeleteDialog({
  open,
  onOpenChange,
  filename,
  scope,
  isPending,
  error,
  onConfirm,
}: KnowledgeDeleteDialogProps) {
  return (
    <ConfirmDialog
      cancelButtonProps={{ "data-testid": "cancel-delete-memory-btn" }}
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "confirm-delete-memory-btn" }}
      confirmIcon={Trash2}
      confirmInputProps={{ "data-testid": "knowledge-delete-confirm-typing" }}
      confirmLabel="Delete"
      confirmTyping={filename}
      contentProps={{ "data-testid": "knowledge-delete-dialog" }}
      description={
        <>
          This removes <span className="font-mono">{filename}</span> from the {scope} scope. The
          controller records the delete decision; the file is removed from{" "}
          {knowledgeScopeLabel(scope)} after the decision applies.
        </>
      }
      error={error}
      errorProps={{ "data-testid": "knowledge-delete-dialog-error" }}
      isPending={isPending}
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
      open={open}
      title="Delete knowledge entry?"
      tone="danger"
    />
  );
}

export { KnowledgeDeleteDialog };
export type { KnowledgeDeleteDialogProps };
