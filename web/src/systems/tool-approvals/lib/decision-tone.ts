import type { PillTone } from "@agh/ui";

import type { ToolApprovalDecision } from "../types";

/**
 * Typed decision → signal-tone dictionary (DESIGN.md §2: route status colors through a
 * typed map, never an inline switch at the feature boundary). `allow` reads as success,
 * `reject` as danger.
 */
const DECISION_TONE: Record<ToolApprovalDecision, PillTone> = {
  allow: "success",
  reject: "danger",
};

export function toolApprovalDecisionTone(decision: ToolApprovalDecision): PillTone {
  return DECISION_TONE[decision];
}
