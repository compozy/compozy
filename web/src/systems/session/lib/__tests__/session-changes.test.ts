import { describe, expect, it } from "vitest";

import { isLargeDiff } from "../session-changes";

describe("isLargeDiff", () => {
  // UT-056. Invariant: one-file mode starts strictly past 200 files or 20,000 changed lines.
  it.each([
    [{ files: 200, additions: 12_000, deletions: 8_000 }, false],
    [{ files: 201, additions: 1, deletions: 0 }, true],
    [{ files: 3, additions: 20_000, deletions: 1 }, true],
    [{ files: 3, additions: 10_000, deletions: 10_001 }, true],
  ])("Should classify totals %j as large=%s", (totals, large) => {
    expect(isLargeDiff(totals)).toBe(large);
  });
});
