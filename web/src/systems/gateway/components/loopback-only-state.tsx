import { Server } from "lucide-react";

import { Alert, AlertDescription, AlertTitle, Empty } from "@compozy/ui";

import { LOOPBACK_ONLY_STATE_COPY } from "../lib/gateway-copy";

export interface LoopbackOnlyStateProps {
  /**
   * `panel` — the framed full state for a surface slot (a deep-linked window,
   * a mutation view). `banner` — the slim shell-level strip for the 403
   * backstop, so a refused call never lands as a generic error toast.
   */
  layout?: "panel" | "banner";
  "data-testid"?: string;
}

/**
 * The truthful loopback-only state (S5, BR-3): the daemon refused because this
 * listener cannot execute the action — it belongs on the machine running
 * CompozyOS. Neutral/informative signal, never error-red: nothing failed, the
 * capability boundary explained itself. It persists until the tier latches
 * `local` again (US-002.EC-1), so there is no dismiss control to lose the
 * explanation behind.
 */
export function LoopbackOnlyState({
  layout = "panel",
  "data-testid": testId,
}: LoopbackOnlyStateProps) {
  if (layout === "banner") {
    return (
      <Alert
        className="border-line bg-canvas-soft"
        data-testid={testId ?? "gateway-loopback-only-banner"}
        variant="neutral"
      >
        <Server aria-hidden="true" />
        <AlertTitle>{LOOPBACK_ONLY_STATE_COPY.banner.title}</AlertTitle>
        <AlertDescription>{LOOPBACK_ONLY_STATE_COPY.banner.description}</AlertDescription>
      </Alert>
    );
  }
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
      data-testid={testId ?? "gateway-loopback-only-state"}
    >
      <Empty
        className="max-w-md"
        description={LOOPBACK_ONLY_STATE_COPY.panel.description}
        framed
        hint={LOOPBACK_ONLY_STATE_COPY.panel.hint}
        icon={Server}
        title={LOOPBACK_ONLY_STATE_COPY.panel.title}
      />
    </div>
  );
}
