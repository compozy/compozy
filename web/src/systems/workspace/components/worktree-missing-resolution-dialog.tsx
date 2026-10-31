import { Archive, FolderX, History, RotateCcw } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Eyebrow,
  Field,
  FieldDescription,
  FieldLabel,
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
import { WorktreePath } from "./worktree-path";

export interface WorktreeMissingResolutionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  worktree: WorktreePayload;
  /** Verbatim daemon outcome for an already-clean dismissal (idempotent no-op). */
  outcome: string | null;
  /** Set when re-verification found a different repository, or none. */
  refusal: WorktreeRefusal | null;
  error?: string | null;
  isPending: boolean;
  /** Opt-in: dismissing also deletes the recorded local branch. */
  deleteBranch: boolean;
  onDeleteBranchChange: (deleteBranch: boolean) => void;
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
  error,
  isPending,
  deleteBranch,
  onDeleteBranchChange,
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
            The directory was removed outside CompozyOS. Run history is preserved.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 px-5 py-4">
          <MetadataList data-testid="worktree-missing-facts">
            <MetadataList.Row label="Branch">
              <MonoId value={worktree.branch} preserveCase size="sm" />
            </MetadataList.Row>
            <MetadataList.Row label="Expected path">
              <WorktreePath className="w-full" path={worktree.path} />
            </MetadataList.Row>
          </MetadataList>

          {refusal || error ? (
            <Alert variant="destructive" data-testid="worktree-missing-refusal">
              <AlertDescription>{refusal?.message ?? error}</AlertDescription>
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
                    {deleteBranch
                      ? "Removes this entry and its local branch. Run history is preserved."
                      : "Removes this entry from the list. Run history is preserved."}
                  </ItemDescription>
                </ItemContent>
              </Item>

              {worktree.branch ? (
                <Field orientation="horizontal" className="px-3 py-1">
                  <Checkbox
                    id="worktree-missing-delete-branch"
                    aria-describedby="worktree-missing-delete-branch-hint"
                    checked={deleteBranch}
                    disabled={isPending}
                    data-testid="worktree-missing-delete-branch"
                    onCheckedChange={checked => onDeleteBranchChange(checked === true)}
                  />
                  <div className="grid gap-1">
                    <FieldLabel htmlFor="worktree-missing-delete-branch">
                      Also delete local branch when dismissing
                    </FieldLabel>
                    <FieldDescription id="worktree-missing-delete-branch-hint">
                      Kept if it is the main checkout’s branch, the default branch, or has unmerged
                      commits.
                    </FieldDescription>
                  </div>
                </Field>
              ) : null}

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
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {outcome ? "Close" : "Cancel"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
