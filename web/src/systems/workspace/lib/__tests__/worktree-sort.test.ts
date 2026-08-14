// Suite: worktree nest ordering
// Invariant (UT-134, pure half): the locked sort is state group then last-activity descending with
// unknown last; truncation keeps the most recently active five and exposes every hidden row.
// Owning layer: workspace sort lib. Canonical suite: this lib test.
import { describe, expect, it } from "vitest";

import type { WorktreeNestEntry } from "../worktree-display";
import {
  adoptedWorktreeCount,
  runningAgentCount,
  sortWorktreeNestEntries,
  truncateWorktreeNest,
} from "../worktree-sort";

function entry(overrides: Partial<WorktreeNestEntry> & { key: string }): WorktreeNestEntry {
  return {
    kind: "worktree",
    displayState: "ready",
    name: overrides.key,
    branch: `pedro/${overrides.key}`,
    path: `/worktrees/${overrides.key}`,
    activityAt: null,
    selectable: true,
    inertReason: null,
    adoptable: false,
    worktree: null,
    discovered: null,
    ...overrides,
  };
}

describe("sortWorktreeNestEntries", () => {
  it("Should order by state group before activity", () => {
    const sorted = sortWorktreeNestEntries([
      entry({ key: "missing", displayState: "missing", activityAt: "2026-04-17T18:00:00Z" }),
      entry({ key: "discovered", displayState: "discovered" }),
      entry({ key: "pending", displayState: "pending", activityAt: "2026-04-10T09:00:00Z" }),
      entry({ key: "ready", displayState: "ready", activityAt: "2026-04-01T09:00:00Z" }),
    ]);

    // A very recent missing row still sorts below a stale ready row.
    expect(sorted.map(item => item.key)).toEqual(["ready", "pending", "discovered", "missing"]);
  });

  it("Should order within a group by last activity descending", () => {
    const sorted = sortWorktreeNestEntries([
      entry({ key: "older", activityAt: "2026-04-10T09:00:00Z" }),
      entry({ key: "newest", activityAt: "2026-04-17T17:00:00Z" }),
      entry({ key: "middle", activityAt: "2026-04-15T09:00:00Z" }),
    ]);

    expect(sorted.map(item => item.key)).toEqual(["newest", "middle", "older"]);
  });

  it("Should sort unknown activity last rather than treating it as oldest-known", () => {
    const sorted = sortWorktreeNestEntries([
      entry({ key: "unknown", activityAt: null }),
      entry({ key: "known", activityAt: "2026-04-01T09:00:00Z" }),
    ]);

    expect(sorted.map(item => item.key)).toEqual(["known", "unknown"]);
  });

  it("Should treat an unparseable stamp as unknown instead of the epoch", () => {
    const sorted = sortWorktreeNestEntries([
      entry({ key: "garbled", activityAt: "not-a-date" }),
      entry({ key: "known", activityAt: "2026-04-01T09:00:00Z" }),
    ]);

    expect(sorted.map(item => item.key)).toEqual(["known", "garbled"]);
  });

  it("Should be stable on ties so the order never depends on input order", () => {
    const forward = sortWorktreeNestEntries([
      entry({ key: "beta", activityAt: "2026-04-01T09:00:00Z" }),
      entry({ key: "alpha", activityAt: "2026-04-01T09:00:00Z" }),
    ]);
    const reverse = sortWorktreeNestEntries([
      entry({ key: "alpha", activityAt: "2026-04-01T09:00:00Z" }),
      entry({ key: "beta", activityAt: "2026-04-01T09:00:00Z" }),
    ]);

    expect(forward.map(item => item.key)).toEqual(["alpha", "beta"]);
    expect(reverse.map(item => item.key)).toEqual(["alpha", "beta"]);
  });
});

describe("truncateWorktreeNest", () => {
  it("Should keep the first five and report the overflow remainder", () => {
    const entries = Array.from({ length: 8 }, (_, index) => entry({ key: `wt-${index}` }));

    const { visible, overflowCount } = truncateWorktreeNest(entries);

    expect(visible).toHaveLength(5);
    expect(visible.map(item => item.key)).toEqual(["wt-0", "wt-1", "wt-2", "wt-3", "wt-4"]);
    expect(overflowCount).toBe(3);
  });

  it("Should render no overflow row when the nest fits", () => {
    const { visible, overflowCount } = truncateWorktreeNest([entry({ key: "only" })]);

    expect(visible).toHaveLength(1);
    expect(overflowCount).toBe(0);
  });

  it("Should expose overflow when hidden rows are discovered entries", () => {
    const entries = [
      entry({ key: "adopted-a" }),
      entry({ key: "adopted-b" }),
      ...Array.from({ length: 6 }, (_, index) =>
        entry({ key: `found-${index}`, kind: "discovered", displayState: "discovered" })
      ),
    ];

    const { visible, overflowCount } = truncateWorktreeNest(entries);

    expect(visible).toHaveLength(5);
    expect(overflowCount).toBe(3);
  });

  it("Should render the overflow row when adopted records overflow the fold", () => {
    const entries = [
      ...Array.from({ length: 6 }, (_, index) => entry({ key: `adopted-${index}` })),
      entry({ key: "found", kind: "discovered", displayState: "discovered" }),
    ];

    const { visible, overflowCount } = truncateWorktreeNest(entries);

    expect(visible).toHaveLength(5);
    expect(overflowCount).toBe(2);
  });
});

describe("adoptedWorktreeCount", () => {
  it("Should exclude discovered entries from the count", () => {
    const entries = [
      entry({ key: "adopted-a" }),
      entry({ key: "adopted-b", displayState: "missing" }),
      entry({ key: "discovered", kind: "discovered", displayState: "discovered" }),
    ];

    // A discovered checkout has no record, so it is not a worktree count.
    expect(adoptedWorktreeCount(entries)).toBe(2);
  });
});

describe("runningAgentCount", () => {
  it("Should count only worktrees whose agent is actually running", () => {
    const running = { agent_activity: "running" } as WorktreeNestEntry["worktree"];
    const idle = { agent_activity: "idle" } as WorktreeNestEntry["worktree"];
    const entries = [
      entry({ key: "a", worktree: running }),
      entry({ key: "b", worktree: idle }),
      entry({ key: "c" }),
    ];

    expect(runningAgentCount(entries)).toBe(1);
  });
});
