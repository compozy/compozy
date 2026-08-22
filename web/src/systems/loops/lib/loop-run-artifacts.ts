import type { PillTone } from "@compozy/ui";

import type { LoopBriefing, LoopRunArtifact, LoopRunOutcome } from "../types";
import { loopOutcomeLabel } from "./loop-formatters";

/**
 * What a finished run produced, and what became of it.
 *
 * A terminal run leads with its outcome and its outputs, so collecting results
 * takes no forensics. The rule that shapes every branch here: an entry never
 * disappears because its bytes did. Retention pruning removes content, not the
 * fact that the run wrote something — so a pruned artifact keeps its name and
 * says plainly that the content is gone (US-008.EC-3).
 */

export type LoopArtifactAvailability = "available" | "partial" | "pruned";

export interface LoopArtifactRow {
  key: string;
  name: string;
  /** The output node that produced it, when the run recorded one. */
  output: string | null;
  availability: LoopArtifactAvailability;
  /** A short truthful note for anything less than fully available. */
  note: string | null;
  /** Absent when there is nothing left to open. */
  ref: string | null;
  toneForNote: PillTone | null;
}

export interface LoopOutcomeView {
  status: string;
  /** Plain-words outcome label, e.g. "Done", "Canceled". */
  label: string;
  cause: string | null;
  at: string;
  /** Who ended it, on a cancel or a kill. */
  actorLabel: string | null;
}

export interface LoopRunOutcomeModel {
  outcome: LoopOutcomeView | null;
  artifacts: LoopArtifactRow[];
  /** True when the run finished without producing anything — stated, not hidden. */
  producedNothing: boolean;
}

const AVAILABILITY_NOTES: Record<LoopArtifactAvailability, string | null> = {
  available: null,
  // The graph-eng lock spells `partial` out and keeps its coverage numbers
  // wherever they exist; here the coverage lives on the run, not the artifact.
  partial: "Partial — the run ended before this was complete",
  pruned: "Content no longer stored",
};

const AVAILABILITY_TONES: Record<LoopArtifactAvailability, PillTone | null> = {
  available: null,
  partial: "warning",
  pruned: "neutral",
};

function readAvailability(value: string): LoopArtifactAvailability {
  return value === "partial" || value === "pruned" ? value : "available";
}

function actorLabel(outcome: LoopRunOutcome): string | null {
  const ref = outcome.actor_ref?.trim();
  if (ref) return ref;
  const kind = outcome.actor_kind?.trim();
  return kind ? kind : null;
}

function projectArtifact(artifact: LoopRunArtifact, index: number): LoopArtifactRow {
  const availability = readAvailability(artifact.availability);
  const output = artifact.output?.trim();
  const ref = artifact.ref?.trim();
  return {
    key: `${artifact.name}:${index}`,
    name: artifact.name,
    output: output ? output : null,
    availability,
    note: AVAILABILITY_NOTES[availability],
    // A pruned blob has nothing behind it. A dead link would be worse than none.
    ref: availability === "pruned" || !ref ? null : ref,
    toneForNote: AVAILABILITY_TONES[availability],
  };
}

export function buildRunOutcome(briefing: LoopBriefing): LoopRunOutcomeModel {
  const artifacts = briefing.artifacts.map(projectArtifact);
  const outcome = briefing.outcome ?? null;
  return {
    outcome: outcome
      ? {
          status: outcome.status,
          label: loopOutcomeLabel(outcome.status),
          cause: outcome.cause?.trim() ? outcome.cause : null,
          at: outcome.at,
          actorLabel: actorLabel(outcome),
        }
      : null,
    artifacts,
    // Only a terminal run can be said to have produced nothing; a live one has
    // simply not produced anything yet, which is a different sentence.
    producedNothing: outcome !== null && artifacts.length === 0,
  };
}
