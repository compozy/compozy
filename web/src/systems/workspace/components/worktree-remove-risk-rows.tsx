import { ArrowUpFromLine, FileDiff, FileX } from "@compozy/ui/icons";

import { Icon, cn } from "@compozy/ui";

import type { WorktreeRemovalRisk } from "../lib/worktree-refusal";

export interface WorktreeRiskRowProps {
  icon: typeof FileDiff;
  what: string;
  quantity: string;
  why: string;
  compact: boolean;
}

/** One blocker, quantified. The "why" is the sentence that prevents the mistake. */
function WorktreeRiskRow({ icon, what, quantity, why, compact }: WorktreeRiskRowProps) {
  return (
    <li
      data-slot="worktree-risk-row"
      className={cn(
        "grid grid-cols-[16px_minmax(0,1fr)] gap-x-2",
        compact ? "py-0.5" : "border-b border-line-soft py-2.5 last:border-b-0"
      )}
    >
      <Icon as={icon} size="sm" className="mt-0.5 justify-self-center text-warning" />
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="flex min-w-0 items-baseline gap-2">
          <span className="text-small-body font-medium text-fg">{what}</span>
          <span className="font-mono text-mono-id text-subtle tabular-nums">{quantity}</span>
        </span>
        {compact ? null : <span className="text-form-hint text-subtle">{why}</span>}
      </span>
    </li>
  );
}

export interface WorktreeRemovalRiskRowsProps {
  risk: WorktreeRemovalRisk;
  worktreeName: string;
  branch: string;
  /** One line per blocker, without the explanation — for dense per-target lists. */
  compact?: boolean;
  className?: string;
}

/**
 * Names exactly what is at risk. Rows render only for blockers that are real —
 * a zero count is not a blocker and must not pad the list.
 */
export function WorktreeRemovalRiskRows({
  risk,
  worktreeName,
  branch,
  compact = false,
  className,
}: WorktreeRemovalRiskRowsProps) {
  return (
    <ul data-testid="worktree-remove-risks" className={cn("flex flex-col", className)}>
      {risk.changedFiles > 0 ? (
        <WorktreeRiskRow
          compact={compact}
          icon={FileDiff}
          what={`${risk.changedFiles} changed ${risk.changedFiles === 1 ? "file" : "files"}`}
          quantity={`+${risk.insertions} −${risk.deletions}`}
          why={`Uncommitted changes in ${worktreeName}. Removing the checkout discards them.`}
        />
      ) : null}
      {risk.leftoverFiles > 0 ? (
        <WorktreeRiskRow
          compact={compact}
          icon={FileX}
          what={`${risk.leftoverFiles} leftover ${risk.leftoverFiles === 1 ? "file" : "files"}`}
          quantity="no .git"
          why={`${worktreeName} lost its Git metadata, so Git cannot tell which files changed. Force remove deletes all of them.`}
        />
      ) : null}
      {risk.unpushedCommits > 0 ? (
        <WorktreeRiskRow
          compact={compact}
          icon={ArrowUpFromLine}
          what={`${risk.unpushedCommits} unpushed ${risk.unpushedCommits === 1 ? "commit" : "commits"}`}
          quantity={`↑${risk.unpushedCommits}`}
          why={`${branch} has commits on no other branch or remote. Push them first to keep them.`}
        />
      ) : null}
    </ul>
  );
}
