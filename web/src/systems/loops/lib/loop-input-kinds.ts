export const LOOP_ENTITY_KINDS = [
  "agent",
  "skill",
  "loop",
  "worktree",
  "session",
  "workspace",
  "secret",
] as const;

export type LoopEntityKind = (typeof LOOP_ENTITY_KINDS)[number];

export function isLoopEntityKind(value: unknown): value is LoopEntityKind {
  return typeof value === "string" && (LOOP_ENTITY_KINDS as readonly string[]).includes(value);
}
