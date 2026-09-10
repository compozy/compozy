import { Check, LoaderCircle } from "lucide-react";

import { SessionWorkEntryView } from "./session-work-entry";
import { summaryFailureSuffix } from "./session-timeline-summary";
import { isStreamingState, type SessionWorkRow } from "./session-timeline.logic";
import { TranscriptDisclosure } from "./transcript-disclosure";

export interface SessionToolGroupRowProps {
  row: SessionWorkRow;
  /** The owning turn ended in a turn-level failure (danger only then, ADR-009). */
  turnFailed: boolean;
  onToggle: () => void;
}

/** A compact work summary opens the original ordered tool and reasoning details. */
export function SessionToolGroupRow({ row, turnFailed, onToggle }: SessionToolGroupRowProps) {
  const summary = row.summary;
  if (!summary) return null;
  const detailsId = `${row.groupId}:entries`;
  const failed = summaryFailureSuffix(summary);
  const busy = row.entries.some(entry =>
    entry.kind === "tool" ? entry.status === "running" : isStreamingState(entry.state)
  );
  return (
    <div
      data-testid="work-summary-row"
      data-open={row.expanded}
      data-live={row.active || undefined}
      className="flex min-w-0 flex-col"
    >
      <TranscriptDisclosure
        className="min-w-0 max-w-full"
        title={summary.label}
        expanded={row.expanded}
        onToggle={onToggle}
        aria-controls={detailsId}
        icon={
          busy ? (
            <LoaderCircle
              aria-hidden="true"
              className="size-3 shrink-0 animate-spin text-subtle motion-reduce:animate-none"
            />
          ) : (
            <Check aria-hidden="true" className="size-3 shrink-0 text-subtle" strokeWidth={1.8} />
          )
        }
        label={
          <span data-testid="work-summary-label">
            {summary.label}
            {failed ? (
              <span data-testid="work-summary-failed">
                <span aria-hidden="true" className="px-1.5 text-faint">
                  ·
                </span>
                {failed}
              </span>
            ) : null}
          </span>
        }
      />
      <div
        id={detailsId}
        data-testid="work-summary-entries"
        hidden={!row.expanded}
        aria-hidden={!row.expanded}
        inert={!row.expanded}
        className={row.expanded ? "flex min-w-0 flex-col gap-0.5 pt-0.5" : undefined}
      >
        {row.expanded
          ? row.entries.map(entry => (
              <SessionWorkEntryView
                key={`${entry.kind}:${entry.id}`}
                entry={entry}
                active={row.active}
                turnFailed={turnFailed}
              />
            ))
          : null}
      </div>
    </div>
  );
}
