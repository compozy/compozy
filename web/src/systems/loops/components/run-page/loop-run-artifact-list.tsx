import { ArrowUpRight, FileText } from "lucide-react";

import { Pill } from "@compozy/ui";

import type { LoopRunOutcomeModel } from "../../lib/loop-run-artifacts";

interface LoopRunArtifactListProps {
  outcome: LoopRunOutcomeModel;
}

/**
 * What the run produced, listed under its outcome.
 *
 * Every entry the run wrote stays listed, including the ones whose content
 * retention has since removed: the name is evidence that the work happened, and
 * dropping the row would quietly revise the run's history. A pruned entry simply
 * offers nothing to open and says why.
 */
export function LoopRunArtifactList({ outcome }: LoopRunArtifactListProps) {
  if (outcome.producedNothing) {
    return (
      <p className="mt-2 text-small-body text-muted" data-testid="loop-run-artifacts-none">
        This run produced no outputs.
      </p>
    );
  }
  if (outcome.artifacts.length === 0) return null;
  return (
    <ul className="mt-3 flex flex-col gap-1.5" data-testid="loop-run-artifacts">
      {outcome.artifacts.map(artifact => (
        <li
          className="flex flex-wrap items-center gap-2 rounded-md border border-line-soft bg-canvas-tint px-3 py-2"
          data-availability={artifact.availability}
          data-testid={`loop-run-artifact-${artifact.name}`}
          key={artifact.key}
        >
          <FileText aria-hidden="true" className="size-3.5 shrink-0 text-muted" />
          <span className="min-w-0 truncate text-small-body font-medium text-fg-strong">
            {artifact.name}
          </span>
          {artifact.output ? (
            <span className="font-mono text-mono-id text-faint">via {artifact.output}</span>
          ) : null}
          {artifact.note ? (
            <Pill
              data-testid={`loop-run-artifact-note-${artifact.availability}`}
              tone={artifact.toneForNote ?? "neutral"}
            >
              {artifact.note}
            </Pill>
          ) : null}
          {artifact.ref ? (
            <span className="ml-auto inline-flex items-center gap-1 text-form-hint text-subtle">
              Open
              <ArrowUpRight aria-hidden="true" className="size-3" />
            </span>
          ) : null}
        </li>
      ))}
    </ul>
  );
}
