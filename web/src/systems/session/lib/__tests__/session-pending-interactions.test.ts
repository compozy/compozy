// Suite: session-pending-interactions
// Invariant: the needs-you reason quotes the newest pending interaction, prefers
// a permission over a later question, and rewrites only terminal actions with
// dedicated approval copy.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import { sessionRuntime } from "../../mocks/fixtures";
import type {
  SessionInteractionRecord,
  SessionPayload,
  SessionPendingInteraction,
} from "../../types";
import {
  expiredInteractionsByRequest,
  interactionExpiredByRestart,
  pendingInteractionReason,
  permissionDecisionActor,
  resolvedInteractionsByRequest,
} from "../session-pending-interactions";

function interaction(
  overrides: Partial<SessionPendingInteraction> & Pick<SessionPendingInteraction, "kind">
): SessionPendingInteraction {
  return {
    interaction_id: "int-1",
    provider_request_id: "req-1",
    status: "pending",
    created_at: "2026-07-20T12:00:00Z",
    ...overrides,
  };
}

function sessionWith(rows: SessionPendingInteraction[]): SessionPayload {
  return {
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    id: "sess-1",
    name: "Review",
    agent_name: "coder",
    runtime: sessionRuntime("claude"),
    workspace_id: "ws-1",
    state: "active",
    type: "user",
    badge: "waiting-for-auth",
    attachable: true,
    archived_at: null,
    available_commands: [],
    pending_interactions: rows,
    created_at: "2026-07-20T12:00:00Z",
    updated_at: "2026-07-20T12:01:00Z",
  };
}

describe("pendingInteractionReason", () => {
  it("Should rewrite a terminal tool-id title as a plain ask", () => {
    expect(
      pendingInteractionReason(
        sessionWith([
          interaction({
            kind: "permission",
            title: "Terminal Exec",
            tool_id: "compozy__terminal_exec",
          }),
        ])
      )
    ).toBe("wants to run");
  });

  it("Should leave a human pending title untouched", () => {
    expect(
      pendingInteractionReason(
        sessionWith([
          interaction({
            kind: "clarify",
            title: "Should I drop the legacy column?",
          }),
        ])
      )
    ).toBe("Should I drop the legacy column?");
  });

  it("Should prefer a pending permission over a later question", () => {
    expect(
      pendingInteractionReason(
        sessionWith([
          interaction({
            interaction_id: "int-q",
            kind: "clarify",
            title: "Should I drop the legacy column?",
          }),
          interaction({
            interaction_id: "int-p",
            kind: "permission",
            title: "Terminal Write",
            tool_id: "compozy__terminal_write",
          }),
        ])
      )
    ).toBe("Terminal Write");
  });

  it("Should return null when nothing is pending", () => {
    expect(pendingInteractionReason(sessionWith([]))).toBeNull();
  });
});

describe("expiredInteractionsByRequest", () => {
  function record(overrides: Partial<SessionInteractionRecord>): SessionInteractionRecord {
    return {
      interaction_id: "int-1",
      kind: "permission",
      provider_request_id: "req-1",
      status: "canceled",
      created_at: "2026-09-05T10:00:00Z",
      resolution: "failed-by-restart",
      resolved_by: "system",
      ...overrides,
    };
  }

  it("Should key only canceled rows by the provider request id the transcript carries", () => {
    const restartExpired = record({});
    const rows = [
      restartExpired,
      record({ interaction_id: "int-2", provider_request_id: "req-2", status: "resolved" }),
      record({ interaction_id: "int-3", provider_request_id: "req-3", status: "timed_out" }),
      record({ interaction_id: "int-4", provider_request_id: "req-4", status: "pending" }),
      record({ interaction_id: "int-5", provider_request_id: "  " }),
    ];

    const expired = expiredInteractionsByRequest(rows);

    expect([...expired.keys()]).toEqual(["req-1"]);
    expect(expired.get("req-1")).toBe(restartExpired);
  });

  it("Should key only resolved rows by the provider request id for attribution", () => {
    const decided = record({
      status: "resolved",
      resolution: "reject-once",
      resolved_by: "timeout",
    });
    const rows = [
      decided,
      record({ interaction_id: "int-2", provider_request_id: "req-2" }),
      record({ interaction_id: "int-3", provider_request_id: "req-3", status: "timed_out" }),
      record({ interaction_id: "int-4", provider_request_id: " ", status: "resolved" }),
    ];

    const resolved = resolvedInteractionsByRequest(rows);

    expect([...resolved.keys()]).toEqual(["req-1"]);
    expect(resolved.get("req-1")).toBe(decided);
  });

  it("Should never let a resolved clarification sharing the request id overwrite the permission", () => {
    const permission = record({
      interaction_id: "int-perm",
      status: "resolved",
      resolution: "reject-once",
      resolved_by: "timeout",
    });
    const clarification = record({
      interaction_id: "int-clar",
      kind: "clarify",
      status: "resolved",
      resolution: "Fast",
      resolved_by: "operator",
    });

    expect(resolvedInteractionsByRequest([permission, clarification]).get("req-1")).toBe(
      permission
    );
    expect(resolvedInteractionsByRequest([clarification, permission]).get("req-1")).toBe(
      permission
    );
    expect(resolvedInteractionsByRequest([clarification]).has("req-1")).toBe(false);
  });

  it("Should read the daemon's resolved_by actor without inventing one", () => {
    const resolved = (resolvedBy: string, status = "resolved") =>
      permissionDecisionActor(
        record({ status, resolution: "reject-once", resolved_by: resolvedBy })
      );

    expect(resolved("operator")).toBe("you");
    expect(resolved("operator:control")).toBe("you");
    expect(resolved("agent_session:sess-reviewer")).toBe("agent");
    expect(resolved("timeout")).toBe("timeout");
    expect(resolved("provider")).toBe("runtime");
    expect(resolved("system")).toBe("runtime");
    expect(resolved("")).toBe("unknown");
    expect(resolved("bridge:acme")).toBe("unknown");
    expect(resolved("operator", "canceled")).toBe("unknown");
    expect(permissionDecisionActor(undefined)).toBe("unknown");
  });

  it("Should attribute a canceled row to the restart only when the daemon says so", () => {
    expect(interactionExpiredByRestart(record({}))).toBe(true);
    expect(
      interactionExpiredByRestart(record({ kind: "clarify", resolution: "", resolved_by: "" }))
    ).toBe(false);
    expect(interactionExpiredByRestart(record({ status: "resolved" }))).toBe(false);
  });
});
