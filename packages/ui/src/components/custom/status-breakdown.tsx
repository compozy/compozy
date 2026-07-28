"use client";

import * as React from "react";

import { toneBg } from "../../lib/tone";
import { cn } from "../../lib/utils";
import type { PillTone } from "./pill";

export interface StatusBreakdownItem {
  label: React.ReactNode;
  value: number;
  tone?: PillTone;
}

export interface StatusBreakdownProps extends React.ComponentProps<"div"> {
  items: ReadonlyArray<StatusBreakdownItem>;
  /** Total used to compute share; defaults to the sum of values. */
  total?: number;
}

function StatusBreakdown({ items, total, className, ...props }: StatusBreakdownProps) {
  const sum = total ?? items.reduce((acc, item) => acc + item.value, 0);
  return (
    <div data-slot="status-breakdown" className={cn("flex flex-col gap-2", className)} {...props}>
      <ul data-slot="status-breakdown-rows" className="flex flex-col gap-1.5">
        {items.map(item => {
          const ratio = sum > 0 ? Math.max(0, Math.min(1, item.value / sum)) : 0;
          const tone: PillTone = item.tone ?? "neutral";
          return (
            <li
              key={String(item.label)}
              data-slot="status-breakdown-row"
              className="flex items-center gap-3"
            >
              <span className="inline-flex w-24 shrink-0 truncate text-form-label text-muted">
                {item.label}
              </span>
              <div className="relative h-1.5 flex-1 overflow-hidden rounded-pill bg-canvas-tint">
                <span
                  data-slot="status-breakdown-bar"
                  className={cn("absolute inset-y-0 left-0 rounded-pill", toneBg(tone))}
                  style={{ width: `${Math.round(ratio * 100)}%` }}
                />
              </div>
              <span className="inline-flex w-12 shrink-0 justify-end font-mono text-mono-id tabular-nums text-muted">
                {item.value}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export { StatusBreakdown };
