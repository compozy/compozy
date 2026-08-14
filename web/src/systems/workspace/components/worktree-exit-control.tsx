import {
  ExternalLinkIcon,
  GitBranchIcon,
  GitCommitHorizontalIcon,
  GitPullRequestIcon,
} from "lucide-react";
import type { ComponentType } from "react";

import {
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  Eyebrow,
  SplitButton,
} from "@compozy/ui";

import type { WorktreeExitLadder, WorktreeExitLadderRow } from "../lib/worktree-exit-ladder";

interface WorktreeExitControlProps {
  ladder: WorktreeExitLadder;
  /** True while an action is in flight; the progress toast owns the reporting. */
  isRunning?: boolean;
  onAction: (row: WorktreeExitLadderRow) => void;
}

/**
 * Icons are the only thing the web maps from an action id. Labels, ordering and
 * blocked reasons are all payload.
 */
const ACTION_ICON: Record<string, ComponentType<{ className?: string }>> = {
  commit: GitCommitHorizontalIcon,
  commit_push: GitBranchIcon,
  push: GitBranchIcon,
  open_pr: GitPullRequestIcon,
  view_pr: ExternalLinkIcon,
};

const MENU_LABEL = "Git actions";

function ActionIcon({ action }: { action: string }) {
  const Glyph = ACTION_ICON[action];
  return Glyph ? <Glyph className="size-3" /> : null;
}

/**
 * The assisted exit, rendered exactly as the daemon computed it.
 *
 * Which action leads, which are blocked and why, and which exist at all are the
 * plan's decisions — this control adds an icon per action and nothing else. A
 * global pause blocks every row with the same daemon literal rather than a
 * locally worded summary.
 */
export function WorktreeExitControl({
  ladder,
  isRunning = false,
  onAction,
}: WorktreeExitControlProps) {
  const primary = ladder.primary;
  if (!primary) return null;

  const primaryReason = ladder.blocked ? ladder.blockedReason : primary.blockedReason;

  return (
    <div
      className="flex items-center gap-2"
      data-blocked={ladder.blocked ? "" : undefined}
      data-forge={ladder.forge ? undefined : "absent"}
      data-pause={ladder.blockedReason}
      data-running={isRunning ? "" : undefined}
      data-slot="worktree-exit-control"
    >
      <SplitButton
        aria-label={MENU_LABEL}
        blocked={ladder.blocked || !primary.enabled}
        blockedReason={primaryReason}
        disabled={isRunning}
        icon={<ActionIcon action={primary.action} />}
        label={primary.label}
        menuLabel={MENU_LABEL}
        onAction={() => onAction(primary)}
        role="group"
      >
        <DropdownMenuGroup>
          <DropdownMenuLabel className="px-2 pt-[5px] pb-1">
            <Eyebrow>{MENU_LABEL}</Eyebrow>
          </DropdownMenuLabel>
          {ladder.menuRows.map(row => {
            const blocked = ladder.blocked || !row.enabled;
            const reason = ladder.blocked ? ladder.blockedReason : row.blockedReason;
            return (
              <DropdownMenuItem
                className="flex w-full items-start gap-[9px] rounded-sm px-2 py-1.5 text-start [&_svg]:mt-0.5 [&_svg]:size-3 [&_svg]:shrink-0 [&_svg]:text-muted"
                data-action={row.action}
                data-blocked={blocked ? "" : undefined}
                data-publish={row.publish ? "" : undefined}
                data-slot="worktree-exit-action"
                disabled={blocked}
                key={row.action}
                onClick={() => onAction(row)}
                title={reason}
              >
                <ActionIcon action={row.action} />
                <span
                  className="min-w-0 flex-1 text-small-body text-fg"
                  data-slot="worktree-exit-action-label"
                >
                  {row.label}
                  {reason ? (
                    <span
                      className="mt-px block text-badge leading-[1.4] text-subtle"
                      data-slot="worktree-exit-action-reason"
                    >
                      {reason}
                    </span>
                  ) : null}
                </span>
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuGroup>
      </SplitButton>
      {ladder.blockedReason ? (
        <span aria-live="polite" className="sr-only" role="status">
          {ladder.blockedReason}
        </span>
      ) : null}
    </div>
  );
}
