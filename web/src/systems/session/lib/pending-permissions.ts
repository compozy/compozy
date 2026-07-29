// Pure derivation of the session's undecided permission requests from the
// transcript messages — the composer decision dock's data source. The runtime
// appends permission data parts per request; the latest part per `request_id`
// carries the current state, and a part with no normalized decision is pending.

import type { PermissionDecision } from "../adapters/session-api";
import { normalizePermissionDecision, toPermissionRequest } from "./message-parts";
import { primaryPermissionSubject } from "./permission-subject";
import type { CompozyPermissionData, PermissionRequest } from "../types";

const FALLBACK_PERMISSION_DECISIONS = [
  "allow-once",
  "allow-always",
  "reject-once",
  "reject-always",
] satisfies PermissionDecision[];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isPermissionData(value: unknown): value is CompozyPermissionData {
  return isRecord(value) && typeof value.request_id === "string";
}

interface MessageLike {
  content?: unknown;
}

/**
 * All undecided permission requests, in first-seen order. A later part for the
 * same `request_id` supersedes the earlier one, so a decided request drops out.
 */
export function derivePendingPermissions(messages: readonly MessageLike[]): PermissionRequest[] {
  const byRequest = new Map<string, CompozyPermissionData>();
  for (const message of messages) {
    const content = Array.isArray(message.content) ? message.content : [];
    for (const part of content) {
      if (!isRecord(part)) continue;
      const isPermissionPart =
        part.type === "data-compozy-permission" ||
        (part.type === "data" && part.name === "compozy-permission");
      if (!isPermissionPart) continue;
      if (!isPermissionData(part.data)) continue;
      byRequest.set(part.data.request_id, part.data);
    }
  }
  const pending: PermissionRequest[] = [];
  for (const data of byRequest.values()) {
    if (normalizePermissionDecision(data.decision) !== null) continue;
    pending.push(toPermissionRequest(data));
  }
  return pending;
}

/**
 * The decisions the dock may offer, in canonical order. An absent or empty
 * advertisement falls back to all four ACP decisions; a non-empty advertisement
 * containing no canonical decisions intentionally renders no decision buttons.
 */
export function permissionDecisionOptions(permission: PermissionRequest): PermissionDecision[] {
  if (permission.supportedDecisions == null || permission.supportedDecisions.length === 0) {
    return FALLBACK_PERMISSION_DECISIONS;
  }
  const supported = new Set(permission.supportedDecisions);
  const filtered = FALLBACK_PERMISSION_DECISIONS.filter(decision => supported.has(decision));
  return filtered;
}

/** The mono subject the dock's pre shows — command, resource, or the raw input. */
export function permissionSubject(permission: PermissionRequest): string | null {
  const primary = primaryPermissionSubject(permission);
  if (primary) return primary;
  if (Object.keys(permission.toolInput).length > 0) {
    try {
      return JSON.stringify(permission.toolInput, null, 2);
    } catch {
      return null;
    }
  }
  return null;
}
