import { Link } from "@tanstack/react-router";
import { ArrowRight } from "lucide-react";

import { loopRunOriginLine } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { LoopStatusPill } from "../loop-status-pill";

interface LoopRunActiveNoticeProps {
  run: LoopRun;
  /** The definition's declared concurrency policy, when it declares one. */
  concurrency?: string;
}

/**
 * What the declared `concurrency` policy means for starting a second run, in the
 * operator's terms. An unrecognized policy says nothing rather than guessing.
 */
function concurrencyNote(concurrency: string | undefined, runId: string): string | null {
  if (concurrency === "forbid") {
    return `This loop runs one at a time. Cancel ${runId} from its run page, or let it finish, before starting another.`;
  }
  if (concurrency === "queue") return `Starting another run queues it behind ${runId}.`;
  if (concurrency === "allow") return "Starting another run leaves this one running.";
  return null;
}

/**
 * The run of this loop that is already live when the form opens.
 *
 * Informative only: the run's own page owns pause/resume/cancel/kill, so this notice
 * links there instead of duplicating controls that would need their own confirmation
 * and lifecycle truth.
 */
export function LoopRunActiveNotice({ run, concurrency }: LoopRunActiveNoticeProps) {
  const note = concurrencyNote(concurrency, run.id);
  return (
    <section
      className="flex flex-col gap-2 rounded-md border border-line-soft bg-canvas-tint px-3.5 py-3"
      data-testid="loop-run-active-notice"
    >
      <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5">
        <LoopStatusPill status={run.status} />
        <span className="font-mono text-mono-id text-fg" data-testid="loop-run-active-id">
          {run.id}
        </span>
        <span className="min-w-0 truncate text-form-hint text-subtle">
          {loopRunOriginLine(run)}
        </span>
        <Link
          className="ml-auto inline-flex items-center gap-1.5 text-form-hint font-medium text-muted transition-colors hover:text-fg-strong"
          data-testid="loop-run-active-link"
          params={{ runId: run.id }}
          to="/loop-runs/$runId"
        >
          View run
          <ArrowRight aria-hidden="true" className="size-3" />
        </Link>
      </div>
      {note ? <p className="text-form-hint leading-relaxed text-faint">{note}</p> : null}
    </section>
  );
}
