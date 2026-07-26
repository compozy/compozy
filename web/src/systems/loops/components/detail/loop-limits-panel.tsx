import { Section } from "@agh/ui";

import { buildLoopLimits } from "../../lib/loop-limits";
import type { LoopEffectiveConfig } from "../../types";

interface LoopLimitsPanelProps {
  effectiveConfig: LoopEffectiveConfig;
}

/**
 * Right-rail Limits & budget panel: each per-loop default paired with its hard
 * daemon ceiling (§5.4). Cost is display-only; budgets are opt-in but enforced.
 */
export function LoopLimitsPanel({ effectiveConfig }: LoopLimitsPanelProps) {
  const rows = buildLoopLimits(effectiveConfig);
  return (
    <Section label="Limits & budget" data-testid="loop-limits">
      <div className="rounded-lg border border-line bg-canvas-soft">
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
          Left is the per-loop default; right is the daemon ceiling, a hard backstop that cannot be
          raised. Token and wall-clock budgets are off unless set (0 = unlimited); a set budget is
          enforced. Cost is display-only, not an enforced cap.
        </p>
      </div>
    </Section>
  );
}
