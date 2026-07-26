import { Pause, Play, Square } from "lucide-react";

import { Button } from "@agh/ui";

import { isTerminalLoopStatus } from "../../lib/loop-formatters";

interface LoopRunControlsProps {
  status?: string | null;
  pauseRequested?: boolean;
  isPausePending?: boolean;
  isResumePending?: boolean;
  isStopPending?: boolean;
  onPause: () => void;
  onResume: () => void;
  onStop: () => void;
}

/**
 * Operator run controls (§4.4 / ADR-017): Pause is valid ONLY from `running` (the
 * daemon rejects it elsewhere, N-003), so it hides outside that state; a pending pause
 * request shows "Pausing…" until the generation boundary flips the run to `paused`,
 * where Resume takes over. Stop is offered for any live (non-terminal) run. Runtime
 * truth wins — the buttons request a transition, they never simulate one.
 */
export function LoopRunControls({
  status,
  pauseRequested,
  isPausePending,
  isResumePending,
  isStopPending,
  onPause,
  onResume,
  onStop,
}: LoopRunControlsProps) {
  const terminal = isTerminalLoopStatus(status);
  if (terminal) return null;
  const isRunning = status === "running";
  const isPaused = status === "paused";
  const pausing = isRunning && Boolean(pauseRequested);
  return (
    <div className="flex items-center gap-2" data-testid="loop-run-controls">
      {isRunning ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          data-testid="loop-run-pause"
          disabled={isPausePending || pausing}
          onClick={onPause}
        >
          <Pause className="size-3.5" aria-hidden="true" />
          {pausing ? "Pausing…" : "Pause"}
        </Button>
      ) : null}
      {isPaused ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          data-testid="loop-run-resume"
          disabled={isResumePending}
          onClick={onResume}
        >
          <Play className="size-3.5" aria-hidden="true" />
          Resume
        </Button>
      ) : null}
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="text-danger hover:border-danger/40 hover:text-danger"
        data-testid="loop-run-stop"
        disabled={isStopPending}
        onClick={onStop}
      >
        <Square className="size-3.5" aria-hidden="true" />
        Stop run
      </Button>
    </div>
  );
}
