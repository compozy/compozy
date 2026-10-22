import type { ToolCallStatus } from "@compozy/ui";

import type { TerminalExit, TerminalInfo, TerminalSignal } from "@/systems/terminal/parts";
import type { UIMessage } from "../types";

/** Native tools that open or run a supervised terminal — not internal command output. */
const DELIBERATE_TERMINAL_TOOLS = new Set(["compozy__terminal_exec", "compozy__terminal_open"]);

const HOSTED_COMPOZY_TOOL_PATTERN = /^mcp__.+?__(compozy__[a-z0-9_]+)$/;

/** Hosted-MCP projections prefix native ids (`mcp__<server>__compozy__x`); recover the id. */
export function canonicalCompozyToolName(toolName: string): string {
  const trimmed = toolName.trim();
  const match = HOSTED_COMPOZY_TOOL_PATTERN.exec(trimmed);
  return match?.[1] ?? trimmed;
}

export function isDeliberateTerminalTool(toolName: string | undefined): boolean {
  return (
    toolName !== undefined && DELIBERATE_TERMINAL_TOOLS.has(canonicalCompozyToolName(toolName))
  );
}

export function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
}

/** MCP results carry the structured payload as a JSON string; decode before reading. */
function asEnvelopeRecord(value: unknown): Record<string, unknown> | null {
  if (typeof value === "string") {
    try {
      const parsed: unknown = JSON.parse(value);
      return typeof parsed === "object" && parsed !== null
        ? (parsed as Record<string, unknown>)
        : null;
    } catch {
      return null;
    }
  }
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

/** The supervised terminal this tool call named, when the runtime created one. */
export function readSupervisedTerminalId(rawOutput: unknown): string | null {
  return readNonEmptyString(readTerminalEnvelope(rawOutput).terminal_id);
}

export function readNonEmptyString(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

export function readTerminalEnvelope(rawOutput: unknown): Record<string, unknown> {
  const envelope = asRecord(rawOutput);
  for (const candidate of [envelope.structured, envelope.raw_output, envelope.rawOutput]) {
    const record = asEnvelopeRecord(candidate);
    if (record !== null) return record;
  }
  return asRecord(rawOutput);
}

export interface SessionTerminalEnvelopeRun {
  stillRunning: boolean;
  exitCode: number | null;
  signal: TerminalSignal | null;
}

/**
 * Live catalog exit wins a frozen ACP envelope.
 *
 * `still_running` and an open-without-exit record the agent's wait, not the
 * terminal's life. Once the catalog row is `exited` or carries `exit`, the
 * block must drop live attach and show the finished foot.
 */
export function resolveSessionTerminalRun(
  envelope: SessionTerminalEnvelopeRun,
  catalog: Pick<TerminalInfo, "state" | "exit"> | undefined
): { stillRunning: boolean; exit: TerminalExit | null } {
  if (catalog !== undefined && (catalog.state === "exited" || catalog.exit != null)) {
    return {
      stillRunning: false,
      exit: catalog.exit ?? envelopeExit(envelope, ""),
    };
  }
  if (envelope.stillRunning) {
    return { stillRunning: true, exit: null };
  }
  return { stillRunning: false, exit: envelopeExit(envelope, catalog?.exit?.at ?? "") };
}

function envelopeExit(envelope: SessionTerminalEnvelopeRun, at: string): TerminalExit {
  if (envelope.signal) {
    return { cause: "signaled", signal: envelope.signal, at };
  }
  if (envelope.exitCode === null) {
    return { cause: "unknown", at };
  }
  return { cause: "exited", code: envelope.exitCode, at };
}

/**
 * The output a terminal tool call recorded — never its envelope.
 *
 * MCP results also carry the structured payload serialized as `content`, so an
 * empty `output` must stay empty: falling back to `content` would paint the
 * envelope's JSON as if the command had printed it. `stdout`/`content` are
 * read only when the result carries no terminal envelope at all.
 */
export function readTerminalOutput(
  result: Pick<NonNullable<UIMessage["toolResult"]>, "rawOutput" | "stdout" | "content"> | undefined
): string {
  const envelope = readTerminalEnvelope(result?.rawOutput);
  if (typeof envelope.output === "string") return envelope.output;
  if (TERMINAL_ENVELOPE_FIELDS.some(field => field in envelope)) return "";
  return result?.stdout ?? result?.content ?? "";
}

const TERMINAL_ENVELOPE_FIELDS = ["terminal_id", "command_id", "exit_code", "still_running"];

export interface SessionTerminalFacts extends SessionTerminalEnvelopeRun {
  terminalId: string;
  /** The human name the agent gave the terminal; never the argv. */
  title: string | null;
  preview: string;
  durationMs: number | null;
}

/**
 * Reads the tool call's own record.
 *
 * The terminal fields ride the raw tool envelope rather than the normalized
 * `ToolUseResult`, so they are read from there and narrowed one field at a time.
 * Nothing is inferred: a field the runtime did not send stays absent.
 */
export function readSessionTerminalFacts(message: UIMessage): SessionTerminalFacts | null {
  const result = message.toolResult;
  const raw = readTerminalEnvelope(result?.rawOutput);
  const terminalId = readSupervisedTerminalId(result?.rawOutput);
  if (!terminalId) return null;
  const exitCode = typeof raw.exit_code === "number" ? raw.exit_code : null;
  const signal = readTerminalSignal(raw.signal);
  const durationMs =
    typeof raw.duration_ms === "number" && Number.isFinite(raw.duration_ms) && raw.duration_ms > 0
      ? raw.duration_ms
      : null;
  return {
    terminalId,
    title: readNonEmptyString(asRecord(message.toolInput).title),
    preview: readTerminalOutput(result),
    exitCode,
    signal,
    durationMs,
    stillRunning:
      raw.still_running === true ||
      (canonicalCompozyToolName(message.toolName ?? "") === "compozy__terminal_open" &&
        exitCode === null &&
        signal === null),
  };
}

function readTerminalSignal(value: unknown): TerminalSignal | null {
  switch (value) {
    case "INT":
    case "TERM":
    case "KILL":
    case "HUP":
      return value;
    default:
      return null;
  }
}

export interface SessionTerminalOutcome {
  status: ToolCallStatus;
  /** The one outcome word the glyph cannot carry (`exit 1`, `signal TERM`); null when calm. */
  mark: string | null;
}

/**
 * The single status a terminal row states, from the resolved run alone.
 *
 * Success rests without a glyph, a non-zero exit is a plain code beside the
 * settled ×, a signal reads as stopped, and a run still going spins.
 */
export function sessionTerminalOutcome(
  run: { stillRunning: boolean; exit: TerminalExit | null },
  toolFailed = false
): SessionTerminalOutcome {
  const exit = run.exit;
  if (!exit) return { status: "running", mark: null };
  if (exit.cause === "signaled") {
    return { status: "stopped", mark: `signal ${exit.signal ?? "unknown"}` };
  }
  if (exit.cause === "unknown" || exit.code === null || exit.code === undefined) {
    return toolFailed ? { status: "absorbed", mark: "failed" } : { status: "empty", mark: "ended" };
  }
  if (exit.code === 0) return { status: "success", mark: null };
  return { status: "absorbed", mark: `exit ${exit.code}` };
}
