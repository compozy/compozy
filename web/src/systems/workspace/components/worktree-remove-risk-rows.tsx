import { ArrowUpFromLine, FileDiff } from "lucide-react";

import { Icon } from "@compozy/ui";

import type { WorktreeRemovalRisk } from "../lib/worktree-refusal";

export interface WorktreeRiskRowProps {
  icon: typeof FileDiff;
  what: string;
  quantity: string;
  why: string;
}

/** One blocker, quantified. The "why" is the sentence that prevents the mistake. */
function WorktreeRiskRow({ icon, what, quantity, why }: WorktreeRiskRowProps) {
  return (
    <li
      data-slot="worktree-risk-row"
      className="flex gap-3 border-b border-line-soft py-3 last:border-b-0"
    >
      <Icon as={icon} size="sm" className="mt-0.5 shrink-0 text-warning" />
      <span className="flex min-w-0 flex-col gap-1">
        <span className="flex items-center gap-2">
          <span className="text-small-body font-semibold text-fg-strong">{what}</span>
          <span className="font-mono text-mono-id tabular-nums text-subtle">{quantity}</span>
        </span>
        <span className="text-form-hint text-subtle">{why}</span>
      </span>
    </li>
  );
}

export interface WorktreeRemovalRiskRowsProps {
  risk: WorktreeRemovalRisk;
  worktreeName: string;
  branch: string;
}

/**
 * Names exactly what is at risk. Rows render only for blockers that are real —
 * a zero count is not a blocker and must not pad the list.
 */
export function WorktreeRemovalRiskRows({
  risk,
  worktreeName,
  branch,
}: WorktreeRemovalRiskRowsProps) {
  return (
    <ul data-testid="worktree-remove-risks" className="flex flex-col">
      {risk.changedFiles > 0 ? (
        <WorktreeRiskRow
          icon={FileDiff}
          what={`${risk.changedFiles} changed ${risk.changedFiles === 1 ? "file" : "files"}`}
          quantity={`+${risk.insertions} −${risk.deletions}`}
          why={`${worktreeName} has uncommitted changes. Commit or stash first, or force remove.`}
        />
      ) : null}
      {risk.unpushedCommits > 0 ? (
        <WorktreeRiskRow
          icon={ArrowUpFromLine}
          what={`${risk.unpushedCommits} unpushed ${risk.unpushedCommits === 1 ? "commit" : "commits"}`}
          quantity={`↑${risk.unpushedCommits}`}
          why={`${branch} has commits not found on any other branch or remote. Deleting it will lose work. Push it first, or force remove.`}
        />
      ) : null}
    </ul>
  );
}
