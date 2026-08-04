import { Pause, Play, Square } from "lucide-react";

import { Button } from "@compozy/ui";

import { type LoopRunVerb, loopRunVerbs } from "../../lib/loop-node-controls";

interface LoopRunControlsProps {
  status?: string | null;
  pauseRequested?: boolean;
  /**
   * The one run verb currently in flight. Only one can be — the daemon settles
   * each before the next is offerable — so this is a single value rather than a
   * row of independent booleans that could describe an impossible state.
   */
  pendingVerb?: LoopRunVerb;
  onPause: () => void;
  onResume: () => void;
  onCancel: () => void;
}

/**
 * Operator run controls (§4.4 / ADR-017, VC-R3). The offered verbs come from
 * `loopRunVerbs`, which reads daemon truth: Pause only from `running` (the
 * daemon rejects it elsewhere, N-003), Resume only from `paused`, Cancel for any
 * live run.
 *
 * Cancel stays a first-class button because winding a run down is the ordinary
 * ending. Kill is deliberately NOT here — it is the destructive escape and lives
 * in the surface's single ⋯ overflow (`LoopRunOverflowMenu`, DESIGN-LESSONS
 * L12), so this row never becomes a wall of stop buttons.
 */
export function LoopRunControls({
  status,
  pauseRequested,
  pendingVerb,
  onPause,
  onResume,
  onCancel,
}: LoopRunControlsProps) {
  const verbs = loopRunVerbs(status, Boolean(pauseRequested));
  if (verbs.length === 0) return null;
  const pausing = status === "running" && Boolean(pauseRequested);
  return (
    <div className="flex items-center gap-2" data-testid="loop-run-controls">
      {verbs.includes("pause") ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          data-testid="loop-run-pause"
          disabled={pendingVerb === "pause"}
          onClick={onPause}
        >
          <Pause className="size-3.5" aria-hidden="true" />
          Pause
        </Button>
      ) : null}
      {/* A requested pause has committed but not yet landed at the generation
          boundary; the daemon still reports `running`, so say so rather than
          offering a verb it would now reject. */}
      {pausing ? (
        <Button type="button" variant="outline" size="sm" data-testid="loop-run-pausing" disabled>
          <Pause className="size-3.5" aria-hidden="true" />
          Pausing…
        </Button>
      ) : null}
      {verbs.includes("resume") ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          data-testid="loop-run-resume"
          disabled={pendingVerb === "resume"}
          onClick={onResume}
        >
          <Play className="size-3.5" aria-hidden="true" />
          Resume
        </Button>
      ) : null}
      {verbs.includes("cancel") ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="text-danger hover:border-danger/40 hover:text-danger"
          data-testid="loop-run-cancel"
          disabled={pendingVerb === "cancel"}
          onClick={onCancel}
        >
          <Square className="size-3.5" aria-hidden="true" />
          Cancel run
        </Button>
      ) : null}
    </div>
  );
}
