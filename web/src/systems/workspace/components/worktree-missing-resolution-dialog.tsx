import { Archive, FolderX, History, RotateCcw } from "lucide-react";

import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Eyebrow,
  Icon,
  Item,
  ItemContent,
  ItemDescription,
  ItemMedia,
  ItemTitle,
  MetadataList,
  MonoId,
} from "@compozy/ui";

import type { WorktreeRefusal } from "../lib/worktree-refusal";
import type { WorktreePayload } from "../types";

export interface WorktreeMissingResolutionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  worktree: WorktreePayload;
  /** Verbatim daemon outcome for an already-clean dismissal (idempotent no-op). */
  outcome: string | null;
  /** Set when re-verification found a different repository, or none. */
  refusal: WorktreeRefusal | null;
  isPending: boolean;
  onDismissRecord: () => void;
  /**
   * Re-verifies the recorded path. Adoption is the restore path: it revalidates
   * Git identity and restores this same record to ready.
   */
  onRestore: () => void;
}

/**
 * Out-of-band removal is reported, never punished (ADR-006). Neither action
 * touches session or task history, and the dialog says so before either choice.
 */
export function WorktreeMissingResolutionDialog({
  open,
  onOpenChange,
  worktree,
  outcome,
  refusal,
  isPending,
  onDismissRecord,
  onRestore,
}: WorktreeMissingResolutionDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="worktree-missing-dialog" showCloseButton={false} unframed>
        <DialogHeader variant="ruled">
          <span className="flex items-center gap-2">
            <Icon as={FolderX} size="sm" className="text-info" />
            <Eyebrow className="text-info">Missing worktree</Eyebrow>
          </span>
          <DialogTitle>{worktree.name} is missing from disk</DialogTitle>
          <DialogDescription>
            The directory was removed outside Compozy. Run history is preserved.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 px-5 py-4">
          <MetadataList data-testid="worktree-missing-facts">
            <MetadataList.Row label="Branch">
              <MonoId value={worktree.branch} preserveCase size="sm" />
            </MetadataList.Row>
            <MetadataList.Row label="Expected path">
              <MonoId value={worktree.path} preserveCase size="sm" />
            </MetadataList.Row>
          </MetadataList>

          {refusal ? (
            <Alert variant="danger" data-testid="worktree-missing-refusal">
              <AlertDescription>{refusal.message}</AlertDescription>
            </Alert>
          ) : null}

          {outcome ? (
            <Alert variant="default" data-testid="worktree-missing-outcome">
              <AlertDescription>{outcome}</AlertDescription>
            </Alert>
          ) : (
            <div className="flex flex-col gap-2">
              <Item
                as="button"
                disabled={isPending}
                data-testid="worktree-missing-dismiss"
                onClick={onDismissRecord}
              >
                <ItemMedia variant="icon">
                  <Icon as={Archive} size="sm" />
                </ItemMedia>
                <ItemContent>
                  <ItemTitle>Dismiss record</ItemTitle>
                  <ItemDescription>
                    Removes this entry from the list. Run history is preserved.
                  </ItemDescription>
                </ItemContent>
              </Item>

              <Item
                as="button"
                disabled={isPending}
                data-testid="worktree-missing-restore"
                onClick={onRestore}
              >
                <ItemMedia variant="icon">
                  <Icon as={RotateCcw} size="sm" />
                </ItemMedia>
                <ItemContent>
                  <ItemTitle>It’s back</ItemTitle>
                  <ItemDescription>
                    Re-checks the expected path and restores the entry only if the same repository
                    is found there.
                  </ItemDescription>
                </ItemContent>
              </Item>
            </div>
          )}

          <p className="inline-flex items-center gap-1.5 text-form-hint text-subtle">
            <History className="size-3.5 shrink-0" />
            Sessions and task runs bound to {worktree.name} stay readable either way.
          </p>
        </div>

        <DialogFooter variant="ruled">
          <Button size="sm" variant="ghost" onClick={() => onOpenChange(false)}>
            {outcome ? "Close" : "Cancel"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
