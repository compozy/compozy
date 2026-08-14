import { GitMerge, TriangleAlert } from "lucide-react";

import { cn, Eyebrow, Icon, PillDot, StatusDot } from "@compozy/ui";

/**
 * Row signals. Every part returns `null` when its fact is unknown — an absent
 * signal means "we do not know", and rendering `+0 −0` or "clean" for a NULL
 * would be a lie about the working tree.
 */

export interface WorktreeDirtySignalProps {
  /** `null` = never read. `true` with no counts = dirty, magnitude unknown. */
  dirty: boolean | null;
  insertions?: number | null;
  deletions?: number | null;
  changedFiles?: number | null;
}

export function WorktreeDirtySignal({
  dirty,
  insertions,
  deletions,
  changedFiles,
}: WorktreeDirtySignalProps) {
  if (dirty !== true) return null;

  const hasCounts = typeof insertions === "number" && typeof deletions === "number";
  return (
    <span
      data-slot="worktree-dirty"
      className="inline-flex shrink-0 items-center gap-[5px] font-mono text-mono-id tabular-nums"
    >
      {hasCounts ? (
        <>
          <span className="text-success">+{insertions}</span>
          <span className="text-danger">−{deletions}</span>
        </>
      ) : (
        // The list payload carries only a boolean; claiming a magnitude here
        // would invent numbers the daemon never sent.
        <span className="text-warning">uncommitted</span>
      )}
      {typeof changedFiles === "number" ? (
        <span className="text-subtle">· {changedFiles} files</span>
      ) : null}
    </span>
  );
}

export interface WorktreeAheadBehindSignalProps {
  ahead: number | null;
  behind: number | null;
}

/** Renders only against a known upstream — nullable counts hide, never zero. */
export function WorktreeAheadBehindSignal({ ahead, behind }: WorktreeAheadBehindSignalProps) {
  const showAhead = typeof ahead === "number" && ahead > 0;
  const showBehind = typeof behind === "number" && behind > 0;
  if (!showAhead && !showBehind) return null;

  return (
    <span
      data-slot="worktree-ahead-behind"
      className="inline-flex shrink-0 items-center gap-1.5 font-mono text-mono-id tabular-nums text-subtle"
    >
      {showAhead ? (
        <span>
          <span aria-hidden="true" className="text-faint">
            ↑
          </span>
          <span className="sr-only">ahead by </span>
          {ahead}
        </span>
      ) : null}
      {showBehind ? (
        // Behind is action-needed, so it takes the warning tone rather than
        // reading as neutral metadata.
        <span className="text-warning" data-hot="true">
          <span aria-hidden="true">↓</span>
          <span className="sr-only">behind by </span>
          {behind}
        </span>
      ) : null}
    </span>
  );
}

export type WorktreeAgentActivity = "running" | "awaiting_input" | "idle" | string;

export interface WorktreeAgentSignalProps {
  activity: WorktreeAgentActivity;
  /** Session title + provider, surfaced as the hover affordance. */
  title?: string;
  /** Nested rows show the dot alone; full rows may show the label. */
  showLabel?: boolean;
}

/** Idle renders nothing in rows — silence is the idle signal. */
export function WorktreeAgentSignal({ activity, title, showLabel }: WorktreeAgentSignalProps) {
  if (activity !== "running" && activity !== "awaiting_input") return null;

  const label = activity === "running" ? "agent running" : "awaiting input";
  return (
    <span
      data-slot="worktree-agent"
      data-activity={activity}
      title={title}
      className="inline-flex shrink-0 items-center gap-1.5 text-mono-id text-subtle"
    >
      {activity === "running" ? (
        <PillDot tone="accent" pulse size="sm" />
      ) : (
        // Hollow, unpulsed: the turn finished and the user owns the next move.
        <StatusDot tone="accent" variant="ring" size="sm" label={label} />
      )}
      {showLabel ? label : <span className="sr-only">{label}</span>}
    </span>
  );
}

/** Run-created provenance. Absent for manually created and adopted worktrees. */
export function WorktreeOriginSignal({ origin }: { origin: string }) {
  if (origin !== "per_run") return null;

  return (
    <span
      data-slot="worktree-origin"
      className="inline-flex h-4 shrink-0 items-center rounded-sm border border-line px-1.5"
    >
      <Eyebrow className="text-subtle">per-run</Eyebrow>
    </span>
  );
}

export interface WorktreeSetupFlagSignalProps {
  setupState: string;
  setupError?: string;
  /** Nested rows are icon-only; the reason survives as the accessible name. */
  iconOnly?: boolean;
}

/** A flagged worktree is usable — the flag reports provisioning, not health. */
export function WorktreeSetupFlagSignal({
  setupState,
  setupError,
  iconOnly,
}: WorktreeSetupFlagSignalProps) {
  if (setupState !== "failed") return null;

  const reason = setupError?.trim()
    ? `Setup failed — ${setupError.trim()}; worktree is usable`
    : "Setup failed; worktree is usable";
  return (
    <span
      data-slot="worktree-setup-flag"
      title={reason}
      aria-label={iconOnly ? reason : undefined}
      className="inline-flex shrink-0 items-center gap-1 text-mono-id font-medium text-warning"
    >
      <Icon as={TriangleAlert} size="xs" />
      {iconOnly ? null : "setup failed"}
    </span>
  );
}

export function WorktreeMergedSignal({ merged }: { merged: boolean | null | undefined }) {
  if (merged !== true) return null;

  return (
    <span
      data-slot="worktree-merged"
      className="inline-flex shrink-0 items-center gap-1 text-mono-id font-semibold text-success"
    >
      <Icon as={GitMerge} size="xs" />
      Merged
    </span>
  );
}

/** Stamps every remote-derived value so freshness is never implied. */
export function WorktreeStaleSignal({ label }: { label: string | null | undefined }) {
  if (!label?.trim()) return null;

  return (
    <span data-slot="worktree-stale" className="shrink-0 font-mono text-micro text-faint">
      as of {label}
    </span>
  );
}

/**
 * A detached HEAD has no branch. Stating the pin is the only truthful rendering
 * — inventing a branch name here is the failure US-012 EC-1 guards against.
 */
export function WorktreeDetachedPin({ sha, className }: { sha: string; className?: string }) {
  const shortSha = sha.trim().slice(0, 7);
  if (shortSha === "") return null;

  return (
    <span
      data-slot="worktree-detached-pin"
      className={cn("font-mono text-mono-id text-subtle", className)}
    >
      pinned at {shortSha}
    </span>
  );
}
