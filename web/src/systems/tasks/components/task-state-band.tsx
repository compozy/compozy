import type * as React from "react";

import { cn, StatusCard, type PillTone } from "@agh/ui";

export interface TaskStateBandProps extends Omit<React.ComponentProps<"div">, "title"> {
  tone: PillTone;
  title: React.ReactNode;
  /** Plain-language consequence line under the title (max ~62ch). */
  body?: React.ReactNode;
  /** Operator scent (event type · id) rendered as quiet mono microtext. */
  micro?: React.ReactNode;
  /** Resolving actions, right-aligned and vertically centered. */
  actions?: React.ReactNode;
}

const TONE_SURFACE: Record<PillTone, string> = {
  neutral: "border-line bg-canvas-soft",
  accent: "border-accent-dim bg-canvas-soft",
  info: "border-info/25 bg-info-tint",
  success: "border-success/25 bg-success-tint",
  warning: "border-warning/25 bg-warning-tint",
  danger: "border-danger/25 bg-danger-tint",
};

const TONE_TITLE: Record<PillTone, string> = {
  neutral: "text-fg-strong",
  accent: "text-fg-strong",
  info: "text-info",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
};

/**
 * Flat tint band shared by the Overview state strip and run outcomes: one
 * state, one plain sentence, and its resolving action.
 */
export function TaskStateBand({
  tone,
  title,
  body,
  micro,
  actions,
  className,
  ...props
}: TaskStateBandProps) {
  return (
    <StatusCard
      data-slot="task-state-band"
      data-tone={tone}
      className={cn(
        "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-1.5 rounded-md border px-4 py-3",
        TONE_SURFACE[tone],
        className
      )}
      tone={tone}
      {...props}
    >
      <h3 className={cn("text-card-title font-medium", TONE_TITLE[tone])}>{title}</h3>
      {actions ? (
        <div className="col-start-2 row-span-2 row-start-1 flex items-center gap-2 self-center">
          {actions}
        </div>
      ) : null}
      {body ? (
        <div className="col-start-1 max-w-[62ch] text-small-body leading-relaxed text-muted">
          {body}
          {micro ? (
            <span className="mt-1.5 block font-mono text-mono-id text-faint">{micro}</span>
          ) : null}
        </div>
      ) : null}
    </StatusCard>
  );
}
