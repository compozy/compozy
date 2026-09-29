import type { PillTone } from "@compozy/ui";

import type { MemoryDecisionOp } from "../types";

/**
 * Decision ops are the only knowledge signal that carries a tone; scopes,
 * tiers, and types render neutral because they are not states.
 */
export function pillToneFromDecisionOp(op: MemoryDecisionOp): PillTone {
  switch (op) {
    case "add":
      return "success";
    case "update":
      return "info";
    case "delete":
      return "danger";
    case "reject":
      return "warning";
    case "noop":
    default:
      return "neutral";
  }
}
