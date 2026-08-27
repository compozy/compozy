import { describe, expect, it } from "vitest";

import type { SessionPayload } from "@/systems/session";

import { buildCallTree, childStatesForRoot, escalateCallStates } from "../agent-comms-tree";
import type { CallPayload } from "../../types";

/**
 * Minimal call record. Every field the projection reads is explicit at the call
 * site of `call()`; the rest exist only to satisfy the wire shape.
 */
function call(overrides: Partial<CallPayload> & Pick<CallPayload, "call_id">): CallPayload {
  return {
    actor: { id: "operator:http", kind: "operator" },
    caller: { id: "ses_root", kind: "session" },
    created_at: "2026-08-20T18:12:04Z",
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
    state: "running",
    strict: false,
    superseded_bytes: 0,
    updated_at: "2026-08-20T18:12:04Z",
    ...overrides,
  };
}

describe("buildCallTree", () => {
  it("Should group by governed root and order calls along the session lineage", () => {
    const tree = buildCallTree([
      call({
        call_id: "call_grandchild",
        parent_session_id: "ses_child",
        child_session_id: "ses_grandchild",
        depth: 2,
        state: "queued",
      }),
      call({
        call_id: "call_child",
        parent_session_id: "ses_root",
        child_session_id: "ses_child",
        depth: 1,
      }),
      call({
        call_id: "call_sibling",
        parent_session_id: "ses_root",
        child_session_id: "ses_sibling",
        depth: 1,
        state: "completed",
      }),
    ]);

    expect(tree.groups).toHaveLength(1);
    const [group] = tree.groups;
    expect(group!.rootSessionId).toBe("ses_root");
    // Lineage order, not the incoming order: the root's own calls first, then
    // the branch that descends from the first of them.
    expect(group!.rows.map(row => row.call.call_id)).toEqual([
      "call_child",
      "call_grandchild",
      "call_sibling",
    ]);
    expect(group!.rows.every(row => row.orphaned === false)).toBe(true);
  });

  it("Should keep one group per root, in the order the daemon returned them", () => {
    const tree = buildCallTree([
      call({ call_id: "call_b", root_session_id: "ses_beta", parent_session_id: "ses_beta" }),
      call({ call_id: "call_a", root_session_id: "ses_alpha", parent_session_id: "ses_alpha" }),
      call({ call_id: "call_b2", root_session_id: "ses_beta", parent_session_id: "ses_beta" }),
    ]);

    expect(tree.groups.map(group => group.rootSessionId)).toEqual(["ses_beta", "ses_alpha"]);
    expect(tree.groups[0]!.rows.map(row => row.call.call_id)).toEqual(["call_b", "call_b2"]);
  });

  it("Should indent from the record's own depth, not from how much of the page loaded", () => {
    // The depth-1 call that created `ses_child` is not in this page at all.
    const tree = buildCallTree([
      call({
        call_id: "call_deep",
        parent_session_id: "ses_child",
        child_session_id: "ses_grandchild",
        depth: 2,
      }),
    ]);

    const [row] = tree.groups[0]!.rows;
    expect(row!.depth).toBe(2);
    expect(row!.orphaned).toBe(true);
  });

  it("Should emit a call whose caller is unreachable rather than dropping it", () => {
    const tree = buildCallTree([
      call({ call_id: "call_reachable", parent_session_id: "ses_root", child_session_id: "ses_a" }),
      call({
        call_id: "call_stranded",
        parent_session_id: "ses_absent",
        child_session_id: "ses_b",
      }),
    ]);

    const rows = tree.groups[0]!.rows;
    expect(rows.map(row => row.call.call_id)).toEqual(["call_reachable", "call_stranded"]);
    expect(rows.find(row => row.call.call_id === "call_stranded")!.orphaned).toBe(true);
  });

  it("Should survive forged lineage that loops, keeping every row exactly once", () => {
    // ses_root -> ses_a -> ses_b -> ses_a: the last edge closes a loop.
    const tree = buildCallTree([
      call({ call_id: "call_1", parent_session_id: "ses_root", child_session_id: "ses_a" }),
      call({ call_id: "call_2", parent_session_id: "ses_a", child_session_id: "ses_b" }),
      call({ call_id: "call_3", parent_session_id: "ses_b", child_session_id: "ses_a" }),
    ]);

    const ids = tree.groups[0]!.rows.map(row => row.call.call_id);
    expect(ids).toHaveLength(3);
    expect(new Set(ids).size).toBe(3);
    expect(tree.cyclicSessionIds.has("ses_a")).toBe(true);
  });

  it("Should partition running, parked-child, and terminal rows without losing any", () => {
    const tree = buildCallTree([
      call({ call_id: "call_run", parent_session_id: "ses_root", state: "running" }),
      call({ call_id: "call_done", parent_session_id: "ses_root", state: "completed" }),
      call({ call_id: "call_bad", parent_session_id: "ses_root", state: "invalid-result" }),
      call({ call_id: "call_gone", parent_session_id: "ses_root", state: "expired" }),
    ]);

    const rows = tree.groups[0]!.rows;
    expect(rows).toHaveLength(4);
    expect(rows.map(row => row.state)).toEqual([
      "running",
      "completed",
      "invalid-result",
      "expired",
    ]);
  });
});

describe("buildCallTree — nesting for the tree primitive", () => {
  it("Should expose the root's own calls as the group's first level", () => {
    const tree = buildCallTree([
      call({ call_id: "call_a", parent_session_id: "ses_root", child_session_id: "ses_a" }),
      call({ call_id: "call_b", parent_session_id: "ses_root", child_session_id: "ses_b" }),
      call({
        call_id: "call_deep",
        parent_session_id: "ses_a",
        child_session_id: "ses_c",
        depth: 2,
      }),
    ]);

    const [group] = tree.groups;
    expect(group!.topLevelCallIds).toEqual(["call_a", "call_b"]);
    expect(tree.rowsByCallId.get("call_a")!.childCallIds).toEqual(["call_deep"]);
    expect(tree.rowsByCallId.get("call_b")!.childCallIds).toEqual([]);
  });

  it("Should give a re-called child's subtree to one parent, not to every caller", () => {
    // Two calls target the same child; the child then delegates once. Attaching
    // that work to both callers would render the subtree twice.
    const tree = buildCallTree([
      call({
        call_id: "call_first",
        parent_session_id: "ses_root",
        child_session_id: "ses_shared",
      }),
      call({
        call_id: "call_again",
        parent_session_id: "ses_root",
        child_session_id: "ses_shared",
      }),
      call({
        call_id: "call_from_shared",
        parent_session_id: "ses_shared",
        child_session_id: "ses_leaf",
        depth: 2,
      }),
    ]);

    expect(tree.rowsByCallId.get("call_first")!.childCallIds).toEqual(["call_from_shared"]);
    expect(tree.rowsByCallId.get("call_again")!.childCallIds).toEqual([]);
    // Still exactly three rows — the shared subtree is not duplicated.
    expect(tree.groups[0]!.rows).toHaveLength(3);
    expect(tree.cyclicSessionIds.size).toBe(0);
  });

  it("Should index every row by call id, orphans included", () => {
    const tree = buildCallTree([
      call({ call_id: "call_ok", parent_session_id: "ses_root" }),
      call({ call_id: "call_stranded", parent_session_id: "ses_absent" }),
    ]);

    expect([...tree.rowsByCallId.keys()].sort()).toEqual(["call_ok", "call_stranded"]);
    expect(tree.groups[0]!.topLevelCallIds).toContain("call_stranded");
  });
});

describe("escalateCallStates", () => {
  it("Should raise the needs-you state above every other outcome in the tree", () => {
    const tree = buildCallTree([
      call({ call_id: "call_run", parent_session_id: "ses_root", state: "running" }),
      call({ call_id: "call_cancel", parent_session_id: "ses_root", state: "canceled" }),
      call({ call_id: "call_bad", parent_session_id: "ses_root", state: "invalid-result" }),
    ]);

    expect(tree.groups[0]!.escalation).toBe("invalid-result");
  });

  it("Should rank a deliberate stop below a fault and above liveness", () => {
    expect(
      escalateCallStates([
        {
          call: call({ call_id: "a" }),
          state: "running",
          depth: 1,
          orphaned: false,
          childCallIds: [],
        },
        {
          call: call({ call_id: "b" }),
          state: "canceled",
          depth: 1,
          orphaned: false,
          childCallIds: [],
        },
      ])
    ).toBe("canceled");

    expect(
      escalateCallStates([
        {
          call: call({ call_id: "a" }),
          state: "canceled",
          depth: 1,
          orphaned: false,
          childCallIds: [],
        },
        {
          call: call({ call_id: "b" }),
          state: "failed",
          depth: 1,
          orphaned: false,
          childCallIds: [],
        },
      ])
    ).toBe("failed");
  });

  it("Should never escalate a good answer", () => {
    expect(
      escalateCallStates([
        {
          call: call({ call_id: "a" }),
          state: "completed",
          depth: 1,
          orphaned: false,
          childCallIds: [],
        },
        {
          call: call({ call_id: "b" }),
          state: "completed",
          depth: 1,
          orphaned: false,
          childCallIds: [],
        },
      ])
    ).toBeNull();
  });
});

/**
 * Invariant: a child is reported gone only when a complete catalog could have
 * listed it and did not. Owning layer: `childStatesForRoot`. Canonical suite:
 * this file — it already owns the tree-row projection.
 */
describe("childStatesForRoot", () => {
  function session(overrides: Partial<SessionPayload> & Pick<SessionPayload, "id">) {
    return { state: "active", ...overrides } as SessionPayload;
  }

  it("Should report an expected child the catalog omits as gone", () => {
    // The whole reason the expected ids are an input: there is no session row
    // to iterate for a child that no longer exists.
    const states = childStatesForRoot(["ses_alive", "ses_reaped"], [session({ id: "ses_alive" })]);

    expect(states.get("ses_reaped")).toBe("gone");
    expect(states.get("ses_alive")).toBe("running");
  });

  it("Should distinguish a settlement-parked child from every other stopped child", () => {
    // A crashed child is also `stopped`, but it is gone rather than resting —
    // so the daemon's own stop detail is what distinguishes the parked child.
    const states = childStatesForRoot(
      ["ses_parked", "ses_crashed"],
      [
        session({ id: "ses_parked", state: "stopped", stop_detail: "call child parked" }),
        session({ id: "ses_crashed", state: "stopped", stop_detail: "provider crashed" }),
      ]
    );

    expect(states.get("ses_parked")).toBe("parked");
    expect(states.get("ses_crashed")).toBe("gone");
  });

  it("Should claim nothing at all while a root's catalog is incomplete", () => {
    // Fail open: a slow or failed read must never render as a dead child.
    expect(childStatesForRoot(["ses_alive", "ses_reaped"], undefined).size).toBe(0);
  });

  it("Should ignore sessions outside the expected set", () => {
    const states = childStatesForRoot(
      ["ses_mine"],
      [session({ id: "ses_mine" }), session({ id: "ses_someone_elses" })]
    );

    expect([...states.keys()]).toEqual(["ses_mine"]);
  });
});
