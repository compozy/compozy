/**
 * Finding the calls a turn made, on the initiator's side.
 *
 * A caller's delegation is already in its transcript: `compozy__agent_call` is a
 * native tool call, and the daemon has ordered it among the turn's other work.
 * Its result carries the call id (or, for a fan-out, one per batch item), so the
 * inline card is a *rendering* of a part that is already there — not a second
 * data source merged in beside it.
 *
 * This module only extracts identities. The records themselves are read by id
 * through the calls API, so what the card shows is the live call record rather
 * than a snapshot frozen into the transcript when the tool returned.
 */

/** The one tool that starts a delegation. */
export const AGENT_CALL_TOOL_NAME = "compozy__agent_call";

export interface AgentCallToolInvocation {
  /** The tool call's own id — stable across re-renders, so it keys the card. */
  toolCallId: string;
  /** Call ids this invocation produced. More than one means a fan-out. */
  callIds: string[];
  /** True while the tool has not returned yet. */
  pending: boolean;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function callIdOf(value: unknown): string | null {
  if (!isRecord(value)) return null;
  const id = value.call_id;
  return typeof id === "string" && id.trim() !== "" ? id : null;
}

/**
 * Every call id a tool result names.
 *
 * Two shapes, matching the tool's two modes: a single call answers with one
 * `call_id`, and a batch answers with a `tasks` array whose entries each carry
 * their own — including entries that failed, which have an `error` and no id and
 * are skipped here because there is no record to open.
 */
export function callIdsFromToolResult(result: unknown): string[] {
  const single = callIdOf(result);
  if (single !== null) return [single];
  if (!isRecord(result)) return [];

  const batch = Array.isArray(result.tasks)
    ? result.tasks
    : Array.isArray(result.results)
      ? result.results
      : null;
  if (batch === null) return [];

  const ids: string[] = [];
  for (const entry of batch) {
    // A batch item is either `{call: {...}}` or the call shape inline.
    const nested = isRecord(entry) && isRecord(entry.call) ? entry.call : entry;
    const id = callIdOf(nested);
    if (id !== null) ids.push(id);
  }
  return ids;
}
