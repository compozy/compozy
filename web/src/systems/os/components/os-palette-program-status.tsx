import { LoaderCircle, RefreshCw, TriangleAlert, Unplug } from "lucide-react";

import { Button } from "@compozy/ui";

import type { CmdPaletteViewProgramPhase } from "../stores/cmd-palette-view-program-store";

export function OsPaletteProgramBand({
  phase,
  onRetry,
}: {
  phase: "busy" | "degraded";
  onRetry: () => void;
}) {
  if (phase === "busy") {
    return (
      <div
        className="flex items-center gap-2 bg-surface-muted px-4 py-2 text-small-body text-muted"
        data-program-band="busy"
        role="status"
      >
        <LoaderCircle aria-hidden="true" className="size-3.5 animate-spin" />
        <span>updating</span>
      </div>
    );
  }
  return (
    <div
      className="flex items-center gap-2 bg-warning-tint px-4 py-2 text-small-body text-warning"
      data-program-band="degraded"
      role="status"
    >
      <TriangleAlert aria-hidden="true" className="size-3.5" />
      <span>degraded</span>
      <Button className="ml-auto" size="sm" variant="outline" onClick={onRetry}>
        Retry
      </Button>
    </div>
  );
}

export function OsPaletteProgramFailure({
  error,
  phase,
  source,
}: {
  error: string | null;
  phase: Extract<CmdPaletteViewProgramPhase, "circuit-open" | "unavailable">;
  source: string;
}) {
  const broken = phase === "circuit-open";
  const Icon = broken ? Unplug : TriangleAlert;
  return (
    <div
      className="flex min-h-44 flex-col items-center justify-center gap-1 px-6 text-center text-small-body"
      data-program-frame={phase}
      role="status"
    >
      <Icon
        aria-hidden="true"
        className={broken ? "mb-2 size-5 text-muted" : "mb-2 size-5 text-warning"}
      />
      <strong className="font-medium text-fg">{broken ? "view broken" : "view unavailable"}</strong>
      <span className="text-muted">{source}</span>
      {broken ? <span className="text-faint">until reopen</span> : null}
      {!broken && error ? <span className="mt-1 text-faint">{error}</span> : null}
    </div>
  );
}

export function OsPaletteProgramReloaded() {
  return (
    <div
      className="flex items-center gap-2 bg-success-tint px-4 py-2 text-small-body text-success"
      data-program-band="reloaded"
      role="status"
    >
      <RefreshCw aria-hidden="true" className="size-3.5" />
      <span>view reloaded</span>
    </div>
  );
}
