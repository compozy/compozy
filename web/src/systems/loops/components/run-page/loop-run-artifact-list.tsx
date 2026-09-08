import { ChevronDown, FileText } from "lucide-react";

import { Button, Collapsible, CollapsibleContent, CollapsibleTrigger, Pill } from "@compozy/ui";

import type { LoopArtifactRow, LoopRunOutcomeModel } from "../../lib/loop-run-artifacts";

interface LoopRunArtifactListProps {
  outcome: LoopRunOutcomeModel;
}

const PREVIEW_COUNT = 3;

export function LoopRunArtifactList({ outcome }: LoopRunArtifactListProps) {
  if (outcome.producedNothing) {
    return (
      <p className="mt-2 text-small-body text-muted" data-testid="loop-run-artifacts-none">
        This run produced no outputs.
      </p>
    );
  }
  if (outcome.artifacts.length === 0) return null;
  const preview = outcome.artifacts.slice(0, PREVIEW_COUNT);
  const remaining = outcome.artifacts.slice(PREVIEW_COUNT);
  const partialCount = outcome.artifacts.filter(
    artifact => artifact.availability === "partial"
  ).length;
  const prunedCount = outcome.artifacts.filter(
    artifact => artifact.availability === "pruned"
  ).length;
  return (
    <div className="mt-3 space-y-2" data-testid="loop-run-artifacts">
      <div className="flex flex-wrap items-center gap-2 text-small-body text-muted">
        <span>
          {outcome.artifacts.length} {outcome.artifacts.length === 1 ? "output" : "outputs"}
        </span>
        {partialCount > 0 ? <Pill tone="warning">{partialCount} partial</Pill> : null}
        {prunedCount > 0 ? <span>{prunedCount} no longer stored</span> : null}
      </div>
      <ArtifactRows artifacts={preview} />
      {remaining.length > 0 ? (
        <Collapsible>
          <CollapsibleTrigger
            className="group/artifacts"
            render={
              <Button size="sm" type="button" variant="ghost">
                {remaining.length} more outputs
                <ChevronDown
                  aria-hidden="true"
                  className="group-data-panel-open/artifacts:rotate-180"
                />
              </Button>
            }
          />
          <CollapsibleContent className="pt-2">
            <ArtifactRows artifacts={remaining} />
          </CollapsibleContent>
        </Collapsible>
      ) : null}
    </div>
  );
}

function ArtifactRows({ artifacts }: { artifacts: LoopArtifactRow[] }) {
  return (
    <ul className="flex flex-col gap-1.5">
      {artifacts.map(artifact => (
        <li
          className="rounded-md border border-line-soft px-3 py-2"
          data-availability={artifact.availability}
          data-testid={`loop-run-artifact-${artifact.name}`}
          key={artifact.key}
        >
          <Collapsible>
            <div className="flex flex-wrap items-center gap-2">
              <FileText aria-hidden="true" className="size-3.5 shrink-0 text-muted" />
              <span className="min-w-0 break-words text-small-body font-medium text-fg-strong">
                {artifact.name}
              </span>
              {artifact.note ? (
                <Pill
                  data-testid={`loop-run-artifact-note-${artifact.availability}`}
                  tone={artifact.toneForNote ?? "neutral"}
                >
                  {artifact.note}
                </Pill>
              ) : null}
              {artifact.ref || artifact.output ? (
                <CollapsibleTrigger
                  className="group/artifact ml-auto"
                  render={
                    <Button
                      aria-label={`Details for ${artifact.name}`}
                      size="sm"
                      type="button"
                      variant="ghost"
                    >
                      Details
                      <ChevronDown
                        aria-hidden="true"
                        className="group-data-panel-open/artifact:rotate-180"
                      />
                    </Button>
                  }
                />
              ) : null}
            </div>
            <CollapsibleContent>
              {artifact.output ? (
                <p className="mt-2 text-small-body text-muted">Output: {artifact.output}</p>
              ) : null}
              {artifact.ref ? (
                <pre
                  className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-md bg-canvas-tint p-3 font-mono text-mono-id text-muted"
                  data-testid={`loop-run-artifact-ref-${artifact.name}`}
                >
                  {artifact.ref}
                </pre>
              ) : null}
            </CollapsibleContent>
          </Collapsible>
        </li>
      ))}
    </ul>
  );
}
