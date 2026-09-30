import { FolderGit2Icon, FolderXIcon } from "lucide-react";

import { Button, cn } from "@compozy/ui";

import {
  toWorktreeDisplayState,
  WorktreeStateChip,
  type WorktreePayload,
} from "@/systems/workspace";

interface SessionWorktreeBindingChipProps {
  /** The bound worktree record, or undefined when the binding no longer resolves. */
  worktree: WorktreePayload | undefined;
  /** The bound id, used when the record itself is gone. */
  worktreeId: string;
  onOpenContext?: () => void;
  onResolve?: () => void;
}

const CHIP_CLASS =
  "inline-flex h-6 items-center gap-1.5 whitespace-nowrap rounded-pill bg-surface-2 px-2.5 text-badge font-medium text-muted [&_svg]:size-3 [&_svg]:text-subtle hover:bg-selected hover:text-fg";

/**
 * States which worktree a session is bound to, in the session header.
 *
 * An unbound session renders nothing — absence is the signal, and a "workspace
 * root" placeholder would add noise to every session that never used a worktree.
 * The exit control deliberately has no mount point here.
 */
export function SessionWorktreeBindingChip({
  worktree,
  worktreeId,
  onOpenContext,
  onResolve,
}: SessionWorktreeBindingChipProps) {
  if (worktreeId.trim() === "") return null;

  const name = worktree?.name ?? worktreeId;
  // No record for a bound id means the row is gone, which is the missing state.
  const state = worktree ? toWorktreeDisplayState(worktree) : "missing";
  const missing = state === "missing";

  return (
    <span
      className="inline-flex items-center gap-1.5"
      data-slot="session-worktree-binding-chip"
      data-state={missing ? "missing" : state === "ready" ? "bound" : state}
    >
      <Button
        aria-label={
          missing ? `Worktree ${name} is missing — resolve` : `Worktree ${name} — open its context`
        }
        className={cn(
          CHIP_CLASS,
          missing && "border border-dashed border-line-strong text-warning [&_svg]:text-warning"
        )}
        onClick={missing ? onResolve : onOpenContext}
        size="sm"
        type="button"
        variant="outline"
      >
        {missing ? <FolderXIcon aria-hidden="true" /> : <FolderGit2Icon aria-hidden="true" />}
        {name}
        {missing ? (
          <span className="text-subtle" data-slot="binding-missing-suffix">
            {" · missing"}
          </span>
        ) : null}
      </Button>
      {!missing && state !== "ready" ? <WorktreeStateChip state={state} /> : null}
      {missing && onResolve ? (
        <Button onClick={onResolve} size="sm" type="button" variant="outline">
          Resolve…
        </Button>
      ) : null}
    </span>
  );
}
