import type { PillTone } from "@agh/ui";
import { Metric, PillDot } from "@agh/ui";

import type { LoopKpi, LoopRunKpis } from "../../lib/loop-runs-view";

interface LoopRunsKpisProps {
  kpis: LoopRunKpis;
}

interface KpiConfig {
  key: keyof LoopRunKpis;
  label: string;
  tone: PillTone;
  pulse: boolean;
  testId: string;
}

const KPI_CONFIG: readonly KpiConfig[] = [
  { key: "activeNow", label: "Active now", tone: "accent", pulse: true, testId: "kpi-active-now" },
  {
    key: "awaitingYou",
    label: "Awaiting you",
    tone: "warning",
    pulse: false,
    testId: "kpi-awaiting-you",
  },
  {
    key: "doneToday",
    label: "Done today",
    tone: "success",
    pulse: false,
    testId: "kpi-done-today",
  },
  {
    key: "needsALook",
    label: "Needs a look",
    tone: "warning",
    pulse: false,
    testId: "kpi-needs-a-look",
  },
];

/**
 * The four workspace-wide Runs KPIs (Active now / Awaiting you / Done today /
 * Needs a look). The Active-now dot pulses to signal live work, gated by
 * `prefers-reduced-motion` inside `PillDot`.
 */
export function LoopRunsKpis({ kpis }: LoopRunsKpisProps) {
  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="loop-runs-kpis">
      {KPI_CONFIG.map(config => {
        const kpi: LoopKpi = kpis[config.key];
        return (
          <Metric
            key={config.key}
            labelCase="eyebrow"
            data-testid={config.testId}
            label={
              <span className="inline-flex items-center gap-1.5">
                <PillDot tone={config.tone} pulse={config.pulse} />
                {config.label}
              </span>
            }
            value={kpi.count}
            subtext={kpi.detail}
          />
        );
      })}
    </div>
  );
}
