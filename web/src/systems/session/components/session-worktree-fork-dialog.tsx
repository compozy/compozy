import { CheckIcon, FileDiffIcon, GitBranchPlusIcon, PlusIcon } from "lucide-react";
import type { ReactNode } from "react";

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

interface SessionWorktreeForkDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sessionTitle: string;
  worktrees: readonly WorktreePayload[];
  /** The session's current binding, excluded from the target list. */
  currentWorktreeId?: string;
  targetWorktreeId: string;
  onTargetChange: (worktreeId: string) => void;
  onConfirm: () => void | Promise<void>;
  isPending?: boolean;
  /** Verbatim daemon reason. Present ⇒ the fork cannot run right now. */
  blockedReason?: string;
  onNewWorktree?: () => void;
  materialization?: WorktreeMaterialization;
}

function Fact({ children, icon, slot }: { children: ReactNode; icon: ReactNode; slot: string }) {
  return (
    <li
      className="flex items-start gap-2 text-small-body leading-[1.5] text-muted [&+&]:mt-2 [&_svg]:mt-0.5 [&_svg]:size-3 [&_svg]:shrink-0 [&_svg]:text-subtle"
      data-fact={slot}
    >
      {icon}
      <span>{children}</span>
    </li>
  );
}

/**
 * Confirms forking a live session into a worktree.
 *
 * The three facts are the whole point of the dialog: forking creates a second
 * session rather than moving this one, and nothing on disk changes. The new
 * session starts clean — the transcript does not travel with it — so the copy
 * says exactly that.
 */
export function SessionWorktreeForkDialog({
  open,
  onOpenChange,
  sessionTitle,
  worktrees,
  currentWorktreeId,
  targetWorktreeId,
  onTargetChange,
  onConfirm,
  isPending = false,
  blockedReason,
  onNewWorktree,
  materialization,
}: SessionWorktreeForkDialogProps) {
  const blocked = Boolean(blockedReason);
  const materializing =
    materialization?.status === "creating" || materialization?.status === "pending";
  const targets = worktrees.filter(worktree => worktree.id !== currentWorktreeId);

  return (
    <ConfirmDialog
      body={
        <div data-slot="session-fork-body" data-state={blocked ? "blocked" : "default"}>
          <ul className="mb-4" data-slot="session-fork-facts">
            <Fact icon={<PlusIcon aria-hidden="true" />} slot="new-session">
              A <span className="font-medium text-fg">new session</span> is created in the worktree,
              starting clean.
            </Fact>
            <Fact icon={<CheckIcon aria-hidden="true" />} slot="untouched">
              This session <span className="font-medium text-fg">stays untouched</span> and keeps
              running here.
            </Fact>
            <Fact icon={<FileDiffIcon aria-hidden="true" />} slot="changes-stay">
              Uncommitted changes <span className="font-medium text-fg">stay where they are</span> —
              nothing moves on disk.
            </Fact>
          </ul>
          <Field data-slot="session-fork-target">
            <FieldLabel>Target</FieldLabel>
            <WorktreeRefSelect
              ariaLabel="Fork target worktree"
              disabled={blocked || isPending || materializing}
              footer={
                onNewWorktree ? (
                  <CommandItem onSelect={onNewWorktree} value="new worktree">
                    <PlusIcon aria-hidden="true" className="size-3" />
                    New worktree…
                  </CommandItem>
                ) : undefined
              }
              onChange={onTargetChange}
              testId="session-fork-target"
              value={targetWorktreeId}
              worktrees={targets}
            />
            {materializing ? (
              <FieldDescription className="flex items-center gap-2">
                <WorktreeStateChip state="pending" />
                <span className="font-mono text-mono-id">
                  {materialization?.phase ?? "creating"}
                </span>
                <Button onClick={materialization?.cancel} size="sm" type="button" variant="ghost">
                  Cancel
                </Button>
              </FieldDescription>
            ) : null}
            {materialization?.status === "failed" ? (
              <FieldError className="flex items-center gap-2">
                <span>{materialization.error ?? "Worktree creation failed."}</span>
                <Button onClick={materialization.retry} size="sm" type="button" variant="ghost">
                  Retry
                </Button>
              </FieldError>
            ) : null}
          </Field>
        </div>
      }
      cancelLabel="Cancel"
      confirmButtonProps={{ disabled: blocked || materializing || targetWorktreeId === "" }}
      confirmIcon={GitBranchPlusIcon}
      confirmLabel="Fork session"
      isPending={isPending}
      note={blockedReason}
      noteTone={blocked ? "warning" : "neutral"}
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
      open={open}
      title={`Fork “${sessionTitle}” to a worktree?`}
      tone={blocked ? "neutral" : "accent"}
    />
  );
}
