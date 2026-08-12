import type { LoopNodeInventoryState } from "../types";

export const LOOP_NODE_INVENTORY_STATES = [
  "waiting",
  "quarantined",
  "attention",
  "retrying",
] as const satisfies readonly LoopNodeInventoryState[];

export function isLoopNodeInventoryState(value: unknown): value is LoopNodeInventoryState {
  return (
    typeof value === "string" && (LOOP_NODE_INVENTORY_STATES as readonly string[]).includes(value)
  );
}
