import { Pause, WifiOff } from "lucide-react";

import { Pill, Spinner, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { useSecondClock } from "@/hooks/use-second-clock";

import { useSessionTransportState } from "../hooks/use-session-transcript-thread-messages";
import { useTransportGraceElapsed } from "../hooks/use-transport-grace-elapsed";
import { formatMessageTimestamp } from "../lib/format-timestamp";
import {
  sessionTransportChip,
  type SessionTransportChipModel,
  type SessionTransportSnapshot,
} from "../lib/session-transport";
import { RECONNECT_MAX_DELAY_MS } from "../hooks/session-live-tail-store-contract";

function pausedSince(sinceMs: number | null, nowMs: number): string | null {
  if (sinceMs === null) return null;
  const minutes = Math.max(0, Math.floor((nowMs - sinceMs) / 60_000));
  return minutes < 1 ? null : minutes < 60 ? `${minutes}m` : `${Math.floor(minutes / 60)}h`;
}

function chipTooltip(model: SessionTransportChipModel, snapshot: SessionTransportSnapshot): string {
  const lostAt = snapshot.lastLiveAt === null ? "" : formatMessageTimestamp(snapshot.lastLiveAt);
  const cadence = `retrying every ${Math.round(RECONNECT_MAX_DELAY_MS / 1000)}s`;
  switch (model.kind) {
    case "connecting":
      return "Opening live updates";
    case "reconnecting":
      return lostAt
        ? `Live updates lost at ${lostAt} · ${cadence}`
        : `Live updates lost · ${cadence}`;
    case "catching-up":
      return "Replaying what happened while the stream was down";
    case "paused":
      return lostAt
        ? `Live updates paused in the background · last applied at ${lostAt}`
        : "Live updates paused in the background";
    case "disconnected":
      return lostAt
        ? `Live updates lost at ${lostAt} · gave up after ${model.attempts} tries`
        : `Gave up after ${model.attempts} tries`;
  }
}

const CHIP_PRESENTATION = {
  connecting: { tone: "info", label: "Connecting", Glyph: Spinner },
  reconnecting: { tone: "info", label: "Reconnecting", Glyph: Spinner },
  "catching-up": { tone: "info", label: "Catching up", Glyph: Spinner },
  paused: { tone: "neutral", label: "Paused", Glyph: Pause },
  disconnected: { tone: "danger", label: "Disconnected", Glyph: WifiOff },
} as const;

function chipCount(model: SessionTransportChipModel, nowMs: number): string | null {
  if (model.kind === "reconnecting") return String(model.attempt);
  if (model.kind === "paused") return pausedSince(model.sinceMs, nowMs);
  return null;
}

export interface SessionTransportChipProps {
  /** Whether this window is allowed to apply live frames; a background window reads paused. */
  windowLive: boolean;
}

/**
 * The connection chip at the window head's trailing edge (S4): absent while
 * the stream is healthy, info-toned while it is coming back, neutral when the
 * window is paused in the background, danger only once retries ran out.
 */
export function SessionTransportChip({ windowLive }: SessionTransportChipProps) {
  const transport = useSessionTransportState();
  const graceElapsed = useTransportGraceElapsed(transport.degradedAt);
  const model = sessionTransportChip(transport, { graceElapsed, windowLive });
  const nowMs = useSecondClock(model?.kind === "paused");
  if (model === null) return null;

  const { tone, label, Glyph } = CHIP_PRESENTATION[model.kind];
  const count = chipCount(model, nowMs);

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Pill
            aria-live="polite"
            data-testid="session-transport-chip"
            data-phase={model.kind}
            role="status"
            size="sm"
            tone={tone}
          >
            <Glyph aria-hidden="true" className="size-3" />
            {label}
            {count ? (
              <span
                className="font-mono text-mono-id font-semibold tracking-normal"
                data-testid="session-transport-chip-count"
              >
                ·{count}
              </span>
            ) : null}
          </Pill>
        }
      />
      <TooltipContent>{chipTooltip(model, transport)}</TooltipContent>
    </Tooltip>
  );
}
