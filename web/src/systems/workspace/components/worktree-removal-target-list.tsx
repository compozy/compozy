import { Check, CircleX, TriangleAlert } from "@compozy/ui/icons";

import { Spinner, cn } from "@compozy/ui";

import type { WorktreeRemovalTarget } from "../hooks/use-worktree-removal-batch";
import type { WorktreeRefusal } from "../lib/worktree-refusal";
import { isStaleDiscovered } from "../lib/worktree-display";
import { WorktreePath } from "./worktree-path";
import { WorktreeRemovalRiskRows } from "./worktree-remove-risk-rows";

export interface WorktreeRemovalTargetResult {
  target: WorktreeRemovalTarget;
  status: "pending" | "running" | "success" | "failed";
  message: string;
  refusal?: WorktreeRefusal | null;
}

/** Name, path, branch and the action one frozen target will take. */
function describeRemovalTarget(target: WorktreeRemovalTarget) {
  if (target.kind === "worktree") {
    const { row } = target;
    return {
      name: row.name,
      path: row.path,
      branch: row.branch,
      action: row.state === "missing" ? "Dismiss record" : "Remove",
      owner: row.profile_name,
    };
  }
  const { discovered } = target;
  return {
    name: discovered.name,
    path: discovered.path,
    // A detached checkout has no branch; the risk copy still needs a subject.
    branch: discovered.branch || "This detached checkout",
    action: isStaleDiscovered(discovered) ? "Clear stale entry" : "Remove",
    owner: null,
  };
}

function TargetStatusGlyph({ result }: { result: WorktreeRemovalTargetResult }) {
  switch (result.status) {
    case "running":
      return (
        <Spinner role="presentation" aria-label={undefined} className="size-3.5 text-subtle" />
      );
    case "success":
      return <Check aria-hidden="true" className="size-3.5 text-success" />;
    case "failed":
      return result.refusal?.risk ? (
        <TriangleAlert aria-hidden="true" className="size-3.5 text-warning" />
      ) : (
        <CircleX aria-hidden="true" className="size-3.5 text-destructive" />
      );
    default:
      return (
        <span
          aria-hidden="true"
          className="inline-block size-status-dot rounded-full bg-line-strong"
        />
      );
  }
}

/**
 * One target: status glyph, name, then a single quiet line — the path while
 * the outcome is open, the receipt once it lands. A refusal keeps the path and
 * adds its reason plus the quantified blockers, so the next decision is local.
 */
function WorktreeRemovalTargetRow({
  result,
  showOwner,
  detailedRisk,
}: {
  result: WorktreeRemovalTargetResult;
  showOwner: boolean;
  detailedRisk: boolean;
}) {
  const target = describeRemovalTarget(result.target);
  const settled = result.status === "success";
  const failed = result.status === "failed";
  return (
    <li
      data-testid={`worktree-bulk-result-${result.target.key}`}
      data-status={result.status}
      className="grid grid-cols-[16px_minmax(0,1fr)_auto] items-start gap-x-2.5 px-3 py-2.5"
    >
      <span className="flex h-5 items-center justify-center">
        <TargetStatusGlyph result={result} />
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate text-small-body font-medium text-fg">{target.name}</span>
        {settled ? (
          <span className="truncate text-micro text-subtle">{result.message}</span>
        ) : (
          <WorktreePath path={target.path} className="text-mono-id" />
        )}
        {/* The force step's detailed blockers already say why; the refusal sentence would repeat it. */}
        {failed && result.message && !(detailedRisk && result.refusal?.risk) ? (
          <span role="alert" className="mt-1 text-form-hint text-muted-foreground">
            {result.message}
          </span>
        ) : null}
        {failed && result.refusal?.risk ? (
          <WorktreeRemovalRiskRows
            risk={result.refusal.risk}
            worktreeName={target.name}
            branch={target.branch}
            compact={!detailedRisk}
            className="mt-1"
          />
        ) : null}
      </span>
      <span className="flex h-5 items-center text-micro whitespace-nowrap text-subtle">
        {/* "Remove" is what the title already says; only a different action is named. */}
        {result.status === "pending" && target.action !== "Remove" ? target.action : null}
        {result.status === "pending" && showOwner && target.owner ? ` · ${target.owner}` : null}
      </span>
    </li>
  );
}

/** The frozen targets as one ruled list, shared by the batch confirm and the force step. */
export function WorktreeRemovalTargetList({
  results,
  detailedRisk = false,
  className,
}: {
  results: WorktreeRemovalTargetResult[];
  /** The force step re-states each blocker with its explanation. */
  detailedRisk?: boolean;
  className?: string;
}) {
  const owners = new Set(
    results.flatMap(result =>
      result.target.kind === "worktree" ? [result.target.row.profile_id] : []
    )
  );
  return (
    <ul
      aria-live="polite"
      data-slot="worktree-removal-targets"
      className={cn(
        "max-h-72 divide-y divide-line-soft overflow-y-auto rounded-md border border-line",
        className
      )}
    >
      {results.map(result => (
        <WorktreeRemovalTargetRow
          key={result.target.key}
          result={result}
          showOwner={owners.size > 1}
          detailedRisk={detailedRisk}
        />
      ))}
    </ul>
  );
}
