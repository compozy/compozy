"use client";

import * as React from "react";

import { toneText } from "../../lib/tone";
import { cn } from "../../lib/utils";
import { ConnectionIndicator, type ConnectionStatus } from "./connection-indicator";
import { Eyebrow } from "./eyebrow";
import type { PillTone } from "./pill";

export interface StatusLineItem {
  /** Stable identity used when items reorder or filter. */
  key: React.Key;
  /** Optional structural prefix rendered as an `<Eyebrow>` (uppercase). */
  label?: string;
  /** Item content (count, identifier, short message). */
  value: React.ReactNode;
  /** Tone applied to the value text. Defaults to `neutral`. */
  tone?: PillTone;
}

export interface StatusLineProps extends React.ComponentProps<"div"> {
  /** Daemon connection status. */
  status: ConnectionStatus;
  /** Optional override for the connection LED label. */
  daemonLabel?: React.ReactNode;
  /** Typed item array + N-013 / N-005. Replaces the legacy `ReactNode[]` shape. */
  items: ReadonlyArray<StatusLineItem>;
}

function StatusLine({ status, daemonLabel, items, className, ...props }: StatusLineProps) {
  return (
    <div
      data-slot="status-line"
      data-status={status}
      className={cn("flex flex-wrap items-center gap-x-4 gap-y-1", className)}
      {...props}
    >
      <ConnectionIndicator label={daemonLabel} status={status} />
      {items.map(item => {
        const tone: PillTone = item.tone ?? "neutral";
        return (
          <span
            key={item.key}
            data-slot="status-line-item"
            data-tone={tone}
            className="flex items-center gap-1.5"
          >
            <span aria-hidden="true" className="text-subtle">
              ·
            </span>
            {item.label ? (
              <Eyebrow data-slot="status-line-item-label" className="text-muted">
                {item.label}
              </Eyebrow>
            ) : null}
            <span
              data-slot="status-line-item-value"
              className={cn(
                "font-sans text-form-label tracking-eyebrow tabular-nums",
                toneText(tone)
              )}
            >
              {item.value}
            </span>
          </span>
        );
      })}
    </div>
  );
}

export { StatusLine };
