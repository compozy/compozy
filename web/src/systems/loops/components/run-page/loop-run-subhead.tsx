import type { ComponentProps } from "react";

import { cn, formatAbsoluteTime, formatRelativeTime } from "@agh/ui";

import { isTerminalLoopStatus } from "../../lib/loop-formatters";
import type { LoopRunRecord } from "../../types";

interface LoopRunSubheadProps extends ComponentProps<"div"> {
  run: LoopRunRecord;
  /** The watched-subject line (`PR 128`); null hides the segment. */
  subject: string | null;
  /** True when the pinned graph parks on a watch source ("Watching …" voice). */
  hasWatchSource: boolean;
  /** Ticking (running) or frozen (§5.4) elapsed label. */
  elapsedLabel: string;
}

function Dot() {
  return <span aria-hidden="true" className="inline-block size-0.5 rounded-full bg-faint" />;
}

function RelativeTime({ iso }: { iso: string }) {
  return (
    <time dateTime={iso} title={formatAbsoluteTime(iso)}>
      {formatRelativeTime(iso)}
    </time>
  );
}

/** The elapsed voice per §5.4: plain while running, `active`/`used` while parked. */
function elapsedSegment(run: LoopRunRecord, elapsedLabel: string): string | null {
  switch (run.status) {
    case "watching":
      return `${elapsedLabel} active`;
    case "needs-approval":
      return `${elapsedLabel} used`;
    case "paused":
      return null;
    default:
      return elapsedLabel;
  }
}

/**
 * The run subhead strip (§2 anatomy): run id, watched subject, started/ended
 * relative time, and the elapsed clock — one quiet line under the topbar.
 */
export function LoopRunSubhead({
  run,
  subject,
  hasWatchSource,
  elapsedLabel,
  className,
  ...props
}: LoopRunSubheadProps) {
  const terminal = isTerminalLoopStatus(run.status);
  const elapsed = elapsedSegment(run, elapsedLabel);
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-2 border-b border-line pb-3.5 text-form-label text-subtle",
        className
      )}
      data-testid="loop-run-subhead"
      {...props}
    >
      <span className="font-mono text-mono-id tabular-nums">{run.id}</span>
      {subject ? (
        <>
          <Dot />
          <span data-testid="loop-run-subject">
            {hasWatchSource ? "Watching " : null}
            <span className="font-medium text-muted">{subject}</span>
          </span>
        </>
      ) : null}
      <Dot />
      {terminal ? (
        <span>
          Ended <RelativeTime iso={run.last_progress_at} />
        </span>
      ) : (
        <span>
          Started <RelativeTime iso={run.started_at} />
        </span>
      )}
      {run.status === "paused" ? (
        <>
          <Dot />
          <span>
            Paused <RelativeTime iso={run.last_progress_at} />
          </span>
        </>
      ) : null}
      {elapsed ? (
        <>
          <Dot />
          <span className="font-mono text-mono-id tabular-nums" data-testid="loop-run-elapsed">
            {elapsed}
          </span>
        </>
      ) : null}
    </div>
  );
}
