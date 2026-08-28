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
  /** Working directory when the daemon sent one. Never invented. */
  cwd: string | null;
  terminalId: string | null;
  risk: TerminalPermissionRisk;
}

export interface TerminalTypingPermission {
  kind: "typing";
  terminalId: string | null;
  /**
   * Optional one-clause activity on the typing tool input.
   *
   * The live authorize-input payload is `terminal_id` + `grant_generation`.
   * This field is shown only when the tool input actually carries `activity`.
   * Keystrokes (`data`) and request_input `reason` are never read as activity.
   */
  activity: string | null;
}

export interface TerminalOpenPermission {
  kind: "open";
  /** Catalog title when the daemon sent one. Never a stand-in name. */
  title: string | null;
  cwd: string | null;
  shell: string | null;
}

export type TerminalPermissionDetail =
  | TerminalExecPermission
  | TerminalTypingPermission
  | TerminalOpenPermission;

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
    return {
      kind: "typing",
      terminalId: readString(toolInput.terminal_id),
      // Only `activity` is a typing-ask field. `reason` belongs to request_input.
      activity: readString(toolInput.activity),
    };
  }
  if (toolName === OPEN_TOOL) {
    return {
      kind: "open",
      title: readString(toolInput.title),
      cwd: readString(toolInput.cwd),
      shell: readString(toolInput.shell),
    };
  }
  if (toolName !== EXEC_TOOL) return null;
  const command = readCommand(toolInput);
  if (command === null) return null;
  return {
    kind: "exec",
    command,
    cwd: readString(toolInput.cwd),
    terminalId: readString(toolInput.terminal_id),
    risk: readRisk(toolInput),
  };
}

/**
 * Remembered decisions are a pattern to keep. Irreversible and unclassifiable
 * always ask, so neither polarity — allow-always nor reject-always — can be
 * stored. Ordinary exec keeps both; typing and open are not this rule.
 */
export function terminalBlockedRememberedDecisions(
  detail: TerminalPermissionDetail | null
): readonly ("allow-always" | "reject-always")[] {
  if (detail?.kind === "exec" && detail.risk !== "ordinary") {
    return ["allow-always", "reject-always"];
  }
  return [];
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
  if (risk === "ordinary" || risk === "irreversible" || risk === "unclassifiable") return risk;
  // Missing or future classifications are not ordinary. Treating them as safe
  // would re-enable remembered approval precisely when this client cannot read
  // the daemon's decision.
  return "unclassifiable";
}

function readString(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}
