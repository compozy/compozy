import { describe, expect, it } from "vitest";

import { buildCallDetailView } from "../call-detail-view-model";
import { buildCallResultShape } from "../call-result-rows";
import type { CallPayload } from "../../types";

function call(overrides: Partial<CallPayload> = {}): CallPayload {
  return {
    actor: { id: "operator:http", kind: "human" },
    agent: "reviewer",
    call_id: "call_01JBD8G2K7Q9",
    caller: { id: "ses_01JBD7ZZAAAA", kind: "session" },
    child_session_id: "ses_01JBD8G2MZTX",
    created_at: "2026-08-20T18:12:04Z",
    depth: 1,
    idle_ttl_seconds: 3600,
    parent_session_id: "ses_01JBD7ZZAAAA",
    profile_id: "pro_default",
    profile_name: "default",
    prompt_bytes: 0,
    repair_attempts: 0,
    result_budget_bytes: 262_144,
    result_overflow: "store",
    root_session_id: "ses_01JBD7ZZAAAA",
    scope: "workspace",
    state: "completed",
    strict: false,
    superseded_bytes: 0,
    updated_at: "2026-08-20T18:14:11Z",
    verdict: "returned",
    workspace_id: "ws_main",
    ...overrides,
  };
}

describe("buildCallDetailView — controls", () => {
  it("Should offer cancel alone while the call is in flight", () => {
    const view = buildCallDetailView({
      call: call({ state: "running", verdict: undefined, settled_at: undefined }),
    });
    expect(view.controls).toMatchObject({ cancel: true, callAgain: false });
  });

  it("Should replace cancel with call-again once the call settles", () => {
    const view = buildCallDetailView({ call: call({ settled_at: "2026-08-20T18:14:11Z" }) });
    expect(view.controls.cancel).toBe(false);
    expect(view.controls.callAgain).toBe(true);
  });

  it("Should keep messaging available on a parked child — that IS the revival", () => {
    // No Revive control exists anywhere; contacting the child revives it.
    const view = buildCallDetailView({ call: call({ state: "completed" }) });
    expect(view.controls.messageChild).toBe(true);
  });

  it("Should withdraw messaging when the target itself expired", () => {
    const view = buildCallDetailView({ call: call({ state: "expired", verdict: undefined }) });
    expect(view.controls.messageChild).toBe(false);
    // Identity is kept; only the affordance goes.
    expect(view.childSessionId).toBe("ses_01JBD8G2MZTX");
  });

  it("Should drop the jump link when the counterpart session was pruned", () => {
    const view = buildCallDetailView({ call: call(), counterpartExists: false });
    expect(view.controls.openChildSession).toBe(false);
    expect(view.childSessionId).toBe("ses_01JBD8G2MZTX");
    expect(view.callerKind).toBe("session");
    expect(view.resultBudgetBytes).toBe(262_144);
    expect(view.resultOverflow).toBe("store");
  });

  it("Should expose exactly the four supported controls", () => {
    // Cancel, call again, message child — plus the jump link. Anything else
    // would be a button for an operation with no operator surface behind it.
    const view = buildCallDetailView({ call: call({ result_preview: { verdict: "ok" } }) });
    expect(Object.keys(view.controls).sort()).toEqual([
      "callAgain",
      "cancel",
      "messageChild",
      "openChildSession",
    ]);
  });
});

describe("buildCallDetailView — provenance and outcome", () => {
  it("Should surface an extracted verdict as extracted, never as returned", () => {
    const view = buildCallDetailView({
      call: call({ verdict: "extracted", settled_at: "2026-08-20T18:20:41Z" }),
    });
    expect(view.verdict).toBe("extracted");
    expect(view.timeline.map(event => event.id)).toContain("extracted");
    expect(view.timeline.find(event => event.id === "settled")!.detail).toBe(
      "the answer was recovered from its last message and checked"
    );
  });

  it("Should record the repair round when the answer was fixed on the second try", () => {
    const view = buildCallDetailView({
      call: call({ verdict: "repaired", repair_attempts: 1, started_at: "2026-08-20T18:31:02Z" }),
    });
    const repair = view.timeline.find(event => event.id === "repair");
    expect(repair).toBeDefined();
    expect(repair!.tone).toBe("warning");
  });

  it("Should keep the validator's own words for an invalid result", () => {
    const view = buildCallDetailView({
      call: call({
        state: "invalid-result",
        verdict: undefined,
        repair_attempts: 1,
        failure_code: "call_result_invalid",
        failure_detail: "/verdict: required property missing",
        first_issue_text: '/findings/0/line: expected number, got string "eighty-eight"',
        second_issue_text: "/verdict: required property missing",
        settled_at: "2026-08-20T18:41:03Z",
      }),
    });
    expect(view.result).toEqual({
      kind: "invalid",
      repairAttempts: 1,
      firstIssueText: '/findings/0/line: expected number, got string "eighty-eight"',
      secondIssueText: "/verdict: required property missing",
    });
    // The attempts panel owns the verbatim text; the timeline says what happened
    // so the same sentence does not print twice on one screen.
    expect(view.timeline.find(event => event.id === "settled")!.detail).toBe(
      "the answer did not match after 2 tries"
    );
  });

  it("Should state a silent finish rather than rendering an empty result pane", () => {
    const view = buildCallDetailView({
      call: call({
        state: "completed-without-result",
        verdict: undefined,
        strict: true,
        settled_at: "2026-08-20T18:50:00Z",
      }),
    });
    expect(view.result).toEqual({ kind: "none", strict: true, prosePreview: null });
  });

  it("Should report a pending result while the call is still running", () => {
    const view = buildCallDetailView({
      call: call({ state: "running", verdict: undefined, result_preview: undefined }),
    });
    expect(view.result.kind).toBe("pending");
  });
});

describe("buildCallDetailView — clocks", () => {
  it("Should say the idle clock is suspended while the call is in flight", () => {
    const view = buildCallDetailView({
      call: call({ state: "running", verdict: undefined }),
    });
    expect(view.idleTtl).toEqual({ kind: "suspended" });
  });

  it("Should show no deadline chrome when nobody set one", () => {
    expect(buildCallDetailView({ call: call() }).deadlineAt).toBeNull();
    expect(
      buildCallDetailView({ call: call({ deadline_at: "2026-08-20T19:00:00Z" }) }).deadlineAt
    ).toBe("2026-08-20T19:00:00Z");
  });
});

describe("buildCallDetailView — the answer", () => {
  it("Should call a stored payload stored, not absent, when nothing was inlined", () => {
    // `result_bytes` is the fact; the preview is a convenience. Reading absence
    // off the preview reports "the child recorded nothing" about 800 KB.
    const view = buildCallDetailView({
      call: call({ result_preview: undefined, result_bytes: 831_488 }),
    });
    expect(view.result).toEqual({
      kind: "stored",
      bytes: 831_488,
      budgetBytes: 262_144,
      overflow: "store",
    });
  });

  it("Should still report a genuinely resultless terminal as none", () => {
    const view = buildCallDetailView({
      call: call({ result_preview: undefined, result_bytes: 0 }),
    });
    expect(view.result).toMatchObject({ kind: "none", strict: false });
  });

  it("Should mark a preview bounded only when it is smaller than what is stored", () => {
    const complete = buildCallDetailView({
      call: call({ result_preview: { value: "ok" }, result_bytes: 14 }),
    });
    const clipped = buildCallDetailView({
      call: call({ result_preview: { value: "ok" }, result_bytes: 4_096 }),
    });
    expect(complete.result).toMatchObject({ kind: "value", bounded: false });
    expect(clipped.result).toMatchObject({ kind: "value", bounded: true });
  });
});

describe("buildCallDetailView — the ask", () => {
  it("Should show no prompt block when the call carried no instruction", () => {
    expect(buildCallDetailView({ call: call({ prompt_bytes: 0 }) }).prompt).toBeNull();
  });

  it("Should measure the preview in bytes, so accented text is not called truncated", () => {
    // `"Revisão"` is 7 UTF-16 code units but 8 UTF-8 bytes, and `"レビュー"` is
    // 4 against 12. Comparing `.length` to a Go byte count would put a
    // "there is more" notice on a prompt that is already complete.
    for (const prompt of ["Revisão", "レビュー", "plain ascii"]) {
      const bytes = new TextEncoder().encode(prompt).length;
      const view = buildCallDetailView({
        call: call({ prompt_preview: prompt, prompt_bytes: bytes }),
      });
      expect(view.prompt).toEqual({ preview: prompt, bytes, bounded: false });
    }
  });

  it("Should mark the ask bounded when the daemon held more than it inlined", () => {
    const view = buildCallDetailView({
      call: call({ prompt_preview: "Review the checkout", prompt_bytes: 12_000 }),
    });
    expect(view.prompt).toMatchObject({ bounded: true, bytes: 12_000 });
  });

  it("Should offer the fetch for a prompt stored entirely behind a reference", () => {
    // No preview at all is not "no prompt" — it is a prompt worth fetching.
    const view = buildCallDetailView({
      call: call({ prompt_preview: undefined, prompt_bytes: 44 }),
    });
    expect(view.prompt).toEqual({ preview: null, bytes: 44, bounded: true });
  });
});

describe("buildCallResultShape", () => {
  it("Should flatten a contracted result into copyable paths", () => {
    const shape = buildCallResultShape({
      verdict: "needs-changes",
      findings: [{ file: "internal/loop/action.go", line: 88 }],
    });
    if (shape.kind !== "rows") throw new Error(`expected rows, got ${shape.kind}`);
    expect(shape.rows).toEqual([
      { path: "verdict", value: '"needs-changes"', summary: false },
      { path: "findings[0].file", value: '"internal/loop/action.go"', summary: false },
      { path: "findings[0].line", value: "88", summary: false },
    ]);
  });

  it("Should summarize a long array instead of enumerating it", () => {
    const shape = buildCallResultShape({ files: Array.from({ length: 200 }, (_, i) => `f${i}`) });
    if (shape.kind !== "rows") throw new Error("expected rows");
    const summary = shape.rows.at(-1)!;
    expect(summary.summary).toBe(true);
    expect(summary.path).toBe("files[0..199]");
    expect(summary.value).toBe("200 records · first 3 shown in preview");
  });

  it("Should treat an admitted null as a value, not as a missing result", () => {
    expect(buildCallResultShape({ verdict: null })).toEqual({
      kind: "rows",
      rows: [{ path: "verdict", value: "null", summary: false }],
      truncated: false,
    });
    expect(buildCallResultShape(undefined)).toEqual({ kind: "absent" });
  });

  it("Should render valid empty containers instead of a blank result", () => {
    expect(buildCallResultShape({})).toEqual({ kind: "scalar", value: "{}" });
    expect(buildCallResultShape([])).toEqual({ kind: "scalar", value: "[]" });
  });
});
