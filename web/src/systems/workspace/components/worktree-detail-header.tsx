import { FolderGit2Icon } from "lucide-react";

import { Button, PillDot } from "@compozy/ui";

import { toWorktreeDisplayState } from "../lib/worktree-display";
import type { WorktreePayload } from "../types";
import { WorktreePath } from "./worktree-path";
import { WorktreeAgentSignal } from "./worktree-signal-parts";
import { WorktreeStateChip } from "./worktree-state-chip";

interface WorktreeDetailHeaderProps {
  worktree: WorktreePayload;
  /** Bound session's display title, for the agent lane. Payload, never composed. */
  sessionTitle?: string;
  onRemove?: () => void;
}

/**
 * The worktree context's identity line.
 *
 * The ready chip is allowed here — unlike list rows, this header has no siblings
 * to compare against, so the state carries information rather than noise.
 * Removal is absent while a worktree is still materializing: there is nothing on
 * disk to remove yet.
 */
export function WorktreeDetailHeader({
  worktree,
  sessionTitle,
  onRemove,
}: WorktreeDetailHeaderProps) {
  const state = toWorktreeDisplayState(worktree);

  return (
    <header
      className="flex items-center gap-3"
      data-agent={worktree.agent_activity}
      data-slot="worktree-detail-header"
      data-state={state}
    >
      <span
        aria-hidden="true"
        className="grid size-8 shrink-0 place-items-center rounded-md border border-line bg-elevated text-muted [&_svg]:size-[15px]"
        data-slot="worktree-detail-header-icon"
      >
        <FolderGit2Icon />
      </span>
      <div className="min-w-0 flex-1" data-slot="worktree-detail-header-main">
        <div className="flex min-w-0 items-center gap-2">
          <h1 className="truncate text-card-title font-semibold tracking-[-0.014em] text-fg-strong">
            {worktree.name}
          </h1>
          {state === "ready" ? (
            <PillDot size="sm" tone="success" />
          ) : (
            <WorktreeStateChip state={state} />
          )}
          <WorktreeAgentSignal activity={worktree.agent_activity} showLabel title={sessionTitle} />
        </div>
        {worktree.path ? (
          <WorktreePath className="mt-0.5 block w-full" path={worktree.path} />
        ) : null}
      </div>
      {onRemove && state === "ready" ? (
        <div
          className="flex shrink-0 items-center gap-1.5"
          data-slot="worktree-detail-header-actions"
        >
          <Button onClick={onRemove} size="sm" type="button" variant="outline">
            Remove…
          </Button>
        </div>
      ) : null}
    </header>
  );
}
