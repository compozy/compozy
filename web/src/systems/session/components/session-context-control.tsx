import { useId, useState } from "react";
import { Button, Pill, Tooltip, TooltipContent, TooltipTrigger, cn } from "@compozy/ui";
import type { SessionContextView } from "../lib/session-context";
import {
  formatContextPercent,
  formatContextTokens,
  formatContextTurn,
} from "../lib/context-format";

export function SessionContextStateChip({ context }: { context: SessionContextView }) {
  const label =
    context.loading && context.used == null
      ? "loading"
      : context.state === "unavailable"
        ? "unavailable"
        : context.stale
          ? "stale"
          : context.warning
            ? "near compaction"
            : context.state === "estimated_size"
              ? "estimated size"
              : context.state;
  return (
    <Pill
      size="xs"
      form={context.state === "unavailable" ? "hollow" : "tint"}
      tone={
        context.state === "unavailable"
          ? "neutral"
          : context.stale || context.warning
            ? "warning"
            : context.state === "estimated_size"
              ? "info"
              : "neutral"
      }
    >
      {label}
    </Pill>
  );
}

export function SessionContextControl({
  context,
  onOpen,
}: {
  context: SessionContextView;
  onOpen: () => void;
}) {
  const { used, size, ratio } = context;
  const tooltipId = useId();
  const [open, setOpen] = useState(false);
  const label =
    context.loading && used == null
      ? "Context usage loading"
      : ratio != null
        ? `Context ${formatContextPercent(ratio)} used`
        : used != null
          ? `Context ${formatContextTokens(used)} used`
          : context.state === "unavailable"
            ? "Context usage unavailable"
            : "Context usage unknown";
  const amount =
    used != null
      ? `${formatContextTokens(used)}${size != null ? ` / ${formatContextTokens(size)}` : " used"}`
      : undefined;
  const fraction = ratio == null ? 0 : Math.max(0, Math.min(1, ratio));
  return (
    <Tooltip open={open} onOpenChange={setOpen}>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="sm"
            aria-label={`${label}${context.stale ? ", stale" : ""}`}
            aria-describedby={open ? tooltipId : undefined}
            aria-busy={context.loading}
            onClick={onOpen}
            data-testid="composer-context-button"
            className={cn("shrink-0 gap-1.5 px-2 text-muted", context.warning && "text-warning")}
          />
        }
      >
        <svg aria-hidden="true" className="size-4 shrink-0 -rotate-90" viewBox="0 0 16 16">
          <circle
            cx="8"
            cy="8"
            r="6"
            fill="none"
            stroke="var(--color-line)"
            strokeWidth="2"
            strokeDasharray={used == null ? "2 2" : undefined}
          />
          <circle
            cx="8"
            cy="8"
            r="6"
            fill="none"
            stroke={context.warning ? "var(--color-warning)" : "var(--color-fg)"}
            strokeWidth="2"
            opacity={context.stale ? 0.5 : undefined}
            pathLength="1"
            strokeDasharray={`${fraction} 1`}
          />
          {used != null && ratio == null ? (
            <circle cx="8" cy="8" r="1.5" fill="var(--color-subtle)" />
          ) : null}
        </svg>
        <span className="font-mono text-mono-id tabular-nums">
          {ratio != null
            ? formatContextPercent(ratio)
            : used != null
              ? formatContextTokens(used)
              : "Context"}
        </span>
      </TooltipTrigger>
      <TooltipContent
        role="tooltip"
        id={tooltipId}
        className="max-w-70 flex-col items-start gap-2"
        side="top"
        align="start"
      >
        {amount ? (
          <p className="flex items-baseline gap-1.5 tabular-nums">
            {ratio != null ? (
              <span
                className={cn("text-small-body font-semibold", context.warning && "text-warning")}
              >
                {formatContextPercent(ratio)}
              </span>
            ) : null}
            {ratio != null ? " · " : null}
            <span className="font-mono text-mono-id font-normal text-muted">{amount}</span>
          </p>
        ) : null}
        {context.loading && used == null ? (
          <p>Loading context usage</p>
        ) : context.state === "unavailable" ? (
          <p>Usage unavailable</p>
        ) : used == null ? (
          <p>This agent hasn't reported context usage.</p>
        ) : null}
        {used != null ? (
          <div className="flex items-center gap-2">
            {!context.stopped ? <SessionContextStateChip context={context} /> : null}
            {context.reported_turn_id ? (
              <span className="text-muted">
                as of turn {formatContextTurn(context.reported_turn_id)}
              </span>
            ) : null}
          </div>
        ) : null}
        {context.size_source === "catalog" ? <p>window from model catalog</p> : null}
        {context.pressure_threshold != null && context.size_source === "agent" ? (
          <p className={context.warning ? "text-warning" : "text-muted"}>
            Compaction runs at {formatContextPercent(context.pressure_threshold)}
          </p>
        ) : null}
      </TooltipContent>
    </Tooltip>
  );
}
