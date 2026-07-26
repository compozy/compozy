import type { PillTone } from "@agh/ui";

import type { MemoryType } from "@/systems/knowledge/types";

/**
 * / §16: knowledge memory-type tone vocabulary.
 *
 * `KnowledgeType` is the forward-looking dictionary key set tracked by the
 * design system; the backend `MemoryType` enum (`user | feedback | project |
 * reference`) is mapped onto it via {@link knowledgeTypeFor}. `archival` falls
 * outside the runtime PillTone vocabulary and resolves to a `"faint"` literal
 * so callers can branch on muted chrome without re-using the `neutral` tone.
 */
export type KnowledgeTypeTone = PillTone | "faint";

export type KnowledgeType = "decisions" | "code" | "notes" | "runbooks" | "archival";

export const KNOWLEDGE_TYPE_TONE = {
  decisions: "info",
  code: "neutral",
  notes: "neutral",
  runbooks: "warning",
  archival: "faint",
} as const satisfies Record<KnowledgeType, KnowledgeTypeTone>;

const MEMORY_TYPE_TO_KNOWLEDGE_TYPE: Record<MemoryType, KnowledgeType> = {
  project: "decisions",
  reference: "code",
  user: "notes",
  feedback: "notes",
};

/**
 * Maps the wire-format {@link MemoryType} onto the {@link KnowledgeType}
 * vocabulary so consumers can look the type tone up directly via
 * `KNOWLEDGE_TYPE_TONE[knowledgeTypeFor(memory.type)]`.
 */
export function knowledgeTypeFor(type: MemoryType): KnowledgeType {
  return MEMORY_TYPE_TO_KNOWLEDGE_TYPE[type];
}
