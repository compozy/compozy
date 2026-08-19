import type { ComponentProps } from "react";
import { Coins } from "lucide-react";

import { cn, Eyebrow, PropertyRow } from "@compozy/ui";

import type { LoopRunUsageRow, LoopUsageTone } from "../../lib/loop-run-usage";

interface LoopRunUsageRailProps extends ComponentProps<"div"> {
  rows: LoopRunUsageRow[];

  note: string | null;
}

const VALUE_TONE_CLASS: Record<LoopUsageTone, string> = {
  neutral: "",
  warn: "text-warning",
  danger: "text-danger",
};

export function LoopRunUsageRail({ rows, note, className, ...props }: LoopRunUsageRailProps) {
  return (
    <div className={cn("px-4.5 py-4", className)} data-testid="loop-run-usage" {...props}>
      <div className="mb-2 flex items-center gap-1.5">
        <Coins aria-hidden="true" className="size-3 text-muted" />
        <Eyebrow className="text-subtle">Usage</Eyebrow>
      </div>
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
