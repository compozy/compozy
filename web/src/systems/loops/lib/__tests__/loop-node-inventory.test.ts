// Suite: Loop node inventory projection
// Invariant: API timestamps are humanized before they reach operator-facing reason text.
// Boundary IN: one workspace-scoped inventory item and the caller's display clock.
// Boundary OUT: component rendering, paging, and query transport.

import { describe, expect, it } from "vitest";

import type { LoopNodeInventoryItem, LoopNodeInventoryState } from "../../types";
import { buildInventoryRow } from "../loop-node-inventory";

const NOW = Date.parse("2026-08-03T14:00:00Z");

function inventoryItem(
  state: LoopNodeInventoryState,
  overrides: Partial<LoopNodeInventoryItem>
): LoopNodeInventoryItem {
  return {
    state,
    loop_run_id: "run-1",
    loop_name: "review-and-fix",
    generation: 2,
    node_id: "publish",
    item_index: 0,
    state_at: "2026-08-03T13:55:00Z",
    ...overrides,
  };
}

describe("buildInventoryRow", () => {
  it("Should format timer and retry timestamps relative to the display clock", () => {
    const waiting = buildInventoryRow(
      inventoryItem("waiting", {
        wait: {
          loop_run_id: "run-1",
          generation: 2,
          node_id: "publish",
          item_index: 0,
          kind: "timer",
          escalation_cursor: 0,
          claim_state: "waiting",
          admission_failures: 0,
          issued_epoch: 1,
          created_at: "2026-08-03T13:55:00Z",
          age_seconds: 300,
          resume_at: "2026-08-03T14:05:00Z",
        },
      }),
      NOW
    );
    const retrying = buildInventoryRow(
      inventoryItem("retrying", {
        output: {
          node_id: "publish",
          status: "failed",
          attempt: 2,
          failure_class: "transport",
          next_attempt_at: "2026-08-03T14:05:00Z",
        },
      }),
      NOW
    );

    expect(waiting.reason).toBe("timer — resumes in 5m");
    expect(retrying.reason).toBe("attempt 2 · transport failure · next in 5m");
    expect(`${waiting.reason} ${retrying.reason}`).not.toContain("2026-08-03T14:05:00Z");
  });
});
