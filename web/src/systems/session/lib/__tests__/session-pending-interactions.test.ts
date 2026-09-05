// Suite: session-pending-interactions
// Invariant: the needs-you reason quotes the newest pending interaction, prefers
// a permission over a later question, and rewrites only terminal actions with
// dedicated approval copy.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import { sessionRuntime } from "../../mocks/fixtures";
import type { SessionPayload, SessionPendingInteraction } from "../../types";
import { pendingInteractionReason } from "../session-pending-interactions";

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
