/**
 * One-line decision copy for a permission receipt.
 *
 * Terminal asks speak the board's register (command, project + agent, did not
 * run). Generic tools keep the tool name and never claim a command they do not
 * have. Allow-always is a remembered project+agent grant, not a session grant.
 *
 * Who decided comes from the daemon's resolved interaction row, never from the
 * transcript part: "by you" only when an operator surface answered, "another
 * agent" for a native approval from another session, "timed out" when nobody
 * answered, "by the runtime" when the row names the runtime (`provider`, `system`),
 * and a neutral lead when no evidence names an actor. The runtime lead never says
 * whether a question was shown: `provider` is also the daemon's fallback actor.
 */

import { terminalPermissionDetail } from "@/systems/terminal/parts";

import type { PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";
import type { PermissionDecisionActor } from "./session-pending-interactions";
import { primaryPermissionSubject } from "./permission-subject";

export type PermissionReceiptTone = "allowed" | "rejected";

export interface PermissionReceiptCopy {
  tone: PermissionReceiptTone;
  prefix: string;
  join: string;
  subject: string | null;
  suffix: string;
}

interface TerminalPermissionDescriptor {
  kind: "exec" | "open";
  subject: string | null;
  verb: "run" | "open";
}

function terminalPermissionDescriptor(
  permission: PermissionRequest
): TerminalPermissionDescriptor | null {
  const detail = terminalPermissionDetail(
    permission.toolId ?? permission.toolName,
    permission.toolInput
  );
  if (!detail) return null;
  if (detail.kind === "exec") {
    return { kind: "exec", subject: detail.command, verb: "run" };
  }
  return { kind: "open", subject: detail.title, verb: "open" };
}

function unsupportedDecision(decision: PermissionDecision): never {
  throw new Error(`unsupported permission decision: ${decision}`);
}

function isAllowDecision(decision: PermissionDecision): boolean {
  return decision === "allow-once" || decision === "allow-always";
}

function isAlwaysDecision(decision: PermissionDecision): boolean {
  return decision === "allow-always" || decision === "reject-always";
}

/**
 * The sentence lead that carries the outcome and its provenance. An allow by you or
 * by nobody in particular stays the bare "Allowed" the board authored; a refusal
 * names you only when the daemon says an operator answered.
 */
function decisionLead(decision: PermissionDecision, actor: PermissionDecisionActor): string {
  if (isAllowDecision(decision)) {
    switch (actor) {
      case "agent":
        return "Allowed by another agent";
      case "runtime":
        return "Allowed by the runtime";
      default:
        return "Allowed";
    }
  }
  switch (actor) {
    case "you":
      return "Not allowed by you";
    case "agent":
      return "Not allowed by another agent";
    case "runtime":
      return "Not allowed by the runtime";
    case "timeout":
      return "Timed out before anyone answered";
    default:
      return "Not allowed";
  }
}

const ALWAYS_SCOPE = " for this project and this agent";

export function permissionReceiptCopy(
  permission: PermissionRequest,
  decision: PermissionDecision,
  actor: PermissionDecisionActor = "unknown"
): PermissionReceiptCopy {
  switch (decision) {
    case "allow-once":
    case "allow-always":
    case "reject-once":
    case "reject-always":
      break;
    default:
      return unsupportedDecision(decision);
  }
  const tone: PermissionReceiptTone = isAllowDecision(decision) ? "allowed" : "rejected";
  const lead = decisionLead(decision, actor);
  const terminal = terminalPermissionDescriptor(permission);
  if (terminal?.kind === "exec" && terminal.subject) {
    return { tone, ...execReceipt(terminal.subject, decision, lead) };
  }
  if (terminal?.kind === "open") {
    return { tone, ...openReceipt(terminal.subject, decision, lead) };
  }
  return { tone, ...genericReceipt(permission, decision, lead) };
}

type ReceiptSentence = Omit<PermissionReceiptCopy, "tone">;

/** Allowed: `<lead> <cmd> once` — a bare "Allowed" keeps the board's shape without a separator. */
function allowedPrefix(lead: string): string {
  return lead === "Allowed" ? "Allowed " : `${lead} · `;
}

function execReceipt(command: string, decision: PermissionDecision, lead: string): ReceiptSentence {
  if (isAllowDecision(decision)) {
    return {
      prefix: allowedPrefix(lead),
      join: "",
      subject: command,
      suffix: isAlwaysDecision(decision) ? ALWAYS_SCOPE : " once",
    };
  }
  return {
    prefix: `${lead} · `,
    join: "",
    subject: command,
    suffix: isAlwaysDecision(decision) ? ALWAYS_SCOPE : " did not run",
  };
}

function openReceipt(
  title: string | null,
  decision: PermissionDecision,
  lead: string
): ReceiptSentence {
  if (isAllowDecision(decision)) {
    return {
      prefix: `${allowedPrefix(lead)}opening `,
      join: "",
      subject: title,
      suffix: isAlwaysDecision(decision) ? ALWAYS_SCOPE : " once",
    };
  }
  return {
    prefix: `${lead} · `,
    join: "",
    subject: title,
    suffix: isAlwaysDecision(decision)
      ? ALWAYS_SCOPE
      : title
        ? " did not open"
        : "the terminal did not open",
  };
}

function genericReceipt(
  permission: PermissionRequest,
  decision: PermissionDecision,
  lead: string
): ReceiptSentence {
  const subject = primaryPermissionSubject(permission);
  const join = subject ? " — " : "";
  const scope = isAlwaysDecision(decision) ? ALWAYS_SCOPE : "";
  if (isAllowDecision(decision)) {
    return {
      prefix: `${allowedPrefix(lead)}${permission.toolName}${scope || " once"}`,
      join,
      subject,
      suffix: "",
    };
  }
  return {
    prefix: `${lead} · ${permission.toolName}${scope}`,
    join,
    subject,
    suffix: "",
  };
}

export function terminalWaitingLead(
  permission: PermissionRequest
): { verb: "run" | "open"; command: string | null } | null {
  const descriptor = terminalPermissionDescriptor(permission);
  return descriptor ? { verb: descriptor.verb, command: descriptor.subject } : null;
}

export interface PermissionExpiredReceiptCopy {
  /** Sentence before the mono subject; carries the "Not decided" lead. */
  prefix: string;
  subject: string | null;
  suffix: string;
}

/**
 * Why a permission ask settled without a decision: `restart` only when the daemon
 * recorded `failed-by-restart`; any other canceled row is a plain cancellation whose
 * cause the daemon did not name.
 */
export type PermissionExpiredCause = "restart" | "canceled";

const EXPIRED_LEAD: Record<PermissionExpiredCause, string> = {
  restart: "Not decided · CompozyOS restarted before you answered",
  canceled: "Not decided · the request was canceled before you answered",
};

/**
 * Copy for a permission ask nobody answered. Names the restart only when the daemon's
 * resolution is evidence of one; otherwise says "canceled" without inventing an actor.
 * Never blames a person or the provider, and keeps the board's "did not run / did not
 * open" tail so the consequence stays explicit.
 */
export function permissionExpiredReceiptCopy(
  permission: PermissionRequest,
  cause: PermissionExpiredCause
): PermissionExpiredReceiptCopy {
  const lead = EXPIRED_LEAD[cause];
  const terminal = terminalPermissionDescriptor(permission);
  if (terminal?.kind === "exec" && terminal.subject) {
    return { prefix: `${lead} — `, subject: terminal.subject, suffix: " did not run" };
  }
  if (terminal?.kind === "open") {
    return {
      prefix: `${lead} — `,
      subject: terminal.subject,
      suffix: terminal.subject ? " did not open" : "the terminal did not open",
    };
  }
  const subject = primaryPermissionSubject(permission);
  return {
    prefix: `${lead} · ${permission.toolName}${subject ? " — " : ""}`,
    subject,
    suffix: "",
  };
}
