import { describe, expect, it, vi } from "vitest";

import type { InboxUiLane } from "../inbox-grouping";
import {
  applyInboxFilterChips,
  buildInboxFilterFields,
  type InboxLaneCount,
  inboxFiltersToChips,
} from "../inbox-filters";

function emptyLaneCounts(): Map<InboxUiLane, InboxLaneCount> {
  return new Map();
}

describe("inboxFiltersToChips", () => {
  it("Should omit chips when filters are at their default state", () => {
    expect(
      inboxFiltersToChips({
        laneFilter: "all",
        statusFilter: null,
        priorityFilter: null,
      })
    ).toEqual([]);
  });

  it("Should emit one chip per non-default slot with operator 'is' and a single value", () => {
    const chips = inboxFiltersToChips({
      laneFilter: "approvals",
      statusFilter: "failed",
      priorityFilter: "urgent",
    });

    expect(chips).toHaveLength(3);
    const lane = chips.find(chip => chip.field === "lane");
    expect(lane?.operator).toBe("is");
    expect(lane?.values).toEqual(["approvals"]);
    expect(chips.find(chip => chip.field === "status")?.values).toEqual(["failed"]);
    expect(chips.find(chip => chip.field === "priority")?.values).toEqual(["urgent"]);
  });

  it("Should keep chip ids stable across renders for the same field", () => {
    const first = inboxFiltersToChips({
      laneFilter: "blocked",
      statusFilter: null,
      priorityFilter: null,
    });
    const second = inboxFiltersToChips({
      laneFilter: "blocked",
      statusFilter: null,
      priorityFilter: null,
    });

    expect(first[0]?.id).toBe(second[0]?.id);
  });
});

describe("applyInboxFilterChips", () => {
  it("Should reset every slot to its default when no chips are present", () => {
    const handlers = {
      onLaneChange: vi.fn(),
      onStatusChange: vi.fn(),
      onPriorityChange: vi.fn(),
    };

    applyInboxFilterChips([], handlers);
    expect(handlers.onLaneChange).toHaveBeenCalledWith("all");
    expect(handlers.onStatusChange).toHaveBeenCalledWith(null);
    expect(handlers.onPriorityChange).toHaveBeenCalledWith(null);
  });

  it("Should route chip values to the matching typed setter", () => {
    const handlers = {
      onLaneChange: vi.fn(),
      onStatusChange: vi.fn(),
      onPriorityChange: vi.fn(),
    };

    applyInboxFilterChips(
      [
        { id: "inbox-filter-lane", field: "lane", operator: "is", values: ["approvals"] },
        { id: "inbox-filter-status", field: "status", operator: "is", values: ["blocked"] },
        { id: "inbox-filter-priority", field: "priority", operator: "is", values: ["high"] },
      ],
      handlers
    );

    expect(handlers.onLaneChange).toHaveBeenCalledWith("approvals");
    expect(handlers.onStatusChange).toHaveBeenCalledWith("blocked");
    expect(handlers.onPriorityChange).toHaveBeenCalledWith("high");
  });

  it("Should drop invalid values from the typed setters", () => {
    const handlers = {
      onLaneChange: vi.fn(),
      onStatusChange: vi.fn(),
      onPriorityChange: vi.fn(),
    };

    applyInboxFilterChips(
      [
        { id: "inbox-filter-lane", field: "lane", operator: "is", values: ["bogus"] },
        { id: "inbox-filter-status", field: "status", operator: "is", values: ["bogus"] },
        { id: "inbox-filter-priority", field: "priority", operator: "is", values: ["bogus"] },
      ],
      handlers
    );

    expect(handlers.onLaneChange).toHaveBeenCalledWith("all");
    expect(handlers.onStatusChange).toHaveBeenCalledWith(null);
    expect(handlers.onPriorityChange).toHaveBeenCalledWith(null);
  });
});

describe("buildInboxFilterFields", () => {
  it("Should expose lane, status, and priority as single-select chip fields", () => {
    const fields = buildInboxFilterFields(emptyLaneCounts());
    expect(fields).toHaveLength(3);
    const keys = (fields as Array<{ key?: string }>).map(field => field.key);
    expect(keys).toEqual(["lane", "status", "priority"]);
  });

  it("Should embed live counts in lane option labels when counts are non-zero", () => {
    const counts: Map<InboxUiLane, InboxLaneCount> = new Map([
      ["approvals", { count: 4, unread: 2 }],
      ["my_work", { count: 0, unread: 0 }],
    ]);

    const fields = buildInboxFilterFields(counts);
    const lane = (
      fields as Array<{
        key?: string;
        options?: Array<{ value: string; label: string }>;
      }>
    ).find(field => field.key === "lane");

    const approvalsLabel = lane?.options?.find(option => option.value === "approvals")?.label;
    const myWorkLabel = lane?.options?.find(option => option.value === "my_work")?.label;
    expect(approvalsLabel).toBe("Approvals · 4");
    expect(myWorkLabel).toBe("My work");
  });
});
