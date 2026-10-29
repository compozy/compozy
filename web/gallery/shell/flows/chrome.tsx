/**
 * Canvas chrome around the flow: the legend of edge kinds, the uncovered-state warnings, the
 * layout status region and the zoom controls.
 */
import { useReactFlow, useStore } from "@xyflow/react";
import { MinusIcon, PlusIcon, ScanIcon, TriangleAlertIcon } from "lucide-react";
import { useState } from "react";

import { Button, cn } from "@compozy/ui";

import { edgeKinds, type CoverageWarning } from "#gallery/flow";
import {
  edgeInk,
  edgeInkHighlighted,
  edgeKindStyle,
  viewportDuration,
} from "#gallery/shell/flows/edge-style";
import type { LayoutStatus } from "#gallery/shell/flows/layout-state";
import { SelectionLink } from "#gallery/shell/selection-link";

const surface = "rounded-lg border border-line bg-card text-small-body text-muted";

function LineSample({
  dash,
  round,
  highlighted,
}: {
  dash?: string;
  round?: boolean;
  highlighted?: boolean;
}) {
  return (
    <svg aria-hidden="true" className="shrink-0" height="6" width="26">
      <line
        style={{
          stroke: highlighted ? edgeInkHighlighted : edgeInk,
          strokeWidth: highlighted ? 2 : 1.4,
          strokeDasharray: dash,
          strokeLinecap: round ? "round" : undefined,
        }}
        x1="1"
        x2="25"
        y1="3"
        y2="3"
      />
    </svg>
  );
}

export function FlowLegend() {
  return (
    <ul
      aria-label="Legend"
      className={cn(
        surface,
        "pointer-events-auto flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-2"
      )}
    >
      {edgeKinds.map(kind => (
        <li className="flex items-center gap-1.5" key={kind}>
          <LineSample dash={edgeKindStyle[kind].dash} round={edgeKindStyle[kind].round} />
          {edgeKindStyle[kind].label}
        </li>
      ))}
      <li className="flex items-center gap-1.5 text-accent">
        <LineSample highlighted />
        selected step's path
      </li>
    </ul>
  );
}

/** "N warnings": prototype states of the touched specs that no flow reaches, each a link. */
export function FlowWarnings({ warnings }: { warnings: CoverageWarning[] }) {
  const [open, setOpen] = useState(false);
  if (warnings.length === 0) return null;
  const count = `${warnings.length} ${warnings.length === 1 ? "warning" : "warnings"}`;
  return (
    <div className="pointer-events-auto flex max-w-80 flex-col items-end gap-1.5">
      <Button
        aria-controls="flow-warnings"
        aria-expanded={open}
        className="bg-card"
        data-testid="flow-warnings"
        onClick={() => setOpen(!open)}
        size="sm"
        variant="outline"
      >
        <TriangleAlertIcon aria-hidden="true" className="text-warning" />
        {count}
        <span className="text-muted">· states outside every flow</span>
      </Button>
      {open ? (
        <ul
          className="flex max-h-72 w-full flex-col gap-0.5 overflow-y-auto rounded-lg border border-line bg-popover p-1.5 text-popover-foreground"
          id="flow-warnings"
        >
          {warnings.map(warning => (
            <li key={`${warning.slug}:${warning.state}`}>
              <SelectionLink
                className="flex flex-col rounded-md px-2.5 py-1.5 outline-none hover:bg-hover focus-visible:shadow-focus-ring"
                to={{ kind: "prototype", slug: warning.slug, state: warning.state }}
              >
                <span className="font-mono text-mono-id tracking-mono-id">
                  {warning.slug} · {warning.state}
                </span>
                <span className="text-small-body text-muted">{warning.spec}</span>
              </SelectionLink>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

export function LayoutStatusRegion({
  status,
  onRetry,
}: {
  status: LayoutStatus;
  onRetry: () => void;
}) {
  return (
    <div
      aria-live="polite"
      className="pointer-events-none absolute inset-0 flex items-center justify-center"
      role="status"
    >
      {status === "failed" ? (
        <div className="pointer-events-auto flex flex-col items-center gap-3 rounded-xl border border-line bg-card p-5 text-center">
          <p className="text-body">Could not lay out the flow</p>
          <Button data-testid="flow-layout-retry" onClick={onRetry} size="sm">
            Try again
          </Button>
        </div>
      ) : status === "ready" ? null : (
        <p className={cn(surface, "px-3 py-2")}>Laying out the flow…</p>
      )}
    </div>
  );
}

export function ZoomControls({ onFit }: { onFit: () => void }) {
  const { zoomIn, zoomOut } = useReactFlow();
  // Rounded in the selector, so a zoom gesture re-renders only when the shown percentage changes.
  const percent = useStore(state => Math.round(state.transform[2] * 100));
  return (
    <div aria-label="Zoom" className="pointer-events-auto flex items-center gap-1" role="group">
      <Button
        aria-label="Zoom out"
        className="bg-card"
        onClick={() => void zoomOut({ duration: viewportDuration() })}
        size="icon-sm"
        variant="outline"
      >
        <MinusIcon aria-hidden="true" />
      </Button>
      <span className={cn(surface, "min-w-12 px-2 py-1 text-center font-mono tabular-nums")}>
        {percent}%
      </span>
      <Button
        aria-label="Zoom in"
        className="bg-card"
        onClick={() => void zoomIn({ duration: viewportDuration() })}
        size="icon-sm"
        variant="outline"
      >
        <PlusIcon aria-hidden="true" />
      </Button>
      <Button
        className="bg-card"
        data-testid="flow-fit"
        onClick={onFit}
        size="sm"
        variant="outline"
      >
        <ScanIcon aria-hidden="true" />
        Fit
      </Button>
    </div>
  );
}
