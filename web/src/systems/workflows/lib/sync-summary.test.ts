import { describe, expect, it } from "vitest";

import { formatWorkflowSyncResult } from "./sync-summary";

describe("formatWorkflowSyncResult", () => {
  it("Should summarize scanned workflows without stale removals", () => {
    expect(formatWorkflowSyncResult({ workflows_scanned: 2 })).toBe(
      "Sync completed - 2 workflows scanned."
    );
  });

  it("Should include pruned workflow count when stale workflows were removed", () => {
    expect(formatWorkflowSyncResult({ workflows_scanned: 8, workflows_pruned: 1 })).toBe(
      "Sync completed - 8 workflows scanned, 1 stale workflow removed."
    );
  });
});
