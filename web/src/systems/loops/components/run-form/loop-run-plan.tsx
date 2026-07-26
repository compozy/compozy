import { Eyebrow } from "@agh/ui";

import type { LoopDryRunPreview } from "../../types";
import { MonoTag } from "../mono-tag";

interface LoopRunPlanProps {
  plan: LoopDryRunPreview;
}

/**
 * Renders the dry-run result (§4.3 / §9.1): the resolved gen-1 plan the daemon would
 * execute — resolved inputs + the materialized node list — with no run created and no
 * budget spent. It appears only after a successful Dry run.
 */
export function LoopRunPlan({ plan }: LoopRunPlanProps) {
  const inputEntries = Object.entries(plan.resolved_inputs ?? {});
  return (
    <div
      className="rounded-lg border border-success/25 bg-success-tint px-4 py-3.5"
      data-testid="loop-run-plan"
    >
      <div className="flex items-center gap-2">
        <Eyebrow className="text-success">Dry run · generation {plan.generation} plan</Eyebrow>
      </div>
      <p className="mt-1.5 text-form-label leading-relaxed text-muted">
        Inputs validated. This is what generation {plan.generation} would run — no run was created
        and no budget was spent.
      </p>
      {inputEntries.length > 0 ? (
        <div className="mt-3">
          <Eyebrow className="text-faint">Resolved inputs</Eyebrow>
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
        <Eyebrow className="text-faint">Plan · {plan.nodes.length} nodes</Eyebrow>
        <ol className="mt-1.5 flex flex-col gap-1">
          {plan.nodes.map((node, index) => (
            <li
              key={node.id}
              className="flex items-center gap-2.5 text-form-label"
              data-testid="loop-run-plan-node"
            >
              <span className="w-4 shrink-0 font-mono text-mono-id text-faint">{index + 1}</span>
              <span className="font-mono text-fg-strong">{node.id}</span>
              <MonoTag className="rounded-xs bg-badge-fill px-1.5 py-0.5">{node.class}</MonoTag>
              <span className="truncate font-mono text-mono-id text-subtle">{node.kind}</span>
            </li>
          ))}
        </ol>
      </div>
    </div>
  );
}
