/**
 * One-line decision copy for a permission receipt.
 *
 * Terminal asks speak the board's register (command, project + agent, did not
 * run). Generic tools keep the tool name and never claim a command they do not
 * have. Allow-always is a remembered project+agent grant, not a session grant.
 */

import { terminalPermissionDetail } from "@/systems/terminal/parts";

import type { PermissionDecision } from "../adapters/session-api";
import type { PermissionRequest } from "../types";
import { primaryPermissionSubject } from "./permission-subject";

export type PermissionReceiptTone = "allowed" | "rejected";

export interface PermissionReceiptCopy {
  tone: PermissionReceiptTone;
  prefix: string;
  join: string;
  subject: string | null;
  suffix: string;
}

function terminalSubject(permission: PermissionRequest): {
  kind: "exec" | "typing" | "open";
  subject: string | null;
} | null {
  const detail = terminalPermissionDetail(
    permission.toolId ?? permission.toolName,
    permission.toolInput
  );
  if (!detail) return null;
  if (detail.kind === "exec") return { kind: "exec", subject: detail.command };
  if (detail.kind === "typing") return { kind: "typing", subject: null };
  return { kind: "open", subject: detail.title };
}

function unsupportedDecision(decision: PermissionDecision): never {
  throw new Error(`unsupported permission decision: ${decision}`);
}

export function permissionReceiptCopy(
  permission: PermissionRequest,
  decision: PermissionDecision
): PermissionReceiptCopy {
  const terminal = terminalSubject(permission);
  if (terminal?.kind === "exec" && terminal.subject) {
    return execReceipt(terminal.subject, decision);
  }
  if (terminal?.kind === "typing") {
    return typingReceipt(decision);
  }
  if (terminal?.kind === "open") {
    return openReceipt(terminal.subject, decision);
  }
  return genericReceipt(permission, decision);
}

function execReceipt(command: string, decision: PermissionDecision): PermissionReceiptCopy {
  switch (decision) {
    case "allow-once":
      return { tone: "allowed", prefix: "Allowed ", join: "", subject: command, suffix: " once" };
    case "allow-always":
      return {
        tone: "allowed",
        prefix: "Allowed ",
        join: "",
        subject: command,
        suffix: " for this project and this agent",
      };
    case "reject-once":
      return {
        tone: "rejected",
        prefix: "Not allowed by you · ",
        join: "",
        subject: command,
        suffix: " did not run",
      };
    case "reject-always":
      return {
        tone: "rejected",
        prefix: "Not allowed by you · ",
        join: "",
        subject: command,
        suffix: " for this project and this agent",
      };
    default:
      return unsupportedDecision(decision);
  }
}

function typingReceipt(decision: PermissionDecision): PermissionReceiptCopy {
  switch (decision) {
    case "allow-once":
      return {
        tone: "allowed",
        prefix: "Allowed typing once",
        join: "",
        subject: null,
        suffix: "",
      };
    case "allow-always":
      return {
        tone: "allowed",
        prefix: "Allowed typing for this terminal",
        join: "",
        subject: null,
        suffix: "",
      };
    case "reject-once":
      return {
        tone: "rejected",
        prefix: "Not allowed by you · the agent did not type",
        join: "",
        subject: null,
        suffix: "",
      };
    case "reject-always":
      return {
        tone: "rejected",
        prefix: "Not allowed by you for this project and this agent",
        join: "",
        subject: null,
        suffix: "",
      };
    default:
      return unsupportedDecision(decision);
  }
}

function openReceipt(title: string | null, decision: PermissionDecision): PermissionReceiptCopy {
  switch (decision) {
    case "allow-once":
      return {
        tone: "allowed",
        prefix: "Allowed opening ",
        join: "",
        subject: title,
        suffix: " once",
      };
    case "allow-always":
      return {
        tone: "allowed",
        prefix: "Allowed opening ",
        join: "",
        subject: title,
        suffix: " for this project and this agent",
      };
    case "reject-once":
      return {
        tone: "rejected",
        prefix: "Not allowed by you · ",
        join: "",
        subject: title,
        suffix: title ? " did not open" : "the terminal did not open",
      };
    case "reject-always":
      return {
        tone: "rejected",
        prefix: "Not allowed by you · ",
        join: "",
        subject: title,
        suffix: " for this project and this agent",
      };
    default:
      return unsupportedDecision(decision);
  }
}

function genericReceipt(
  permission: PermissionRequest,
  decision: PermissionDecision
): PermissionReceiptCopy {
  const subject = primaryPermissionSubject(permission);
  const join = subject ? " — " : "";
  switch (decision) {
    case "allow-once":
      return {
        tone: "allowed",
        prefix: `Allowed ${permission.toolName} once`,
        join,
        subject,
        suffix: "",
      };
    case "allow-always":
      return {
        tone: "allowed",
        prefix: `Allowed ${permission.toolName} for this project and this agent`,
        join,
        subject,
        suffix: "",
      };
    case "reject-once":
      return {
        tone: "rejected",
        prefix: `Not allowed by you · ${permission.toolName}`,
        join,
        subject,
        suffix: "",
      };
    case "reject-always":
      return {
        tone: "rejected",
        prefix: `Not allowed by you · ${permission.toolName} for this project and this agent`,
        join,
        subject,
        suffix: "",
      };
    default:
      return unsupportedDecision(decision);
  }
}

export function terminalWaitingLead(
  permission: PermissionRequest
): { verb: "run" | "type" | "open"; command: string | null } | null {
  const detail = terminalPermissionDetail(
    permission.toolId ?? permission.toolName,
    permission.toolInput
  );
  if (!detail) return null;
  if (detail.kind === "exec") return { verb: "run", command: detail.command };
  if (detail.kind === "typing") return { verb: "type", command: null };
  return { verb: "open", command: detail.title };
}
