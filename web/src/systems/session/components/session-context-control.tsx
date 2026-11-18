import { useId, useState, type ComponentProps } from "react";
import { InputGroupButton, Tooltip, TooltipContent, TooltipTrigger, cn } from "@compozy/ui";
import type { SessionContextRingState, SessionContextView } from "../lib/session-context";
import {
  describeSessionContextControl,
  describeSessionContextRing,
  type SessionContextTooltipRow,
} from "../lib/session-context-view";

const RING_RADIUS = 6.5;

function SessionContextRing({
  state,
  fraction,
  className,
  ...props
}: ComponentProps<"svg"> & {
  state: SessionContextRingState;
  fraction: number;
}) {
  const maskId = `session-context-ring-${useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const ring = describeSessionContextRing(state, fraction);
  return (
    <svg
      aria-hidden="true"
      className={cn("size-4 shrink-0 -rotate-90", className)}
      viewBox="0 0 16 16"
      {...props}
    >
      <circle
        cx="8"
        cy="8"
        r={RING_RADIUS}
        fill="none"
        strokeWidth="2"
        stroke={ring.track.stroke}
        strokeDasharray={ring.track.dasharray}
      />
      {ring.arc ? (
        <circle
          cx="8"
          cy="8"
          r={RING_RADIUS}
          fill="none"
          strokeWidth="2"
          stroke={ring.arc.stroke}
          strokeLinecap={ring.arc.linecap}
          pathLength="1"
          strokeDasharray={`${fraction} 1`}
          mask={ring.arc.dotted ? `url(#${maskId})` : undefined}
          className="transition-[stroke-dasharray] duration-slow ease-out motion-reduce:transition-none"
        />
      ) : null}
      {ring.dot ? <circle cx="8" cy="8" r="1.6" fill="var(--color-subtle)" /> : null}
      {ring.arc?.dotted ? (
        // The mask dots the arc without touching its `fraction 1` dash, which stays the fill truth.
        <defs>
          <mask id={maskId} maskUnits="userSpaceOnUse" x="0" y="0" width="16" height="16">
            <circle
              cx="8"
              cy="8"
              r={RING_RADIUS}
              fill="none"
              stroke="#fff"
              strokeWidth="2"
              strokeDasharray="1.6 2.4"
            />
          </mask>
        </defs>
      ) : null}
    </svg>
  );
}

function SessionContextTooltipLine({
  row,
  className,
  ...props
}: ComponentProps<"p"> & { row: SessionContextTooltipRow }) {
  switch (row.kind) {
    case "numbers":
      return (
        <p className={cn("flex items-baseline gap-1 text-fg tabular-nums", className)} {...props}>
          {row.percent ? (
            <>
              <b className="text-small-body font-semibold tracking-tight">{row.percent}</b>
              <span aria-hidden="true" className="text-faint">
                {" · "}
              </span>
            </>
          ) : null}
          <span className="font-mono text-mono-id font-normal text-muted-foreground">
            {row.amount}
          </span>
        </p>
      );
    case "headline":
      return (
        <p className={cn("text-small-body font-medium text-fg", className)} {...props}>
          {row.text}
        </p>
      );
    case "stale":
      return (
        <p className={cn("flex items-center", className)} {...props}>
          <span className="text-micro text-subtle">Updated a while ago</span>
        </p>
      );
    default:
      return (
        <p className={className} {...props}>
          {row.text}
        </p>
      );
  }
}

/**
 * The composer's context meter and the one control that opens and closes the Context rail (#1487):
 * pressed while the rail is open, sharing the rail's per-session preference.
 */
export function SessionContextControl({
  context,
  onToggle,
  open = false,
}: {
  context: SessionContextView;
  onToggle: () => void;
  /** The Context rail is open: the trigger reads pressed and keeps its plate. */
  open?: boolean;
}) {
  const tooltipId = useId();
  const [tipOpen, setTipOpen] = useState(false);
  const view = describeSessionContextControl(context);
  return (
    <Tooltip open={tipOpen} onOpenChange={setTipOpen}>
      <TooltipTrigger
        render={
          <InputGroupButton
            type="button"
            size="tool-icon"
            aria-label={`${view.label}${context.stale ? ", may be out of date" : ""}`}
            aria-describedby={tipOpen ? tooltipId : undefined}
            aria-busy={context.loading}
            aria-pressed={open}
            onClick={onToggle}
            data-testid="composer-context-button"
            data-state={view.state}
            // Pressed reads as the held plate, from the ghost variant.
            className="shrink-0 data-[popup-open]:bg-surface-2 data-[popup-open]:text-fg"
          />
        }
      >
        <SessionContextRing state={view.state} fraction={view.fraction} />
      </TooltipTrigger>
      <TooltipContent
        role="tooltip"
        id={tooltipId}
        className="min-w-53 max-w-70 flex-col items-start gap-1.25 px-2.75 py-2.25 text-eyebrow leading-normal text-muted-foreground"
        side="top"
        align="start"
      >
        {view.rows.map(row => (
          <SessionContextTooltipLine
            key={row.kind === "sentence" ? `sentence:${row.text}` : row.kind}
            row={row}
          />
        ))}
      </TooltipContent>
    </Tooltip>
  );
}
