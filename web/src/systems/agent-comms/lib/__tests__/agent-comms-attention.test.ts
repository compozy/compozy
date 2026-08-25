import { describe, expect, it } from "vitest";

import { deriveCallAttention } from "../agent-comms-attention";
import type { CallPayload } from "../../types";

function call(overrides: Partial<CallPayload> & Pick<CallPayload, "call_id">): CallPayload {
  return {
    actor: { id: "operator:http", kind: "operator" },
    agent: "reviewer",
    caller: { id: "ses_root", kind: "session" },
    created_at: "2026-08-20T18:00:00Z",
    depth: 1,
    idle_expires_at: null,
    idle_ttl_seconds: 3600,
    profile_id: "pro_default",
    profile_name: "default",
    prompt_bytes: 0,
    repair_attempts: 0,
    result_budget_bytes: 262_144,
    result_overflow: "store",
    root_session_id: "ses_root",
    scope: "workspace",
    state: "invalid-result",
    strict: false,
    superseded_bytes: 0,
    updated_at: "2026-08-20T18:10:00Z",
    ...overrides,
  };
}

const NONE = new Set<string>();

describe("deriveCallAttention — causes", () => {
  it("Should raise exactly the two needs-you states", () => {
    const model = deriveCallAttention({
      needsYouCalls: [
        call({ call_id: "call_bad", state: "invalid-result" }),
        call({
          call_id: "call_silent",
          state: "completed-without-result",
          root_session_id: "ses_b",
        }),
      ],
      needsYouTotal: 2,
      openCalls: [],
      blockedSessionIds: NONE,
      stale: false,
    });

    expect(model.rows.map(row => row.cause)).toEqual([
      "invalid-result",
      "completed-without-result",
    ]);
    expect(model.count).toBe(2);
  });

  it("Should not raise a failed, canceled, timeout, or expired call", () => {
    // They share the danger tone because expectations went unmet, but the spec
    // names three causes and these are not among them.
    const model = deriveCallAttention({
      needsYouCalls: [
        call({ call_id: "call_failed", state: "failed" }),
        call({ call_id: "call_expired", state: "expired" }),
        call({ call_id: "call_canceled", state: "canceled" }),
        call({ call_id: "call_timeout", state: "timeout" }),
      ],
      needsYouTotal: 0,
      openCalls: [],
      blockedSessionIds: NONE,
      stale: false,
    });
    expect(model.rows).toEqual([]);
  });

  it("Should join a blocked child from the session source rather than a call state", () => {
    const model = deriveCallAttention({
      needsYouCalls: [],
      needsYouTotal: 0,
      openCalls: [
        call({ call_id: "call_open", state: "running", child_session_id: "ses_child" }),
        call({ call_id: "call_other", state: "running", child_session_id: "ses_calm" }),
      ],
      blockedSessionIds: new Set(["ses_child"]),
      stale: false,
    });

    expect(model.rows).toHaveLength(1);
    expect(model.rows[0]).toMatchObject({
      cause: "blocked-on-decision",
      callId: "call_open",
      childSessionId: "ses_child",
    });
    // The OS layer suppresses the duplicate bare session row for this child.
    expect([...model.blockedChildSessionIds]).toEqual(["ses_child"]);
    expect(model.count).toBe(1);
  });
});

describe("deriveCallAttention — counts", () => {
  it("Should take the state count from the daemon, not from the loaded rows", () => {
    const model = deriveCallAttention({
      needsYouCalls: [call({ call_id: "call_one" })],
      needsYouTotal: 47,
      openCalls: [],
      blockedSessionIds: NONE,
      stale: false,
    });
    expect(model.count).toBe(47);
  });

  it("Should contribute zero while the source is stale, but keep rows clickable", () => {
    const model = deriveCallAttention({
      needsYouCalls: [call({ call_id: "call_one" })],
      needsYouTotal: 12,
      openCalls: [],
      blockedSessionIds: NONE,
      stale: true,
    });
    expect(model.count).toBe(0);
    expect(model.rows).toHaveLength(1);
  });

  it("Should label a needs-you call without re-deciding whether it belongs", () => {
    // Membership is the daemon's `attention` filter, and resolution happens
    // there — a later call or message to the child drops the row. This layer
    // must not second-guess that, so a row it was handed is a row it renders.
    // The clearing round trip itself belongs to `use-attention-calls`.
    const model = deriveCallAttention({
      needsYouCalls: [
        call({ call_id: "call_bad", state: "invalid-result", root_session_id: "ses_one" }),
        call({
          call_id: "call_quiet",
          state: "completed-without-result",
          root_session_id: "ses_two",
        }),
      ],
      needsYouTotal: 2,
      openCalls: [],
      blockedSessionIds: NONE,
      stale: false,
    });

    expect(model.count).toBe(2);
    expect(model.rows.map(row => row.cause).sort()).toEqual([
      "completed-without-result",
      "invalid-result",
    ]);
  });
});

describe("deriveCallAttention — coalescing", () => {
  it("Should show one row per tree when a fan-out fails, carrying the real count", () => {
    const failures = Array.from({ length: 5 }, (_, index) =>
      call({
        call_id: `call_${index}`,
        root_session_id: "ses_sweep",
        updated_at: `2026-08-20T18:1${index}:00Z`,
      })
    );

    const model = deriveCallAttention({
      needsYouCalls: failures,
      needsYouTotal: 5,
      openCalls: [],
      blockedSessionIds: NONE,
      stale: false,
    });

    expect(model.rows).toHaveLength(1);
    expect(model.rows[0]).toMatchObject({
      id: "tree:ses_sweep",
      rootSessionId: "ses_sweep",
      count: 5,
    });
    // The dock still says five things need a look — one row, five causes.
    expect(model.count).toBe(5);
  });

  it("Should keep separate trees separate", () => {
    const model = deriveCallAttention({
      needsYouCalls: [
        call({ call_id: "a1", root_session_id: "ses_a" }),
        call({ call_id: "a2", root_session_id: "ses_a" }),
        call({ call_id: "b1", root_session_id: "ses_b" }),
      ],
      needsYouTotal: 3,
      openCalls: [],
      blockedSessionIds: NONE,
      stale: false,
    });

    expect(model.rows).toHaveLength(2);
    expect(model.rows.map(row => row.count).sort()).toEqual([1, 2]);
  });

  it("Should order rows newest first", () => {
    const model = deriveCallAttention({
      needsYouCalls: [
        call({ call_id: "old", root_session_id: "ses_a", updated_at: "2026-08-20T18:00:00Z" }),
        call({ call_id: "new", root_session_id: "ses_b", updated_at: "2026-08-20T18:30:00Z" }),
      ],
      needsYouTotal: 2,
      openCalls: [],
      blockedSessionIds: NONE,
      stale: false,
    });
    expect(model.rows.map(row => row.callId)).toEqual(["new", "old"]);
  });
});
