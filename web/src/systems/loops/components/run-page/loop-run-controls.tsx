import { Ban, Pause, Play } from "lucide-react";

import { Button, cn } from "@compozy/ui";

import { type LoopRunVerb, loopRunVerbs } from "../../lib/loop-node-controls";

interface LoopRunControlsProps extends React.ComponentProps<"div"> {
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

export function LoopRunControls({
  status,
  pauseRequested,
  pendingVerb,
  onPause,
  onResume,
  onCancel,
  className,
  ...props
}: LoopRunControlsProps) {
  const verbs = loopRunVerbs(status, Boolean(pauseRequested));
  if (verbs.length === 0) return null;
  const pausing = status === "running" && Boolean(pauseRequested);
  return (
    <div
      className={cn("flex items-center gap-2", className)}
      data-testid="loop-run-controls"
      {...props}
    >
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
          variant="destructive"
          size="sm"
          data-testid="loop-run-cancel"
          disabled={pendingVerb === "cancel"}
          onClick={onCancel}
        >
          <Ban className="size-3.5" aria-hidden="true" />
          Cancel run
        </Button>
      ) : null}
    </div>
  );
}
