import { Button } from "@compozy/ui";

import { cn } from "@compozy/ui";

import type { WorktreeExitHookChunk, WorktreeExitProgress } from "../lib/worktree-exit-progress";
import { WorktreePhaseSteps } from "./worktree-phase-steps";

interface WorktreeExitProgressProps {
  progress: WorktreeExitProgress;
  onCancel?: () => void;
  onDismiss?: () => void;
  /** The daemon's single follow-up action, when it supplied one. */
  onCta?: () => void;
}

/**
 * Renders hook output as it streams.
 *
 * Chunks arrive already redacted and are shown in arrival order, so the block
 * is a transcript of what the hook printed rather than a summary of it. stderr
 * is marked so a warning is not mistaken for ordinary output.
 */
function HookChunks({ chunks }: { chunks: readonly WorktreeExitHookChunk[] }) {
  return (
    <pre
      aria-live="polite"
      className="max-h-24 overflow-y-auto rounded-md bg-rail px-2.5 py-2 font-mono text-micro leading-[1.6] whitespace-pre-wrap text-subtle"
      data-slot="worktree-exit-hook-stream"
    >
      {chunks.map((chunk, index) => (
        <span
          className={cn(chunk.stream === "stderr" && "text-danger")}
          data-stream={chunk.stream}
          key={`${chunk.stream}-${index}`}
        >
          {chunk.text}
        </span>
      ))}
    </pre>
  );
}

/**
 * One updating report for one exit operation.
 *
 * Hook output streams in while a phase runs and stays visible after it ends;
 * the captured command output arrives with the phase's terminal frame. A
 * finished operation offers exactly one next step, named by the daemon.
 */
export function WorktreeExitProgress({
  progress,
  onCancel,
  onDismiss,
  onCta,
}: WorktreeExitProgressProps) {
  const failed = progress.phases.find(phase => phase.state === "failed");
  const lastWithOutput = [...progress.phases].reverse().find(phase => phase.output);
  const output = failed?.output ?? lastWithOutput?.output;
  // The phase currently printing, or the last one that printed anything.
  const streaming =
    progress.phases.find(phase => phase.state === "running" && phase.chunks?.length) ??
    [...progress.phases].reverse().find(phase => phase.chunks?.length);

  return (
    <div
      className="flex min-w-72 flex-col gap-2"
      data-slot="worktree-exit-progress"
      data-status={progress.status}
    >
      <WorktreePhaseSteps phases={progress.phases} />
      {streaming?.chunks?.length ? <HookChunks chunks={streaming.chunks} /> : null}
      {progress.message ? (
        <p className="text-badge leading-[1.45] text-danger" data-slot="worktree-exit-message">
          {progress.message}
        </p>
      ) : null}
      {output ? (
        <pre
          className="max-h-24 overflow-y-auto rounded-md bg-rail px-2.5 py-2 font-mono text-micro leading-[1.6] whitespace-pre-wrap text-subtle"
          data-slot="worktree-exit-output"
        >
          {output}
        </pre>
      ) : null}
      <div className="flex items-center gap-2">
        {progress.status === "running" && onCancel ? (
          <Button onClick={onCancel} size="sm" type="button" variant="ghost">
            Cancel
          </Button>
        ) : null}
        {progress.status !== "running" && progress.cta ? (
          <Button
            onClick={() => {
              if (progress.cta?.url) window.open(progress.cta.url, "_blank", "noreferrer");
              else onCta?.();
            }}
            size="sm"
            type="button"
          >
            {progress.cta.label}
          </Button>
        ) : null}
        {progress.status !== "running" && onDismiss ? (
          <Button onClick={onDismiss} size="sm" type="button" variant="ghost">
            Dismiss
          </Button>
        ) : null}
      </div>
    </div>
  );
}
