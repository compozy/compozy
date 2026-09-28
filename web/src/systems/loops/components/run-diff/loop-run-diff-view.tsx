import { useId, type ReactNode } from "react";
import { CircleDot, GitCompare, Info } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Empty,
  ListGroupHeader,
  Pill,
  SkeletonRows,
} from "@compozy/ui";

import type { LoopDiffGroupView, LoopDiffView } from "../../lib/loop-run-diff-model";
import { LoopRunDiffInputs } from "./loop-run-diff-inputs";
import { LoopRunDiffRow } from "./loop-run-diff-row";

export interface LoopRunDiffViewProps {
  view: LoopDiffView | null;
  isLoading?: boolean;

  error?: string;

  pickers?: ReactNode;
}

interface DiffBodyProps {
  view: LoopDiffView;
}

interface DiffGroupProps {
  group: LoopDiffGroupView;
}

const LIVE_SIDE_SENTENCE: Record<"base" | "against", string> = {
  base: "The left run is still running; results may change.",
  against: "The right run is still running; results may change.",
};

function DiffGroup({ group }: DiffGroupProps) {
  const headingId = `loop-diff-group-${useId()}-heading`;
  return (
    <section
      aria-labelledby={headingId}
      className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
      data-testid={`loop-diff-group-${group.change}`}
    >
      <ListGroupHeader
        count={group.rows.length}
        label={<span id={headingId}>{group.label}</span>}
      />
      <ul>
        {group.rows.map(row => (
          <LoopRunDiffRow key={row.key} row={row} />
        ))}
      </ul>
    </section>
  );
}

function DiffBody({ view }: DiffBodyProps) {
  return (
    <>
      {view.hasDefinitionDivergence ? (
        <Alert data-testid="loop-diff-divergence" role="note" variant="info">
          <Info aria-hidden="true" />
          <AlertTitle>These runs used different versions of the Loop</AlertTitle>
          <AlertDescription>
            Only steps in both are compared. A step that exists in one version alone is left out,
            not marked failed.
          </AlertDescription>
        </Alert>
      ) : null}
      {view.liveSide ? (
        <p
          className="flex flex-wrap items-center gap-2 text-form-hint text-muted"
          data-testid="loop-diff-live-side"
        >
          <Pill size="sm" tone="info">
            <CircleDot aria-hidden="true" />
            Live
          </Pill>
          {LIVE_SIDE_SENTENCE[view.liveSide]}
        </p>
      ) : null}
      <LoopRunDiffInputs inputs={view.inputs} />
      {view.isEmpty ? (
        <Empty
          data-testid="loop-diff-empty"
          description="Both runs match. Nothing changed, reran, or was skipped."
          framed
          icon={GitCompare}
          title="No differences"
          titleAs="h3"
        />
      ) : (
        <div className="flex flex-col gap-4">
          {view.groups.map(group => (
            <DiffGroup group={group} key={group.change} />
          ))}
        </div>
      )}
    </>
  );
}

export function LoopRunDiffView({ view, isLoading, error, pickers }: LoopRunDiffViewProps) {
  return (
    <section className="flex min-w-0 flex-col gap-4" data-testid="loop-run-diff-view">
      {pickers}
      {error ? (
        <Alert role="status" variant="neutral">
          <Info aria-hidden="true" />
          <AlertTitle>No comparison was returned</AlertTitle>
          <AlertDescription>
            <span className="font-mono text-mono-id break-words text-subtle">{error}</span>
          </AlertDescription>
        </Alert>
      ) : null}
      {!error && isLoading ? <SkeletonRows className="gap-4" count={3} /> : null}
      {!error && !isLoading && view ? <DiffBody view={view} /> : null}
    </section>
  );
}
