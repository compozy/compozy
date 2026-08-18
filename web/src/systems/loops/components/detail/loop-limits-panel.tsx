import { Gauge } from "lucide-react";

import { buildLoopLimits } from "../../lib/loop-limits";
import { resolveLoopEffectiveConfig } from "../../lib/loop-effective-config";
import type { LoopEffectiveConfig } from "../../types";
import { LoopRailSection } from "../loop-rail-section";

interface LoopLimitsPanelProps {
  effectiveConfig: LoopEffectiveConfig;
}

export function LoopLimitsPanel({ effectiveConfig }: LoopLimitsPanelProps) {
  const rows = buildLoopLimits(effectiveConfig);
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  const budgets =
    effective.budget_tokens > 0 || effective.budget_wall_sec > 0 ? "budgets set" : "no budgets set";
  return (
    <LoopRailSection
      data-testid="loop-limits"
      gist={`${effective.iteration_cap} generations · ${budgets}`}
      icon={<Gauge aria-hidden="true" className="size-3.5" />}
      title="Limits"
    >
      <>
        <div className="flex flex-col px-3.5 py-1">
          {rows.map(row => (
            <div
              key={row.label}
              className="flex items-center justify-between gap-2.5 border-t border-line-soft py-2 first:border-t-0"
              data-testid="loop-limit-row"
            >
              <span className="text-xs text-subtle">{row.label}</span>
              <span className="font-mono text-mono-id tabular-nums text-fg">
                {row.value} <span className="text-faint">{row.ceiling}</span>
              </span>
            </div>
          ))}
        </div>
        <p className="border-t border-line-soft px-3.5 py-3 text-form-hint leading-relaxed text-faint">
          Right values are daemon ceilings, hard backstops. A set budget is enforced.
        </p>
      </>
    </LoopRailSection>
  );
}
