import type { JobPreviewSummary } from "../../../lib/job-preview";
import { PreviewCard } from "../../trigger-form/preview/preview-card";

interface SummaryCardProps {
  summary: JobPreviewSummary;
}

/** Plain-language one-line summary of the job's schedule, output, and scope. */
export function SummaryCard({ summary }: SummaryCardProps) {
  return (
    <PreviewCard label="Summary">
      <p className="text-small-body leading-relaxed text-muted">
        {summary.output === "agent" ? (
          <>
            {summary.scheduleLabel} UTC, run agent{" "}
            <span className="text-accent-strong font-semibold">{summary.agentName}</span>{" "}
            {summary.scopeLabel}.
          </>
        ) : summary.output === "loop" ? (
          <>
            {summary.scheduleLabel} UTC, start Loop{" "}
            <span className="text-accent-strong font-semibold">
              {summary.loopTarget?.loopName || "not selected"}
            </span>{" "}
            {summary.scopeLabel}.
          </>
        ) : (
          <>
            {summary.scheduleLabel} UTC, materialize a{" "}
            <span className="text-fg-strong font-semibold">task</span>{" "}
            {summary.ownerLabel ? (
              <>
                owned by{" "}
                <span className="text-accent-strong font-semibold">{summary.ownerLabel}</span>
              </>
            ) : (
              "for any available owner"
            )}{" "}
            {summary.scopeLabel}.
          </>
        )}
      </p>
    </PreviewCard>
  );
}
