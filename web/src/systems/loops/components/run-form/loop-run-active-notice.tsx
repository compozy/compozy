import { Link } from "@tanstack/react-router";
import { Activity, ArrowRight } from "lucide-react";

import { buttonVariants, cn, formatRelativeTime } from "@compozy/ui";

import { loopRunOriginLine } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { LoopRailSection } from "../loop-rail-section";
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
function concurrencyNote(concurrency: string | undefined): string | null {
  if (concurrency === "forbid") {
    return "This Loop runs one at a time. Cancel the current run from its page, or let it finish, before starting another.";
  }
  if (concurrency === "queue") return "Starting another run makes it wait for the current one.";
  if (concurrency === "allow") return "Starting another run leaves this one running.";
  return null;
}

/**
 * The run of this loop that is already live when the form opens.
 *
 * Informative only: the run's own page owns pause/resume/cancel, so this notice
 * links there instead of duplicating controls that would need their own confirmation
 * and lifecycle truth.
 */
export function LoopRunActiveNotice({ run, concurrency }: LoopRunActiveNoticeProps) {
  const note = concurrencyNote(concurrency);
  return (
    <LoopRailSection
      data-testid="loop-run-active-notice"
      defaultOpen
      gist={`Started ${formatRelativeTime(run.created_at)}`}
      icon={<Activity aria-hidden="true" className="size-3.5" />}
      title="Already running"
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3">
        <LoopStatusPill status={run.status} />
        <span
          className="min-w-0 truncate text-form-hint text-subtle"
          data-testid="loop-run-active-id"
          title={run.id}
        >
          {loopRunOriginLine(run)}
        </span>
        <Link
          className={cn(buttonVariants({ size: "sm", variant: "ghost" }), "ml-auto")}
          data-testid="loop-run-active-link"
          params={{ runId: run.id }}
          to="/loop-runs/$runId"
        >
          View run
          <ArrowRight aria-hidden="true" className="size-3" />
        </Link>
      </div>
      {note ? (
        <p className="border-t border-line-soft px-4 py-3 text-form-hint leading-relaxed text-faint">
          {note}
        </p>
      ) : null}
    </LoopRailSection>
  );
}
