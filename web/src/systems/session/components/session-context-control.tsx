import { useId, useState } from "react";
import { Minimize2 } from "lucide-react";
import { Button, Pill, Tooltip, TooltipContent, TooltipTrigger, cn } from "@compozy/ui";
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
  warning,
}: {
  state: SessionContextRingState;
  fraction: number;
  warning: boolean;
}) {
  const maskId = `session-context-ring-${useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const ring = describeSessionContextRing(state, fraction, warning);
  return (
    <svg aria-hidden="true" className="size-4 shrink-0 -rotate-90" viewBox="0 0 16 16">
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

function SessionContextTooltipLine({ row }: { row: SessionContextTooltipRow }) {
  switch (row.kind) {
    case "numbers":
      return (
        <p className="flex items-baseline gap-1 text-fg-strong tabular-nums">
          {row.percent ? (
            <>
              <b
                className={cn(
                  "text-small-body font-semibold tracking-tight",
                  row.warning && "text-warning"
                )}
              >
                {row.percent}
              </b>
              <span aria-hidden="true" className="text-faint">
                {" · "}
              </span>
            </>
          ) : null}
          <span className="font-mono text-mono-id font-normal text-muted">{row.amount}</span>
        </p>
      );
    case "headline":
      return <p className="text-small-body font-medium text-fg">{row.text}</p>;
    case "state":
      return (
        <p className="flex items-center gap-1.5 text-subtle">
          {row.chip ? (
            <Pill size="xs" tone={row.chip.tone}>
              {row.chip.label}
            </Pill>
          ) : null}
          {row.chip && row.asOf ? (
            <span aria-hidden="true" className="text-faint">
              ·
            </span>
          ) : null}
          {row.asOf ? <span>{row.asOf}</span> : null}
        </p>
      );
    case "policy":
      return (
        <p className="flex items-center gap-1.5 text-warning">
          <Minimize2 aria-hidden="true" className="size-3 shrink-0" />
          {row.text}
        </p>
      );
    default:
      return <p>{row.text}</p>;
  }
}

export function SessionContextControl({
  context,
  onOpen,
  open = false,
}: {
  context: SessionContextView;
  onOpen: () => void;
  /** The Context rail is open: the trigger keeps its plate, sharing the rail's preference. */
  open?: boolean;
}) {
  const tooltipId = useId();
  const [tipOpen, setTipOpen] = useState(false);
  const view = describeSessionContextControl(context);
  return (
    <Tooltip open={tipOpen} onOpenChange={setTipOpen}>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={`${view.label}${context.stale ? ", stale" : ""}`}
            aria-describedby={tipOpen ? tooltipId : undefined}
            aria-busy={context.loading}
            onClick={onOpen}
            data-testid="composer-context-button"
            data-state={view.state}
            className={cn(
              "shrink-0 text-muted hover:text-fg data-[popup-open]:bg-btn-default-hover data-[popup-open]:text-fg",
              view.state === "warning" && "text-warning hover:text-warning",
              open && "bg-btn-default-hover text-fg-strong"
            )}
          />
        }
      >
        <SessionContextRing state={view.state} fraction={view.fraction} warning={view.warning} />
      </TooltipTrigger>
      <TooltipContent
        role="tooltip"
        id={tooltipId}
        className="min-w-53 max-w-70 flex-col items-start gap-1.25 px-2.75 py-2.25 text-eyebrow leading-[1.45] text-muted"
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
