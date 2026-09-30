import { CircleAlertIcon, GitMergeIcon, ShieldCheckIcon } from "lucide-react";

import { Button, cn } from "@compozy/ui";

import type { WorktreeExitCleanupEvidence } from "../types";

interface WorktreeMergedEvidenceProps {
  cleanup: WorktreeExitCleanupEvidence;
  onCleanUp?: () => void;
  staleLabel?: string;
}

type EvidenceTone = "success" | "info" | "warning" | "neutral";

const ROW_TONE_CLASS: Record<EvidenceTone, string | undefined> = {
  success: "text-fg",
  info: "text-muted",
  warning: "text-muted",
  neutral: undefined,
};

const ICON_TONE_CLASS: Record<EvidenceTone, string | undefined> = {
  success: "text-success",
  info: "text-info",
  warning: "text-warning",
  neutral: undefined,
};

function evidenceTone(cleanup: WorktreeExitCleanupEvidence): EvidenceTone {
  if (cleanup.blocker) return "warning";
  if (cleanup.downgraded) return "info";
  return cleanup.safe ? "success" : "neutral";
}

/** Presence-only data attribute: `""` when set, omitted otherwise. */
function flagAttr(value: unknown): "" | undefined {
  return value ? "" : undefined;
}

function evidenceDataAttrs(cleanup: WorktreeExitCleanupEvidence, tone: EvidenceTone) {
  return {
    "data-blocked": flagAttr(cleanup.blocker),
    "data-downgraded": flagAttr(cleanup.downgraded),
    "data-forge-state": cleanup.forge_state,
    "data-safe": flagAttr(cleanup.safe),
    "data-stale": flagAttr(cleanup.stale),
    "data-slot": "worktree-merged-evidence",
    "data-testid": "worktree-merged-evidence",
    "data-source": cleanup.source,
    "data-tone": tone,
  };
}

function EvidenceIcon({ cleanup }: { cleanup: WorktreeExitCleanupEvidence }) {
  if (cleanup.blocker) return <CircleAlertIcon aria-hidden="true" />;
  if (cleanup.forge_state) return <GitMergeIcon aria-hidden="true" />;
  return <ShieldCheckIcon aria-hidden="true" />;
}

function EvidenceText({
  cleanup,
  staleLabel,
}: {
  cleanup: WorktreeExitCleanupEvidence;
  staleLabel?: string;
}) {
  return (
    <span className="min-w-0 flex-1">
      {cleanup.summary ?? cleanup.forge_state}
      {cleanup.stale && staleLabel ? (
        <span className="ml-1 text-micro text-faint">{`as of ${staleLabel}`}</span>
      ) : null}
      {cleanup.blocker ? (
        <span className="mt-px block text-badge text-subtle" data-slot="worktree-cleanup-blocker">
          {cleanup.blocker}
        </span>
      ) : null}
    </span>
  );
}

/**
 * Why this worktree is — or is not — safe to clean up.
 *
 * The evidence and its blocker are the daemon's; this row states them and lets
 * the operator act only when the daemon says it is safe. A blocker suppresses
 * the action entirely rather than disabling it, so nothing dangles as a control
 * that will never work. A downgraded verdict reads as information, not alarm:
 * the branch still exists on the remote, which is a fact, not a failure.
 */
export function WorktreeMergedEvidence({
  cleanup,
  onCleanUp,
  staleLabel,
}: WorktreeMergedEvidenceProps) {
  if (!cleanup.summary && !cleanup.blocker && !cleanup.forge_state) return null;

  const tone = evidenceTone(cleanup);
  const canCleanUp = cleanup.safe && !cleanup.blocker && onCleanUp;

  return (
    <div
      className={cn(
        "flex items-center gap-2.5 rounded-lg bg-canvas shadow-card px-3.5 py-2.5 text-small-body",
        ROW_TONE_CLASS[tone]
      )}
      {...evidenceDataAttrs(cleanup, tone)}
    >
      <span
        className={cn(
          "grid size-4 shrink-0 place-items-center [&_svg]:size-3",
          ICON_TONE_CLASS[tone]
        )}
      >
        <EvidenceIcon cleanup={cleanup} />
      </span>
      <EvidenceText cleanup={cleanup} staleLabel={staleLabel} />
      {canCleanUp ? (
        <Button onClick={onCleanUp} size="sm" type="button" variant="secondary">
          Clean up
        </Button>
      ) : null}
    </div>
  );
}
