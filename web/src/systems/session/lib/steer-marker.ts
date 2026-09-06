// Steer markers (VC-07): how the operator's message reached the turn, read from
// the daemon's own transcript markers — never inferred from the bubble.

import { CornerDownRight, ListPlus, Scissors } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import type { TranscriptMarkerPayload } from "../types";

export const PROMPT_STEERED_MARKER = "transcript_marker.prompt_steered";
export const PROMPT_SUPERSEDED_MARKER = "transcript_marker.prompt_superseded";
export const PROMPT_ACCEPTED_MARKER = "transcript_marker.prompt_accepted";

export type SteerMarkerKind =
  | "injected"
  | "pending_injection"
  | "interrupt_fallback"
  | "superseded"
  | "queued";

export interface SteerMarkerView {
  kind: SteerMarkerKind;
  icon: LucideIcon;
  text: string;
  /** Trailing meta ("was #2") for a prompt that came from the queue. */
  meta: string | null;
}

const META_TEXT: Record<SteerMarkerKind, string> = {
  injected: "Steered — delivered into the live turn",
  pending_injection: "Steered — the agent sees it when the current tool finishes",
  interrupt_fallback: "Steered — interrupted and replaced",
  superseded: "Superseded by your next steer",
  queued: "From the queue",
};

/** The words of the meta line for a steer kind (VC-07). */
export function steerMetaText(kind: SteerMarkerKind): string {
  return META_TEXT[kind];
}

/** The verb glyph of the meta line: steer, interrupt-and-replace, or the queue. */
export function steerMetaGlyph(kind: SteerMarkerKind): LucideIcon {
  if (kind === "interrupt_fallback") return Scissors;
  if (kind === "queued") return ListPlus;
  return CornerDownRight;
}

function evidenceString(marker: TranscriptMarkerPayload, key: string): string | null {
  const value = marker.evidence?.[key];
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
}

/**
 * How the operator's message reached the turn (VC-07), from the daemon's own
 * markers: a steer delivered live, one the agent sees when the current tool
 * finishes, one that fell back to interrupt-and-replace, one superseded by a
 * later steer (never removed, US-001.EC-3), and a prompt dispatched from the
 * queue with the position it held. Anything else is not a steer marker.
 */
export function steerMarkerView(marker: TranscriptMarkerPayload): SteerMarkerView | null {
  let kind: SteerMarkerKind;
  let meta: string | null = null;
  switch (marker.kind) {
    case PROMPT_STEERED_MARKER: {
      const delivery = evidenceString(marker, "steer_delivery");
      kind =
        delivery === "interrupt_fallback"
          ? "interrupt_fallback"
          : delivery === "pending_injection"
            ? "pending_injection"
            : "injected";
      break;
    }
    case PROMPT_SUPERSEDED_MARKER:
      kind = "superseded";
      break;
    case PROMPT_ACCEPTED_MARKER: {
      // The position is shown only when the daemon recorded one; never invented.
      const position = evidenceString(marker, "queue_position");
      kind = "queued";
      meta = position === null ? null : `was #${position}`;
      break;
    }
    default:
      return null;
  }
  return { kind, icon: steerMetaGlyph(kind), text: steerMetaText(kind), meta };
}
