// Suite: clarify-event
// Invariant: the durable clarify payload parses unbounded waits (deadline
// null) into a deadline-free view, and the deadline hint stays empty.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import type { AgentEventPayload } from "../../types";
import { formatClarifyDeadline, parseClarifyEvent } from "../clarify-event";

function pendingPayload(deadline: string | null): AgentEventPayload {
  return {
    type: "clarify",
    raw: {
      status: "pending",
      request: {
        request_id: "req-clarify-01",
        session_id: "sess-9f2c",
        question: "Which environment should deploy first?",
        choices: ["staging", "production"],
        asked_at: "2026-09-17T10:00:00Z",
        deadline,
      },
      at: "2026-09-17T10:00:00Z",
    },
  };
}

describe("parseClarifyEvent", () => {
  it("parses an unbounded pending event with a null deadline", () => {
    const view = parseClarifyEvent(pendingPayload(null));

    expect(view?.status).toBe("pending");
    expect(view?.requestId).toBe("req-clarify-01");
    expect(view?.request.deadline).toBeNull();
  });

  it("keeps a finite deadline on the parsed view", () => {
    const view = parseClarifyEvent(pendingPayload("2026-09-17T10:05:00Z"));

    expect(view?.request.deadline).toBe("2026-09-17T10:05:00Z");
  });

  it("rejects a payload whose deadline is neither a string nor null", () => {
    const view = parseClarifyEvent(pendingPayload(7 as unknown as string));

    expect(view).toBeNull();
  });
});

describe("formatClarifyDeadline", () => {
  it("returns no hint for a null or missing deadline", () => {
    expect(formatClarifyDeadline(null)).toBeNull();
    expect(formatClarifyDeadline(undefined)).toBeNull();
  });

  it("renders a finite deadline as a local time hint", () => {
    expect(formatClarifyDeadline("2026-09-17T10:05:00Z")).not.toBeNull();
  });
});
