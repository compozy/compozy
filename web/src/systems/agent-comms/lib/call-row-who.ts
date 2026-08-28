/**
 * The human name on a Calls inspector row.
 *
 * Direction is a server filter, not a field. Made names who was asked; Received
 * names who asked. A session ULID is an id, not a name — it belongs on the
 * record, never on the rail.
 */
import type { CallPayload } from "../types";

export function callRowWho(
  call: CallPayload,
  direction: "made" | "received",
  callerName?: string
): string {
  if (direction === "made") {
    const agent = call.agent?.trim();
    return agent ? agent : "unknown agent";
  }
  if (call.actor.kind === "human") return "operator";
  const resolvedCaller = callerName?.trim();
  if (resolvedCaller) return resolvedCaller;
  return "unknown caller";
}
