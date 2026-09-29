import { useState } from "react";

import { cn, Eyebrow, TranscriptDisclosure } from "@compozy/ui";

import type { GoalComposerAffordance, SessionGoalSnapshot } from "./goal-status-types";
import { SessionGoalStripBody } from "./session-goal-strip-body";

export interface SessionGoalStripProps {
  snapshot: SessionGoalSnapshot;
  composerAffordance?: GoalComposerAffordance;
  onPrefillComposer?: (text: string) => void;
}

type GoalStripState = "active" | "paused" | "blocked" | "done" | "moved";

// Dot tones per the strip contract: accent pulse active · warning
// blocked/paused · success done · faint moved. Every non-canonical status
// (usage-limited, budget-limited, unknown) reads as blocked.
function goalStripState(snapshot: SessionGoalSnapshot): GoalStripState {
  if (snapshot.bound_session_id !== snapshot.origin_session_id) return "moved";
  switch (snapshot.status) {
    case "active":
      return "active";
    case "paused":
      return "paused";
    case "complete":
      return "done";
    default:
      return "blocked";
  }
}

const STATE_DOT: Record<GoalStripState, string> = {
  active: "bg-accent session-state-pulse",
  paused: "bg-warning",
  blocked: "bg-warning",
  done: "bg-success",
  moved: "bg-faint",
};

/**
 * The goal as one quiet line above the transcript (`.goalstrip`): state dot +
 * GOAL kicker + objective + a plain "Step a of b" fact + chevron, expanding to
 * key/value rows (context usage lives in the composer ring and the body; the
 * node id and cause ride on data attributes). No pills, no meters, no control bar — lifecycle actions
 * live on the window head; the body's Actions row only stages `/goal` commands
 * into the composer, never sends.
 */
export function SessionGoalStrip({
  snapshot,
  composerAffordance,
  onPrefillComposer,
}: SessionGoalStripProps) {
  const [open, setOpen] = useState(false);
  const state = goalStripState(snapshot);
  const moved = state === "moved";
  const facts = `Step ${snapshot.turns_used} of ${snapshot.turn_limit}`;

  return (
    <section
      aria-label="Goal status"
      data-testid="session-goal-strip"
      data-state={state}
      data-goal-status={snapshot.status}
      data-run-status={snapshot.run_status}
      data-live={snapshot.live ? "true" : "false"}
      data-goal-node={snapshot.node_id}
      data-goal-cause={snapshot.cause || undefined}
      className="min-w-0 border-b border-line py-1.5"
    >
      <TranscriptDisclosure
        className="w-full min-w-0 gap-2 font-normal"
        data-testid="goal-strip-line"
        expanded={open}
        icon={
          <span
            aria-hidden="true"
            data-testid="goal-strip-dot"
            className={cn("size-1.5 shrink-0 rounded-full", STATE_DOT[state])}
          />
        }
        label={
          <span className="flex min-w-0 items-center gap-2">
            <Eyebrow className="shrink-0 text-subtle">Goal</Eyebrow>
            <span className="min-w-0 max-w-sm truncate text-small-body text-fg">
              {snapshot.objective}
            </span>
          </span>
        }
        onToggle={() => setOpen(value => !value)}
        trailing={
          <span
            aria-atomic="true"
            aria-live="polite"
            data-testid="goal-strip-facts"
            className="ml-auto shrink-0 text-transcript-meta text-subtle tabular-nums"
          >
            {moved ? `Moved · ${facts}` : facts}
          </span>
        }
      />
      {open ? (
        <SessionGoalStripBody
          snapshot={snapshot}
          moved={moved}
          composerAffordance={composerAffordance}
          onPrefillComposer={onPrefillComposer}
        />
      ) : null}
    </section>
  );
}
