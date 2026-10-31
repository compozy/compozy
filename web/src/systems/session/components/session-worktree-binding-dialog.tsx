import { FolderGit2Icon, PlusIcon } from "@compozy/ui/icons";
import {
  Button,
  CommandItem,
  ConfirmDialog,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from "@compozy/ui";
import {
  WorktreeRefSelect,
  WorktreeStateChip,
  type WorktreeMaterialization,
  type WorktreePayload,
} from "@/systems/workspace";

interface SessionWorktreeBindingDialogProps {
  worktrees: readonly WorktreePayload[];
  currentWorktreeId: string;
  target: string;
  onTargetChange: (value: string) => void;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
  onNewWorktree: () => void;
  isPending: boolean;
  blockedReason?: string;
  available: boolean;
  materialization: WorktreeMaterialization;
}

/** Selects an execution directory while preserving this conversation. */
export function SessionWorktreeBindingDialog({
  worktrees,
  currentWorktreeId,
  target,
  onTargetChange,
  onConfirm,
  onOpenChange,
  onNewWorktree,
  isPending,
  blockedReason,
  available,
  materialization,
}: SessionWorktreeBindingDialogProps) {
  const materializing =
    materialization.status === "creating" || materialization.status === "pending";
  return (
    <ConfirmDialog
      open
      title="Use worktree"
      body={
        <Field>
          <FieldDescription>
            Continue this conversation in the selected worktree from the next turn. Files stay where
            they are.
          </FieldDescription>
          <FieldLabel>Worktree</FieldLabel>
          <WorktreeRefSelect
            ariaLabel="Session worktree"
            disabled={!available || isPending || materializing}
            footer={
              <CommandItem onSelect={onNewWorktree} value="new worktree">
                <PlusIcon aria-hidden="true" />
                New worktree…
              </CommandItem>
            }
            onChange={onTargetChange}
            value={target}
            worktrees={worktrees.filter(worktree => worktree.id !== currentWorktreeId)}
          />
          {materializing ? (
            <FieldDescription className="flex items-center gap-2">
              <WorktreeStateChip state="pending" />
              <span>{materialization.phase ?? "Creating worktree…"}</span>
              <Button onClick={materialization.cancel} size="sm" variant="ghost">
                Cancel
              </Button>
            </FieldDescription>
          ) : null}
          {materialization.status === "failed" ? (
            <FieldError>
              {materialization.error ?? "Worktree creation failed."}
              <Button onClick={materialization.retry} size="sm" variant="ghost">
                Retry
              </Button>
            </FieldError>
          ) : null}
        </Field>
      }
      confirmIcon={FolderGit2Icon}
      confirmLabel="Use worktree"
      cancelLabel="Cancel"
      confirmButtonProps={{ disabled: !available || materializing || !target }}
      isPending={isPending}
      note={blockedReason}
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
    />
  );
}
