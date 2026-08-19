export { LOOP_ENTITY_KINDS } from "@/generated/loop-enums";
import { LOOP_ENTITY_KINDS } from "@/generated/loop-enums";

export type LoopEntityKind = (typeof LOOP_ENTITY_KINDS)[number];

export function isLoopEntityKind(value: unknown): value is LoopEntityKind {
  return typeof value === "string" && (LOOP_ENTITY_KINDS as readonly string[]).includes(value);
}
