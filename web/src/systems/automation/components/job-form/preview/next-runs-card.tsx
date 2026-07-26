import { Clock } from "lucide-react";

import { cn } from "@agh/ui";

import type { JobNextRun } from "../../../lib/job-preview";
import { PreviewCard } from "../../trigger-form/preview/preview-card";

const WONT_REGISTER_PREFIX = "Won't register";

interface NextRunsEmptyProps {
  reason: string;
}

/** Dashed empty state. The "Won't register" lead clause is emphasized in warning tone. */
function NextRunsEmpty({ reason }: NextRunsEmptyProps) {
  if (reason.startsWith(WONT_REGISTER_PREFIX)) {
    return (
      <div className="rounded-md border border-dashed border-line-soft bg-canvas-soft px-3 py-2.5 text-form-hint leading-snug text-subtle">
        <span className="text-warning font-semibold">{WONT_REGISTER_PREFIX}</span>
        {reason.slice(WONT_REGISTER_PREFIX.length)}
      </div>
    );
  }
  return (
    <div className="rounded-md border border-dashed border-line-soft bg-canvas-soft px-3 py-2.5 text-form-hint leading-snug text-subtle">
      {reason}
    </div>
  );
}

interface NextRunRowProps {
  run: JobNextRun;
}

/** A single upcoming fire time: index chip + relative label + absolute UTC stamp. */
function NextRunRow({ run }: NextRunRowProps) {
  return (
    <div className="flex items-center gap-3">
      <span
        className={cn(
          "grid size-[18px] flex-none place-items-center rounded-full font-mono text-badge font-semibold",
          run.isFirst ? "bg-accent-tint-strong text-accent-strong" : "bg-badge-fill text-subtle"
        )}
      >
        {run.index}
      </span>
      <span
        className={cn(
          "flex-1 text-small-body font-medium",
          run.isFirst ? "text-fg-strong" : "text-fg"
        )}
      >
        {run.relative}
        {run.oneTime ? " · one-time" : ""}
      </span>
      <span className="font-mono text-form-hint text-subtle tabular-nums">{run.absolute}</span>
    </div>
  );
}

interface NextRunsCardProps {
  nextRuns: JobNextRun[] | null;
  emptyReason: string | null;
}

/** Live "next runs" list: invalid schedule, empty-but-valid reason, or the fire-time rows. */
export function NextRunsCard({ nextRuns, emptyReason }: NextRunsCardProps) {
  return (
    <PreviewCard
      icon={Clock}
      label="Next runs"
      right={<span className="font-mono text-form-hint text-subtle">UTC</span>}
    >
      {nextRuns === null ? (
        <div className="rounded-md border border-dashed border-line-soft bg-canvas-soft px-3 py-2.5 text-form-hint leading-snug text-subtle">
          Fix the schedule above to preview fire times.
        </div>
      ) : nextRuns.length === 0 ? (
        <NextRunsEmpty reason={emptyReason ?? ""} />
      ) : (
        <div className="flex flex-col gap-1.5">
          {nextRuns.map(run => (
            <NextRunRow key={run.index} run={run} />
          ))}
        </div>
      )}
    </PreviewCard>
  );
}
