import { describe, expect, it } from "vitest";

import {
  subagentCardLines,
  subagentChipPreview,
  subagentChipState,
  subagentCounts,
  subagentElapsedClock,
  subagentGroupClock,
  subagentGroupSummary,
  subagentHoverPreview,
  subagentLocationRows,
  subagentRosterGroups,
  subagentRosterTitle,
  subagentRuntimeLabel,
  subagentWaitingBannerRows,
  SUBAGENT_STATUS_GLYPH,
  SUBAGENT_STATUS_WORD,
} from "../subagent-format";
import type { SubagentStatus, SubagentView } from "../types";

// Suite: subagent presentation rules (pure).
// Invariant: card lines, elapsed source/format, group copy/span, hover content, banner
// visibility, chip state and roster order follow `_uiux.md` S1–S3, S5, S9, S10 from the
// row's own server fields. Owning layer: session subagent view rules. Canonical suite: this file.
const NO_RUNTIME = { agent: "", provider: "", model: "", reasoning_effort: "", speed: "" };
const T0 = "2026-10-08T21:00:00.000Z";
const at = (seconds: number) => new Date(Date.parse(T0) + seconds * 1_000).toISOString();

function subagent(overrides: Partial<SubagentView> = {}): SubagentView {
  return {
    id: "sub-1",
    parent_session_id: "sess-parent",
    result_preview: "",
    error: null,
    child_session_id: "sess-child",
    origin: "delegated",
    title: "Audit payment webhooks for retry safety",
    status: "running",
    progress: "",
    runtime: {
      ...NO_RUNTIME,
      provider: "claude",
      model: "opus-5.5",
      reasoning_effort: "high",
      speed: "normal",
    },
    started_at: T0,
    settled_at: null,
    created_at: T0,
    updated_at: T0,
    delivery: "none",
    ...overrides,
  };
}

const withStatus = (status: SubagentStatus, id: string, created = 0) =>
  subagent({
    id,
    status,
    created_at: at(created),
    settled_at: ["queued", "running", "waiting"].includes(status) ? null : at(60),
  });

describe("subagentCardLines (UT-W05)", () => {
  it("Should show live progress on line 2 with the status word beside the title", () => {
    expect(
      subagentCardLines(subagent({ progress: "Reading   internal/payments/webhook.go" }))
    ).toEqual({
      titleWord: "Running",
      lineTwo: "Reading internal/payments/webhook.go",
      lineTwoTone: "default",
    });
  });

  it("Should fall back to the status word on line 2 while live without progress", () => {
    expect(subagentCardLines(subagent({ progress: "" }))).toEqual({
      titleWord: null,
      lineTwo: "Running",
      lineTwoTone: "word",
    });
    expect(subagentCardLines(subagent({ status: "waiting" })).lineTwo).toBe("Waiting for you");
  });

  it("Should show the result's first line once completed, without a title word", () => {
    const lines = subagentCardLines(
      subagent({
        status: "completed",
        result_preview: "\n\nDrafted notes: 6 changes.\nMore detail",
      })
    );
    expect(lines).toEqual({
      titleWord: null,
      lineTwo: "Drafted notes: 6 changes.",
      lineTwoTone: "default",
    });
  });

  it("Should show the error's first line in danger when failed", () => {
    expect(
      subagentCardLines(subagent({ status: "failed", error: "3 of 41 specs failed\nstack…" }))
    ).toEqual({ titleWord: "Failed", lineTwo: "3 of 41 specs failed", lineTwoTone: "danger" });
    expect(subagentCardLines(subagent({ status: "failed" }))).toEqual({
      titleWord: null,
      lineTwo: "Failed",
      lineTwoTone: "danger",
    });
  });

  it("Should keep the fixed status word list and glyph canon", () => {
    expect(Object.values(SUBAGENT_STATUS_WORD)).toEqual([
      "Queued",
      "Running",
      "Waiting for you",
      "Completed",
      "Failed",
      "Canceled",
      "Interrupted",
    ]);
    expect(SUBAGENT_STATUS_GLYPH).toEqual({
      queued: "queued",
      running: "running",
      waiting: "attention",
      completed: "done",
      failed: "failed",
      canceled: "stopped",
      interrupted: "stopped",
    });
  });
});

describe("subagent rows read agent markdown as plain text (D-09)", () => {
  it("Should strip markdown from card line 2 and the hover preview", () => {
    expect(
      subagentCardLines(
        subagent({
          status: "completed",
          settled_at: at(9),
          result_preview: "# API Versioning\nbody",
        })
      ).lineTwo
    ).toBe("API Versioning");
    expect(subagentCardLines(subagent({ progress: "**Planning** the `fix`" })).lineTwo).toBe(
      "Planning the fix"
    );
    expect(
      subagentHoverPreview(
        subagent({
          status: "completed",
          settled_at: at(9),
          result_preview:
            "**Yes—ship it.**\n- see [the PR](https://x.test/1)\nkeep snake_case_names",
        })
      )
    ).toBe("Yes—ship it. see the PR keep snake_case_names");
  });
});

describe("subagent elapsed (UT-W06)", () => {
  it("Should freeze at settled_at and tick from started_at only while live", () => {
    expect(subagentElapsedClock(subagent({ status: "completed", settled_at: at(52) }))).toEqual({
      kind: "frozen",
      ms: 52_000,
    });
    expect(subagentElapsedClock(subagent())).toEqual({ kind: "ticking", startMs: Date.parse(T0) });
    expect(subagentElapsedClock(subagent({ started_at: null }))).toEqual({ kind: "none" });
  });

  it("Should not tick a stale row: it freezes at the server's updated_at", () => {
    expect(subagentElapsedClock(subagent({ updated_at: at(161) }), { stale: true })).toEqual({
      kind: "frozen",
      ms: 161_000,
    });
  });
});

describe("subagent group (UT-W07)", () => {
  it("Should count states in words with tone info while live and danger when any failed", () => {
    const mixed = [
      withStatus("running", "a"),
      withStatus("queued", "b"),
      withStatus("completed", "c"),
    ];
    expect(subagentGroupSummary(mixed)).toEqual({
      label: "3 subagents",
      summary: "2 working · 1 done",
      tone: "live",
      settled: false,
    });
    const failed = [...mixed.slice(1), withStatus("failed", "d")];
    expect(subagentGroupSummary(failed).summary).toBe("1 working · 1 done · 1 failed");
    expect(subagentGroupSummary(failed).tone).toBe("failed");
  });

  it("Should add N needs you for a waiting member", () => {
    const group = [
      withStatus("running", "a"),
      withStatus("waiting", "b"),
      withStatus("completed", "c"),
    ];
    expect(subagentGroupSummary(group).summary).toBe("1 working · 1 needs you · 1 done");
    expect(subagentGroupSummary(group).tone).toBe("live");
  });

  it("Should span first start to last settle and withhold it while any end is unknown", () => {
    const settled = [
      subagent({ id: "a", status: "completed", started_at: at(10), settled_at: at(70) }),
      subagent({ id: "b", status: "failed", started_at: at(0), settled_at: at(305) }),
    ];
    expect(subagentGroupClock(settled)).toEqual({ kind: "frozen", ms: 305_000 });
    expect(subagentGroupClock([...settled, subagent({ id: "c", started_at: at(5) })])).toEqual({
      kind: "ticking",
      startMs: Date.parse(T0),
    });
    expect(subagentGroupClock([...settled, subagent({ id: "c" })], { stale: true })).toEqual({
      kind: "none",
    });
    expect(subagentGroupClock([...settled, subagent({ id: "c", started_at: null })])).toEqual({
      kind: "none",
    });
  });
});

describe("subagent hover content (UT-W08)", () => {
  it("Should render a missing model as Not reported and drop the effort", () => {
    expect(
      subagentRuntimeLabel(
        subagent({ runtime: { ...NO_RUNTIME, provider: "claude", reasoning_effort: "high" } })
      )
    ).toEqual({
      model: null,
      effort: null,
      fast: false,
    });
    expect(
      subagentRuntimeLabel(
        subagent({ runtime: { ...NO_RUNTIME, model: "gpt-5.6-sol", speed: "fast" } })
      )
    ).toEqual({
      model: "gpt-5.6-sol",
      effort: null,
      fast: true,
    });
  });

  it("Should collapse whitespace and cap the preview at 280 characters plus an ellipsis", () => {
    const long = `Found   2 N+1\nqueries. ${"x".repeat(400)}`;
    const preview = subagentHoverPreview(subagent({ status: "completed", result_preview: long }));
    expect(preview.startsWith("Found 2 N+1 queries.")).toBe(true);
    expect(preview).toHaveLength(281);
    expect(preview.endsWith("…")).toBe(true);
    expect(subagentHoverPreview(subagent({ status: "failed", error: "timed out" }))).toBe(
      "timed out"
    );
  });

  it("Should list workspace or worktree only where the child differs from its parent", () => {
    const parent = { workspace: "compozy", worktree: "main" };
    expect(subagentLocationRows({ workspace: "compozy", worktree: "main" }, parent)).toEqual([]);
    expect(
      subagentLocationRows({ workspace: "compozy", worktree: "subagent/enum" }, parent)
    ).toEqual([{ label: "Worktree", value: "subagent/enum" }]);
  });
});

describe("subagentWaitingBannerRows (UT-W13)", () => {
  const rows = [
    withStatus("running", "a"),
    subagent({ id: "native", origin: "provider_native", child_session_id: null }),
    withStatus("completed", "c"),
  ];

  it("Should list live delegated rows only while the parent has no running turn", () => {
    expect(subagentWaitingBannerRows(rows, false).map(row => row.id)).toEqual(["a"]);
    expect(subagentWaitingBannerRows(rows, true)).toEqual([]);
  });

  it("Should stay hidden when only provider-native rows are live", () => {
    expect(subagentWaitingBannerRows(rows.slice(1), false)).toEqual([]);
  });
});

describe("subagentChipState (UT-W17)", () => {
  it("Should read live/total with the glyph by urgency", () => {
    expect(subagentChipState({ live: 3, total: 10, failed: 1, attention: 0 }, true)).toEqual({
      glyph: "failed",
      text: "3/10",
      ariaLabel: "3 of 10 subagents running",
    });
    expect(subagentChipState({ live: 3, total: 10, failed: 1, attention: 1 }, true)).toEqual({
      glyph: "attention",
      text: "3/10",
      ariaLabel: "3 of 10 subagents running, 1 needs you",
    });
    expect(subagentChipState({ live: 2, total: 10, failed: 0, attention: 0 }, true)?.glyph).toBe(
      "running"
    );
  });

  it("Should read delegated when the parent is idle with live subagents", () => {
    expect(subagentChipState({ live: 2, total: 4, failed: 0, attention: 0 }, false)?.glyph).toBe(
      "delegated"
    );
  });

  it("Should count a waiting subagent as live", () => {
    const counts = subagentCounts([withStatus("waiting", "a"), withStatus("completed", "b")]);
    expect(counts).toEqual({ live: 1, total: 2, failed: 0, attention: 1 });
    expect(subagentChipState(counts, false)?.text).toBe("1/2");
  });

  it("Should show the total alone when nothing is live but failures remain", () => {
    expect(subagentChipState({ live: 0, total: 6, failed: 2, attention: 0 }, false)).toEqual({
      glyph: "failed",
      text: "6",
      ariaLabel: "6 subagents, 2 failed",
    });
  });

  it("Should hide once everything settled without failure or attention", () => {
    expect(subagentChipState({ live: 0, total: 6, failed: 0, attention: 0 }, false)).toBeNull();
  });

  it("Should preview the five most urgent then +N more", () => {
    const rows = [
      withStatus("completed", "done", 9),
      withStatus("running", "run-old", 1),
      withStatus("running", "run-new", 5),
      withStatus("failed", "failed", 2),
      withStatus("waiting", "waiting", 0),
      withStatus("queued", "queued", 3),
      withStatus("canceled", "canceled", 4),
    ];
    const preview = subagentChipPreview(rows, 10);
    expect(preview.rows.map(row => row.id)).toEqual([
      "waiting",
      "failed",
      "run-new",
      "run-old",
      "queued",
    ]);
    expect(preview.more).toBe(5);
  });
});

describe("subagentRosterGroups (UT-W19)", () => {
  it("Should pin failed rows, then live rows waiting first, then previous rows newest first", () => {
    const groups = subagentRosterGroups([
      withStatus("completed", "done-old", 1),
      withStatus("running", "run", 6),
      withStatus("failed", "failed", 2),
      withStatus("waiting", "waiting", 3),
      withStatus("canceled", "canceled", 5),
      withStatus("interrupted", "interrupted", 4),
    ]);
    expect(groups.failed.map(row => row.id)).toEqual(["failed"]);
    expect(groups.live.map(row => row.id)).toEqual(["waiting", "run"]);
    expect(groups.previous.map(row => row.id)).toEqual(["canceled", "interrupted", "done-old"]);
  });

  it("Should title the section with the running count", () => {
    expect(subagentRosterTitle(2)).toBe("Subagents · 2 running");
    expect(subagentRosterTitle(0)).toBe("Subagents");
  });
});
