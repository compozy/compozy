import type { PillTone } from "@agh/ui";

import type { KnowledgeTone } from "../lib/knowledge-formatters";
import type { MemoryDecisionOp, MemoryDecisionSource } from "../types";

/**
 * Scope-only tone mapper (retune dropped accent on memory types;
 * type tones now flow through `KNOWLEDGE_TYPE_TONE`). Any future tone leak
 * back into types should add a new entry to `KNOWLEDGE_TYPE_TONE` rather than
 * here.
 */
export function pillToneFromKnowledgeTone(tone: KnowledgeTone): PillTone {
  switch (tone) {
    case "workspace":
    case "reference":
      return "info";
    case "agent":
      return "warning";
    case "project":
      return "info";
    case "global":
    case "user":
    case "feedback":
    default:
      return "neutral";
  }
}

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

export function pillToneFromDecisionSource(source: MemoryDecisionSource): PillTone {
  return source === "llm" ? "info" : "neutral";
}
