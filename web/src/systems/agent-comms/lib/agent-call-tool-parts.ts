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

/** The tool that closes a child call. Rendered as the return bookend, not a row. */
export const AGENT_CALL_RETURN_TOOL_NAME = "compozy__call_return";

/** ACP title some streams use instead of the native tool id. */
const AGENT_CALL_TOOL_TITLE = "Agent Call";

export interface AgentCallToolArgs {
  agent: string | null;
  prompt: string | null;
}

/**
 * Whether this part is a delegation, including the ACP title form.
 * Title match is prefix-based so "Agent Call reviewer" still counts.
 */
export function isAgentCallToolName(toolName: string): boolean {
  const trimmed = toolName.trim();
  return (
    trimmed === AGENT_CALL_TOOL_NAME ||
    trimmed === AGENT_CALL_TOOL_TITLE ||
    trimmed.startsWith(`${AGENT_CALL_TOOL_TITLE} `)
  );
}

export function isCallReturnToolName(toolName: string): boolean {
  return toolName.trim() === AGENT_CALL_RETURN_TOOL_NAME;
}

function textArg(source: Record<string, unknown>, key: string): string | null {
  const value = source[key];
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

/** The ask the tool was invoked with — used while the record is still hydrating. */
export function agentCallArgsFromTool(
  args: Record<string, unknown> | undefined
): AgentCallToolArgs {
  if (args === undefined) return { agent: null, prompt: null };
  return {
    agent: textArg(args, "agent") ?? textArg(args, "target"),
    prompt: textArg(args, "prompt"),
  };
}

export function verdictFromReturnResult(result: unknown): string | null {
  const unwrapped = unwrapToolResult(result);
  if (!isRecord(unwrapped)) return null;
  const verdict = unwrapped.verdict;
  return typeof verdict === "string" && verdict.trim() !== "" ? verdict : null;
}

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
 * Reach the provider's structured result through live and replay envelopes.
 * Each layer is named by a shipped transcript contract; arbitrary object walks
 * would risk treating a prompt or diagnostic field as a call record.
 */
function unwrapToolResult(value: unknown): unknown {
  let current = value;
  const seen = new Set<object>();
  for (let depth = 0; depth < 4 && isRecord(current); depth += 1) {
    if (seen.has(current)) break;
    seen.add(current);
    const nested = current.raw ?? current.raw_output ?? current.rawOutput;
    if (nested === undefined) break;
    current = nested;
  }
  return current;
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
  const unwrapped = unwrapToolResult(result);
  const single = callIdOf(unwrapped);
  if (single !== null) return [single];
  if (!isRecord(unwrapped)) return [];

  const batch = Array.isArray(unwrapped.items)
    ? unwrapped.items
    : Array.isArray(unwrapped.tasks)
      ? unwrapped.tasks
      : Array.isArray(unwrapped.results)
        ? unwrapped.results
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
