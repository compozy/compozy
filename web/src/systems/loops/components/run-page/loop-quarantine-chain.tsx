import { Fragment } from "react";
import { KeyRound, TimerOff, TriangleAlert, Unplug } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import {
  formatAbsoluteTime,
  formatRelativeTime,
  Timeline,
  TimelineEvent,
  type PillTone,
} from "@compozy/ui";

import type { LoopQuarantineChainRow, LoopQuarantineEntry } from "../../lib/loop-quarantine-entry";
import { quarantineChainRows } from "../../lib/loop-quarantine-entry";

const DISPOSITION_TONE: Record<string, PillTone> = {
  retried: "warning",
  routed: "info",
  absorbed: "neutral",
  escalated: "warning",
  quarantined: "danger",
  canceled: "neutral",
};

const FAILURE_ICONS: Record<string, LucideIcon> = {
  transport: Unplug,
  attempt_timeout: TimerOff,
  payload_declared: KeyRound,
};

function attemptSpan(row: LoopQuarantineChainRow): string | null {
  if (!row.startedAt && !row.endedAt) return null;
  const start = row.startedAt ? formatAbsoluteTime(row.startedAt) : null;
  const end = row.endedAt ? formatAbsoluteTime(row.endedAt) : null;
  if (start && end) return `${start} → ${end}`;
  return start ?? end;
}

function EpisodeBoundary({ row }: { row: LoopQuarantineChainRow }) {
  const openedBy = row.openedBy;
  if (!openedBy) return null;
  const who = openedBy.actorId || openedBy.actorKind || "an operator";
  const when = openedBy.requestedAt ? formatRelativeTime(openedBy.requestedAt) : "";
  return (
    <li
      className="relative flex items-center gap-2 py-1.5 pl-6"
      data-testid={`loop-quarantine-episode-${row.episodeIndex}`}
    >
      <span
        aria-hidden="true"
        className="absolute top-1/2 left-2 size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-line"
      />
      <span className="min-w-0 font-mono text-pill-group-badge text-faint">
        {`Episode ${row.episodeIndex + 1} — requeued by ${who}`}
        {when ? ` ${when}` : ""}
        {openedBy.reason ? ` · ${openedBy.reason}` : ""}
      </span>
    </li>
  );
}

interface LoopQuarantineChainProps {
  entry: LoopQuarantineEntry;
}

/** Classified attempt chain on the canonical timeline spine. */
export function LoopQuarantineChain({ entry }: LoopQuarantineChainProps) {
  const rows = quarantineChainRows(entry);
  return (
    <Timeline ariaLabel="Classified failure chain" data-testid="loop-quarantine-chain-list">
      {rows.map(row => {
        const span = attemptSpan(row);
        const trail = [row.failureClass, row.disposition].filter(Boolean).join(" → ");
        return (
          <Fragment key={row.key}>
            {row.openedBy ? <EpisodeBoundary row={row} /> : null}
            <TimelineEvent
              data-testid={`loop-quarantine-attempt-${row.key}`}
              description={row.hint}
              icon={FAILURE_ICONS[row.failureClass ?? ""] ?? TriangleAlert}
              meta={
                <span className="font-mono text-pill-group-badge">
                  {[trail || null, span].filter(Boolean).join(" · ")}
                </span>
              }
              time={`attempt ${row.attempt}${
                row.endedAt ? ` · ${formatRelativeTime(row.endedAt)}` : ""
              }`}
              title={row.cause || row.failureCode || "An attempt failed"}
              tone={DISPOSITION_TONE[row.disposition ?? ""] ?? "neutral"}
            />
          </Fragment>
        );
      })}
    </Timeline>
  );
}
