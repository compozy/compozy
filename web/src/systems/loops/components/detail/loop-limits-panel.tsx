import { Gauge } from "lucide-react";

import { PropertyRow } from "@compozy/ui";

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
  const rounds =
    effective.iteration_cap === 0 ? "No round limit" : `${effective.iteration_cap} rounds`;
  const budgets =
    effective.budget_tokens > 0 || effective.budget_wall_sec > 0 ? "budgets set" : "no budgets";
  return (
    <LoopRailSection
      data-testid="loop-limits"
      gist={`${rounds} · ${budgets}`}
      icon={<Gauge aria-hidden="true" className="size-3.5" />}
      title="Limits"
    >
      <div className="flex flex-col px-4 py-1">
        {rows.map(row => (
          <PropertyRow
            data-testid="loop-limit-row"
            key={row.label}
            label={row.label}
            valueTitle={row.ceiling || row.value}
          >
            {row.value}
          </PropertyRow>
        ))}
      </div>
    </LoopRailSection>
  );
}
