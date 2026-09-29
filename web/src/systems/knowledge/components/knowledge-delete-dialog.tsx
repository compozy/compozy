import { Trash2 } from "lucide-react";

import { ConfirmDialog } from "@compozy/ui";

import type { KnowledgeScope } from "../types";
import { knowledgeScopeLabel } from "../lib/knowledge-formatters";

interface KnowledgeDeleteDialogProps {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  /** Memory display name; the user types it to confirm. */
  name: string;
  scope: KnowledgeScope;
  isPending: boolean;
  error?: string | null;
  onConfirm: () => Promise<void>;
}

function KnowledgeDeleteDialog({
  open,
  onOpenChange,
  name,
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
      confirmTyping={name}
      contentProps={{ "data-testid": "knowledge-delete-dialog" }}
      description={
        <>
          This removes “{name}” from {knowledgeScopeLabel(scope)} knowledge. Agents won’t see it
          anymore.
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
