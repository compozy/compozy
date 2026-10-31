import { CircleAlert, CircleStop, Clock3 } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

import type { SessionTurnFoldRow } from "./session-timeline.logic";
import { TranscriptDisclosure } from "@compozy/ui";

export interface SessionTurnFoldRowViewProps {
  row: SessionTurnFoldRow;
  expanded: boolean;
  onToggle: () => void;
  /** Find's word on the fold: "N matches inside" while closed, "opened for a match" after a jump. */
  note?: ReactNode;
  /** The folded rows, rendered by the timeline so nesting keeps one renderer. */
  children: ReactNode;
}

/**
 * `SessionTurnFoldRow` (ADR-006 rule 2): a settled turn rests behind one line
 * whose clock glyph hands its place to a chevron on hover or open — opened, the
 * folded rows sit beside a spine dropping from that glyph, their interim prose
 * one ink step quieter than the answer —
 * "Worked for 4m 12s · Ran 6 commands · Edited 2 files" — closed by default,
 * expanding to the settled group rows. A turn a fallback steer interrupted
 * folds the same way and names its cause. The two open variants never
 * collapse: "You stopped after 1m 40s" in warning ink (you did this, nothing
 * broke — ADR-009 delta from production's danger) and "Failed after 2m 03s" in
 * danger, the one red the fold ever earns.
 */
export function SessionTurnFoldRowView({
  row,
  expanded,
  onToggle,
  note,
  children,
}: SessionTurnFoldRowViewProps) {
  const turnId = row.turnId ?? row.id;
  const detailsId = `turn-fold:${turnId}:entries`;
  if (row.open) {
    const failed = row.cause === "failed";
    const Glyph = failed ? CircleAlert : CircleStop;
    return (
      // The rows stay where they were; the label is the line that would have been
      // the fold, at the end of the turn where the stop happened (task_07 VC-06).
      <div
        data-testid={failed ? "turn-fold-failed" : "turn-fold-interrupted"}
        data-cause={row.cause}
        className="mb-2.5 flex min-w-0 flex-col gap-1 border-b border-line pb-1.5"
      >
        <div className="flex min-w-0 flex-col gap-0.5">{children}</div>
        <div
          data-testid="turn-fold-open-label"
          className={cn(
            "flex w-fit items-center gap-1.5 px-1 text-transcript-body tabular-nums",
            failed ? "text-danger" : "text-warning"
          )}
        >
          <Glyph className="size-3 shrink-0" aria-hidden="true" />
          <span>{row.label}</span>
        </div>
      </div>
    );
  }
  return (
    <div data-open={expanded} data-cause={row.cause} className="mb-2.5 min-w-0">
      <TranscriptDisclosure
        data-testid="turn-fold-row"
        className="min-w-0 max-w-full"
        expanded={expanded}
        onToggle={onToggle}
        aria-controls={detailsId}
        icon={<Clock3 aria-hidden="true" className="size-3.5 shrink-0 text-faint" />}
        label={<span className="tabular-nums">{row.label}</span>}
        trailing={
          note ? (
            <span className="ml-1.5 font-mono text-micro text-faint" data-testid="turn-fold-note">
              {note}
            </span>
          ) : null
        }
      />
      <div
        id={detailsId}
        hidden={!expanded}
        aria-hidden={!expanded}
        inert={!expanded}
        className={expanded ? "session-fold-body session-rail flex min-w-0 flex-col" : undefined}
      >
        {expanded ? children : null}
      </div>
    </div>
  );
}
