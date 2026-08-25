/**
 * Reading a permission request as a terminal one.
 *
 * The daemon asks for terminal permission through the same tool-approval path as
 * every other native tool, so the session's decision surface stays one surface.
 * What this module adds is the detail that surface cannot infer on its own: the
 * exact command, where it would run, and whether the runtime could classify it.
 */

import { formatTerminalArgv } from "./terminal-argv";

/** The `compozy__terminal_*` tools that ask before acting. */
const EXEC_TOOL = "compozy__terminal_exec";
const WRITE_TOOL = "compozy__terminal_write";
const OPEN_TOOL = "compozy__terminal_open";

/**
 * Why the runtime is asking.
 *
 * `irreversible` is the fixed safety net — it always prompts, at every autonomy
 * level, and no remembered shape covers it. `unclassifiable` asks for the
 * opposite reason: what cannot be taken apart cannot be remembered either.
 */
export type TerminalPermissionRisk = "ordinary" | "irreversible" | "unclassifiable";

export interface TerminalExecPermission {
  kind: "exec";
  /** The exact command, unmodified. */
  command: string;
  cwd: string;
  terminalId: string | null;
  risk: TerminalPermissionRisk;
}

export interface TerminalTypingPermission {
  kind: "typing";
  terminalId: string | null;
}

export type TerminalPermissionDetail = TerminalExecPermission | TerminalTypingPermission;

export function isTerminalPermission(toolName: string): boolean {
  return toolName === EXEC_TOOL || toolName === WRITE_TOOL || toolName === OPEN_TOOL;
}

/**
 * Reads the terminal detail out of a permission request.
 *
 * Returns null for anything that is not a terminal ask, so the decision surface
 * keeps its generic rendering for every other tool. Nothing is invented: a field
 * the daemon did not send is simply absent.
 */
export function terminalPermissionDetail(
  toolName: string,
  toolInput: Record<string, unknown>
): TerminalPermissionDetail | null {
  if (toolName === WRITE_TOOL) {
    return { kind: "typing", terminalId: readString(toolInput.terminal_id) };
  }
  if (toolName !== EXEC_TOOL && toolName !== OPEN_TOOL) return null;
  const command = readCommand(toolInput);
  if (command === null) return null;
  return {
    kind: "exec",
    command,
    cwd: readString(toolInput.cwd) ?? ".",
    terminalId: readString(toolInput.terminal_id),
    risk: readRisk(toolInput),
  };
}

/**
 * `{command, args}` is the tool's own shape — a vector, not a line.
 *
 * The line shown for approval has to mean exactly the vector that would run, so
 * arguments are quoted rather than blindly joined, and a vector this client
 * cannot represent exactly yields no detail at all. Silently dropping a
 * non-string argument, as the first shape did, would put a shorter command on
 * screen than the one being approved.
 */
function readCommand(toolInput: Record<string, unknown>): string | null {
  const command = readString(toolInput.command);
  if (command === null) return null;
  return formatTerminalArgv(command, toolInput.args);
}

/**
 * The runtime's own classification, never the client's guess.
 *
 * Pattern-matching a command here would be a second, weaker classifier sitting
 * in front of the one that actually gates execution — and the two would
 * disagree exactly where it matters.
 */
function readRisk(toolInput: Record<string, unknown>): TerminalPermissionRisk {
  const risk = readString(toolInput.risk);
  if (risk === "irreversible" || risk === "unclassifiable") return risk;
  return "ordinary";
}

function readString(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}
