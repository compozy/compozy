import type { ComponentProps } from "react";
import { GitFork } from "lucide-react";
import { Link } from "@tanstack/react-router";

import type { LoopForkLink } from "../../types";
import { LoopSection } from "../loop-section";

export interface LoopRunLineageSectionProps extends Omit<
  ComponentProps<typeof LoopSection>,
  "children" | "icon" | "title"
> {
  forkedFrom: LoopForkLink | null;
  forks: readonly { run_id: string; generation: number }[];
}

interface LineageRowProps {
  generation: number;
  lead: string;
  runId: string;
  testId: string;
}

function LineageRow({ generation, lead, runId, testId }: LineageRowProps) {
  return (
    <li
      className="flex flex-wrap items-center gap-x-2 gap-y-1 border-t border-line-soft py-2 first:border-t-0 first:pt-0"
      data-testid={testId}
    >
      <GitFork aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      <span className="text-small-body text-fg">{lead}</span>
      {/* The fork point is only useful if it goes somewhere: US-009.EC-3 asks
          the story to link the related run, not merely to name it. */}
      <Link
        className="font-mono text-mono-id tabular-nums text-fg underline-offset-3 hover:underline"
        params={{ runId }}
        to="/loop-runs/$runId"
      >
        {runId}
      </Link>
      <span className="text-small-body text-muted">{`· round ${generation}`}</span>
    </li>
  );
}

export function LoopRunLineageSection({ forkedFrom, forks, ...props }: LoopRunLineageSectionProps) {
  if (!forkedFrom && forks.length === 0) return null;
  return (
    <LoopSection
      data-testid="loop-run-lineage"
      icon={<GitFork aria-hidden="true" />}
      title="Lineage"
      {...props}
    >
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
    </LoopSection>
  );
}
