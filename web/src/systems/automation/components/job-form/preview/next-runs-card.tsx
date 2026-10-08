import { Clock } from "lucide-react";

import type { JobNextRun } from "../../../lib/job-preview";
import { AutomationNextRuns } from "../../automation-detail/automation-next-runs";
import { PreviewCard } from "../../automation-form/preview/preview-card";

const WONT_REGISTER_PREFIX = "Won't register";
const EMPTY_CLASS =
  "rounded-md border border-dashed border-line-soft bg-sunken px-3 py-2.5 text-form-hint leading-snug text-subtle";

/** Dashed empty state. The "Won't register" lead clause is emphasized in warning tone. */
function NextRunsEmpty({ reason }: { reason: string }) {
  if (reason.startsWith(WONT_REGISTER_PREFIX)) {
    return (
      <div className={EMPTY_CLASS}>
        <span className="text-warning font-semibold">{WONT_REGISTER_PREFIX}</span>
        {reason.slice(WONT_REGISTER_PREFIX.length)}
      </div>
    );
  }
  return <div className={EMPTY_CLASS}>{reason}</div>;
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
        <div className={EMPTY_CLASS}>Choose Back to form to fix the schedule.</div>
      ) : nextRuns.length === 0 ? (
        <NextRunsEmpty reason={emptyReason ?? ""} />
      ) : (
        <AutomationNextRuns runs={nextRuns} />
      )}
    </PreviewCard>
  );
}
