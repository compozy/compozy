import { Eyebrow } from "@agh/ui";

import type { JobPreviewModel } from "../../../lib/job-preview";
import { AutomationRequestPayload } from "../../automation-request-payload";
import { NextRunsCard } from "./next-runs-card";
import { RunDigestCard } from "./run-digest-card";
import { SummaryCard } from "./summary-card";

interface JobPreviewProps {
  preview: JobPreviewModel;
}

/**
 * Live preview: summary, next runs, run digest, request payload.
 *
 * Swaps in for the form body rather than sitting in a permanent rail, so the
 * default read of the editor is the form alone. The enclosing `EntityDialogBody`
 * owns the frame and the scroll.
 */
export function JobPreview({ preview }: JobPreviewProps) {
  return (
    <div className="flex flex-col gap-3" data-testid="job-preview">
      <div className="flex items-center gap-2">
        <span
          aria-hidden="true"
          className={
            preview.targetIssue
              ? "size-1.5 rounded-full bg-warning ring-[3px] ring-warning-tint"
              : "size-1.5 rounded-full bg-success ring-[3px] ring-success-tint"
          }
        />
        <Eyebrow className="text-subtle">
          {preview.targetIssue ? "Cannot save" : "Live preview"}
        </Eyebrow>
      </div>
      <SummaryCard summary={preview.summary} />
      <NextRunsCard emptyReason={preview.nextRunsEmptyReason} nextRuns={preview.nextRuns} />
      <RunDigestCard runDigest={preview.runDigest} />
      <AutomationRequestPayload blockedReason={preview.targetIssue} request={preview.request} />
    </div>
  );
}
