import { CircleAlertIcon, GitMergeIcon, ShieldCheckIcon } from "lucide-react";

import { Button, cn } from "@compozy/ui";

import type { WorktreeExitCleanupEvidence } from "../types";

interface WorktreeMergedEvidenceProps {
  cleanup: WorktreeExitCleanupEvidence;
  onCleanUp?: () => void;
  staleLabel?: string;
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

  const tone = cleanup.blocker
    ? "warning"
    : cleanup.downgraded
      ? "info"
      : cleanup.safe
        ? "success"
        : "neutral";

  return (
    <div
      className={cn(
        "flex items-center gap-2.5 rounded-lg border border-line bg-canvas-soft px-3.5 py-2.5 text-small-body",
        tone === "success" && "text-fg",
        tone === "info" && "text-muted",
        tone === "warning" && "text-muted"
      )}
      data-blocked={cleanup.blocker ? "" : undefined}
      data-downgraded={cleanup.downgraded ? "" : undefined}
      data-forge-state={cleanup.forge_state}
      data-safe={cleanup.safe ? "" : undefined}
      data-stale={cleanup.stale ? "" : undefined}
      data-slot="worktree-merged-evidence"
      data-testid="worktree-merged-evidence"
      data-source={cleanup.source}
      data-tone={tone}
    >
      <span
        className={cn(
          "grid size-4 shrink-0 place-items-center [&_svg]:size-3",
          tone === "success" && "text-success",
          tone === "info" && "text-info",
          tone === "warning" && "text-warning"
        )}
      >
        {cleanup.blocker ? (
          <CircleAlertIcon aria-hidden="true" />
        ) : cleanup.forge_state ? (
          <GitMergeIcon aria-hidden="true" />
        ) : (
          <ShieldCheckIcon aria-hidden="true" />
        )}
      </span>
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
      {cleanup.safe && !cleanup.blocker && onCleanUp ? (
        <Button onClick={onCleanUp} size="sm" type="button" variant="outline">
          Clean up
        </Button>
      ) : null}
    </div>
  );
}
