import type { ComponentProps } from "react";
import { GitFork } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { cn, Pill } from "@compozy/ui";

import { LOOP_FORK_SIGNAL } from "../../lib/loop-request-vocabulary";
import type { LoopForkLink } from "../../types";

export interface LoopRunLineageSectionProps extends Omit<ComponentProps<"div">, "children"> {
  forkedFrom: LoopForkLink | null;
  forks: readonly { run_id: string; generation: number }[];
}

interface LineageRowProps {
  generation: number;
  lead: string;
  runId: string;
  testId: string;
}

const ForkGlyph = LOOP_FORK_SIGNAL.icon;

function LineageRow({ generation, lead, runId, testId }: LineageRowProps) {
  return (
    <li
      className="flex flex-wrap items-center gap-x-2 gap-y-1 border-t border-line-soft py-2 first:border-t-0 first:pt-0"
      data-testid={testId}
    >
      <Pill size="sm" tone={LOOP_FORK_SIGNAL.tone}>
        <ForkGlyph aria-hidden="true" />
        {LOOP_FORK_SIGNAL.word}
      </Pill>
      <span className="text-small-body text-fg">{lead}</span>
      {/* The fork point is only useful if it goes somewhere: US-009.EC-3 asks
          the story to link the related run, not merely to name it. */}
      <Link
        className="font-mono text-mono-id tabular-nums text-info hover:text-fg-strong"
        params={{ runId }}
        to="/loop-runs/$runId"
      >
        {runId}
      </Link>
      <span className="text-small-body text-muted">{`· generation ${generation}`}</span>
    </li>
  );
}

export function LoopRunLineageSection({
  forkedFrom,
  forks,
  className,
  ...props
}: LoopRunLineageSectionProps) {
  if (!forkedFrom && forks.length === 0) return null;
  return (
    <div className={cn(className)} data-testid="loop-run-lineage" {...props}>
      <div className="mb-2 flex items-center gap-1.5 text-subtle">
        <GitFork aria-hidden="true" className="size-3" />
        <h3 className="eyebrow text-subtle">Lineage</h3>
      </div>
      <ul className="flex flex-col">
        {forkedFrom ? (
          <LineageRow
            generation={forkedFrom.generation}
            lead="Forked from"
            runId={forkedFrom.run_id}
            testId="loop-lineage-forked-from"
          />
        ) : null}
        {forks.map(fork => (
          <LineageRow
            generation={fork.generation}
            key={fork.run_id}
            lead="Forked to"
            runId={fork.run_id}
            testId={`loop-lineage-fork-${fork.run_id}`}
          />
        ))}
      </ul>
    </div>
  );
}
