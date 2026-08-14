import { CheckIcon, CircleAlertIcon, MinusIcon } from "lucide-react";

import { Spinner, cn } from "@compozy/ui";

import type {
  WorktreeExitProgressPhase,
  WorktreeExitPhaseState,
} from "../lib/worktree-exit-progress";
import type { WorktreeExitPhase } from "../lib/worktree-events";

interface WorktreePhaseStepsProps {
  phases: readonly WorktreeExitProgressPhase[];
}

/** Present tense while a phase runs, past tense once it has. */
const PHASE_LABEL: Record<WorktreeExitPhase, { active: string; done: string }> = {
  commit: { active: "Committing", done: "Committed" },
  push: { active: "Pushing", done: "Pushed" },
  pr: { active: "Opening pull request", done: "Opened pull request" },
};

const STAGE: Record<WorktreeExitPhaseState, string> = {
  pending: "pending",
  running: "active",
  completed: "done",
  skipped: "skip",
  failed: "fail",
};

function PhaseGlyph({ state }: { state: WorktreeExitPhaseState }) {
  if (state === "running") return <Spinner className="size-3" />;
  if (state === "completed") return <CheckIcon aria-hidden="true" className="size-3" />;
  if (state === "failed") return <CircleAlertIcon aria-hidden="true" className="size-3" />;
  if (state === "skipped") return <MinusIcon aria-hidden="true" className="size-3" />;
  return <span aria-hidden="true" className="size-1.5 rounded-full bg-line-strong" />;
}

/**
 * The phases of an exit action, announced up front.
 *
 * Every phase the daemon declared is listed from the first frame, so the
 * operator sees the whole sequence rather than discovering it one step at a
 * time. A phase that did no work says why in the daemon's own words.
 */
export function WorktreePhaseSteps({ phases }: WorktreePhaseStepsProps) {
  return (
    <ol className="flex flex-col gap-1" data-slot="worktree-phase-steps">
      {phases.map(phase => {
        const label = PHASE_LABEL[phase.phase];
        const done = phase.state === "completed" || phase.state === "skipped";
        return (
          <li
            className={cn(
              "flex items-center gap-2 text-badge text-muted",
              phase.state === "pending" && "text-faint",
              phase.state === "failed" && "text-danger",
              phase.state === "completed" && "text-fg"
            )}
            data-phase={phase.phase}
            data-slot="worktree-phase-step"
            data-stage={STAGE[phase.state]}
            key={phase.phase}
          >
            <span className="grid size-3.5 shrink-0 place-items-center">
              <PhaseGlyph state={phase.state} />
            </span>
            <span>{done ? label.done : label.active}</span>
            {phase.reason ? (
              <span className="text-subtle" data-slot="worktree-phase-reason">
                {phase.reason}
              </span>
            ) : null}
          </li>
        );
      })}
    </ol>
  );
}
