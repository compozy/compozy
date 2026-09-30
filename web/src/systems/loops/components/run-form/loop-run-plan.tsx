import { Eyebrow, Pill } from "@compozy/ui";

import { humanizeLoopNodeId } from "../../lib/loop-node-labels";
import type { LoopDryRunPreview } from "../../types";

interface LoopRunPlanProps {
  plan: LoopDryRunPreview;
}

export function LoopRunPlan({ plan }: LoopRunPlanProps) {
  const inputEntries = Object.entries(plan.resolved_inputs ?? {});
  return (
    <div className="rounded-lg bg-canvas shadow-card p-4" data-testid="loop-run-plan">
      <div className="flex items-center gap-2">
        <Eyebrow className="text-muted">Dry run · round {plan.generation} plan</Eyebrow>
      </div>
      <p className="mt-1.5 text-form-label leading-relaxed text-muted">
        Your inputs look good. This is what round {plan.generation} would run. Nothing was started.
      </p>
      {inputEntries.length > 0 ? (
        <div className="mt-3">
          <Eyebrow className="text-faint">Inputs</Eyebrow>
          <dl className="mt-1.5 flex flex-col gap-1">
            {inputEntries.map(([key, value]) => (
              <div key={key} className="flex items-baseline justify-between gap-3 text-form-label">
                <dt className="font-mono text-subtle">{key}</dt>
                <dd className="truncate font-mono text-fg" title={String(value)}>
                  {typeof value === "string" ? value : JSON.stringify(value)}
                </dd>
              </div>
            ))}
          </dl>
        </div>
      ) : null}
      <div className="mt-3">
        <Eyebrow className="text-faint">
          Plan · {plan.nodes.length} {plan.nodes.length === 1 ? "step" : "steps"}
        </Eyebrow>
        <ol className="mt-1.5 flex flex-col gap-1">
          {plan.nodes.map((node, index) => (
            <li
              key={node.id}
              className="flex items-center gap-2.5 text-form-label"
              data-testid="loop-run-plan-node"
            >
              <span className="w-4 shrink-0 font-mono text-mono-id text-faint">{index + 1}</span>
              <span className="text-fg-strong" title={node.id}>
                {humanizeLoopNodeId(node.id)}
              </span>
              <Pill mono size="xs" tone="neutral">
                {node.class}
              </Pill>
            </li>
          ))}
        </ol>
      </div>
    </div>
  );
}
