import { Eyebrow, MetadataList } from "@compozy/ui";

import type { LoopRunGeneration, LoopRunGenerationOutput } from "../../types";

type ResolvedRuntime = NonNullable<LoopRunGenerationOutput["resolved_runtime"]>;
type RuntimeField = "provider" | "model" | "reasoning" | "speed";

interface RuntimeOutput {
  generation: number;
  output: LoopRunGenerationOutput;
  runtime: ResolvedRuntime;
}

const SOURCE_LABELS: Record<string, string> = {
  run: "per-run rule",
  frontmatter: "task frontmatter",
  config: "config rule",
  input: "runtime input",
  node: "node runtime",
  default: "runtime default",
  criterion: "criterion runtime",
  agent: "agent definition",
};

function sourceLabel(source: string | undefined): string | null {
  if (!source) return null;
  return SOURCE_LABELS[source] ?? source;
}

function speedOutcome(runtime: ResolvedRuntime): string | null {
  const resolution = runtime.speed_resolution;
  if (!resolution) return null;
  const reason = resolution.reason?.replaceAll("_", " ");
  return reason ? `${resolution.status} · ${reason}` : resolution.status;
}

function outputLabel({ generation, output }: RuntimeOutput): string {
  const item = output.item_index === undefined ? "" : ` · item ${output.item_index + 1}`;
  return `Generation ${generation} · ${output.node_id}${item}`;
}

function RuntimeFieldValue({ runtime, field }: { runtime: ResolvedRuntime; field: RuntimeField }) {
  const value = runtime[field];
  if (!value) return null;
  const source = sourceLabel(runtime.source[field]);
  return (
    <span className="inline-flex min-w-0 items-baseline justify-end gap-1.5 text-right">
      <span className="truncate font-mono text-mono-id text-fg">{value}</span>
      {source ? <span className="shrink-0 text-badge text-faint">({source})</span> : null}
    </span>
  );
}

function runtimeOutputs(generations: readonly LoopRunGeneration[]): RuntimeOutput[] {
  return generations.flatMap(generation =>
    generation.outputs.flatMap(output => {
      const runtime = output.resolved_runtime;
      return runtime ? [{ generation: generation.generation, output, runtime }] : [];
    })
  );
}

/**
 * Durable runtime selections from the generation projection. This is inspection
 * data only: each populated field carries the daemon-reported source that won.
 */
export function LoopRunResolvedRuntimes({
  generations,
}: {
  generations: readonly LoopRunGeneration[];
}) {
  const outputs = runtimeOutputs(generations);
  if (outputs.length === 0) return null;

  return (
    <section data-testid="loop-run-resolved-runtimes">
      <Eyebrow className="text-subtle">Resolved runtime</Eyebrow>
      <div className="mt-1.5 flex flex-col gap-3">
        {outputs.map(entry => {
          const key = `${entry.generation}:${entry.output.node_id}:${entry.output.item_index ?? ""}`;
          const taskRunId = entry.output.task_run_id;
          return (
            <section
              key={key}
              aria-label={`Resolved runtime for ${outputLabel(entry)}`}
              className="border-t border-line-soft pt-2 first:border-t-0 first:pt-0"
              data-testid="loop-run-resolved-runtime"
            >
              <div className="flex items-baseline justify-between gap-3">
                <span className="min-w-0 truncate font-mono text-mono-id text-fg">
                  {outputLabel(entry)}
                </span>
                {taskRunId ? (
                  <span className="shrink-0 font-mono text-mono-id text-faint">{taskRunId}</span>
                ) : null}
              </div>
              <MetadataList className="mt-1.5 gap-y-1">
                <MetadataList.Row label="Provider">
                  <RuntimeFieldValue runtime={entry.runtime} field="provider" />
                </MetadataList.Row>
                <MetadataList.Row label="Model">
                  <RuntimeFieldValue runtime={entry.runtime} field="model" />
                </MetadataList.Row>
                <MetadataList.Row label="Reasoning">
                  <RuntimeFieldValue runtime={entry.runtime} field="reasoning" />
                </MetadataList.Row>
                <MetadataList.Row label="Speed">
                  <RuntimeFieldValue runtime={entry.runtime} field="speed" />
                </MetadataList.Row>
                <MetadataList.Row label="Speed outcome">
                  <span className="font-mono text-mono-id text-fg">
                    {speedOutcome(entry.runtime)}
                  </span>
                </MetadataList.Row>
              </MetadataList>
            </section>
          );
        })}
      </div>
    </section>
  );
}
