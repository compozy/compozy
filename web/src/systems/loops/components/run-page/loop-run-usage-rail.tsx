import type { ComponentProps } from "react";

import { cn, Eyebrow, PropertyRow } from "@agh/ui";

import type { LoopRunUsageRow, LoopUsageTone } from "../../lib/loop-run-usage";

interface LoopRunUsageRailProps extends ComponentProps<"div"> {
  rows: LoopRunUsageRow[];
  /** The one policy/state note (§6); null on terminal runs. */
  note: string | null;
}

/** Value tint only near a ceiling (§1: color carries state, never decoration). */
const VALUE_TONE_CLASS: Record<LoopUsageTone, string> = {
  neutral: "",
  warn: "text-warning",
  danger: "text-danger",
};

/**
 * The Usage rail section (§2/§3): four `prow` rows — Time, Tokens, Cost,
 * Rounds — with faint ceilings (`/ 45m`, `/ ∞`, `estimate`) and the single
 * policy note.
 */
export function LoopRunUsageRail({ rows, note, className, ...props }: LoopRunUsageRailProps) {
  return (
    <div className={cn("px-4.5 py-4", className)} data-testid="loop-run-usage" {...props}>
      <Eyebrow className="mb-2 block text-subtle">Usage</Eyebrow>
      {rows.map(row => (
        <PropertyRow key={row.key} label={row.label} mono data-testid={`loop-run-usage-${row.key}`}>
          <span className={VALUE_TONE_CLASS[row.tone]}>{row.value}</span>
          <span className="text-faint">{row.max}</span>
        </PropertyRow>
      ))}
      {note ? (
        <p
          className="mt-2.5 text-badge leading-normal text-faint"
          data-testid="loop-run-usage-note"
        >
          {note}
        </p>
      ) : null}
    </div>
  );
}
