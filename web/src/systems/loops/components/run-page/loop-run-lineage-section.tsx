import { GitFork } from "lucide-react";

import { Button, Pill } from "@compozy/ui";

import { LOOP_FORK_SIGNAL } from "../../lib/loop-request-vocabulary";
import type { LoopForkLink } from "../../types";

export interface LoopRunLineageSectionProps {
  forkedFrom: LoopForkLink | null;
  forks: readonly { run_id: string; generation: number }[];
  onOpenRun: (runId: string) => void;
}

interface LineageRowProps {
  generation: number;
  lead: string;
  onOpenRun: (runId: string) => void;
  runId: string;
  testId: string;
}

const ForkGlyph = LOOP_FORK_SIGNAL.icon;

function LineageRow({ generation, lead, onOpenRun, runId, testId }: LineageRowProps) {
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
      <Button
        className="h-auto p-0 font-mono text-mono-id tabular-nums text-info hover:text-fg-strong"
        onClick={() => onOpenRun(runId)}
        size="xs"
        type="button"
        variant="link"
      >
        {runId}
      </Button>
      <span className="text-small-body text-muted">{`· generation ${generation}`}</span>
    </li>
  );
}

export function LoopRunLineageSection({
  forkedFrom,
  forks,
  onOpenRun,
}: LoopRunLineageSectionProps) {
  if (!forkedFrom && forks.length === 0) return null;
  return (
    <div data-testid="loop-run-lineage">
      <div className="mb-2 flex items-center gap-1.5 text-subtle">
        <GitFork aria-hidden="true" className="size-3" />
        <h3 className="eyebrow text-subtle">Lineage</h3>
      </div>
      <ul className="flex flex-col">
        {forkedFrom ? (
          <LineageRow
            generation={forkedFrom.generation}
            lead="Forked from"
            onOpenRun={onOpenRun}
            runId={forkedFrom.run_id}
            testId="loop-lineage-forked-from"
          />
        ) : null}
        {forks.map(fork => (
          <LineageRow
            generation={fork.generation}
            key={fork.run_id}
            lead="Forked to"
            onOpenRun={onOpenRun}
            runId={fork.run_id}
            testId={`loop-lineage-fork-${fork.run_id}`}
          />
        ))}
      </ul>
    </div>
  );
}
