/**
 * Plain-language words for a terminal permission.
 *
 * The session dock and the grants list share this register so a prompt, a
 * receipt, and a remembered row say the same promise. Nothing here invents a
 * command shape, a terminal name, or a digest reading.
 */

import type { TerminalGrant } from "./terminal-grant";
import type { TerminalPermissionDetail } from "./terminal-permission";

const EXEC_TOOL = "compozy__terminal_exec";
const WRITE_TOOL = "compozy__terminal_write";
const OPEN_TOOL = "compozy__terminal_open";

const ATTENTION_BY_ID: Record<string, string> = {
  [EXEC_TOOL]: "wants to run",
  [WRITE_TOOL]: "wants to type",
  [OPEN_TOOL]: "wants to open a terminal",
};

const ATTENTION_BY_TITLE: Record<string, string> = {
  "Terminal Exec": "wants to run",
  "Terminal Write": "wants to type",
  "Terminal Open": "wants to open a terminal",
};

function askVerb(detail: TerminalPermissionDetail): string {
  if (detail.kind === "typing") return "wants to type";
  if (detail.kind === "open") return "wants to open a terminal";
  return "wants to run";
}

/** Title on the decision surface: agent + verb, and a live catalog title when known. */
export function terminalAskTitle(
  detail: TerminalPermissionDetail,
  agentName: string,
  terminalTitle?: string
): string {
  const known = terminalTitle?.trim();
  const head = `${agentName} ${askVerb(detail)}`;
  return known ? `${head} · ${known}` : head;
}

/** Durable-allow label. Exec remembers one hashed input, never a class of commands. */
export function terminalAlwaysAllowLabel(detail: TerminalPermissionDetail): string {
  if (detail.kind === "typing") return "Allow for this terminal";
  if (detail.kind === "exec") return "Always allow this exact command";
  return "Always allow";
}

export function terminalRejectOnceLabel(): string {
  return "Don't allow";
}

export function terminalGrantLabel(grant: TerminalGrant): string {
  if (grant.kind === "typing") return "Can type in one terminal";
  return "Always allowed: this exact command";
}

/**
 * Rewrites a tool-id title into the board's verb. Human titles stay untouched.
 * Does not invent the command — a pending interaction does not carry argv.
 */
export function terminalAttentionReason(title?: string, toolId?: string): string | null {
  const id = toolId?.trim();
  if (id && ATTENTION_BY_ID[id]) return ATTENTION_BY_ID[id] ?? null;
  const labeled = title?.trim();
  if (!labeled) return null;
  if (ATTENTION_BY_ID[labeled]) return ATTENTION_BY_ID[labeled] ?? null;
  if (ATTENTION_BY_TITLE[labeled]) return ATTENTION_BY_TITLE[labeled] ?? null;
  return null;
}

export function terminalIdFromDetail(detail: TerminalPermissionDetail): string | null {
  if (detail.kind === "open") return null;
  return detail.terminalId;
}
