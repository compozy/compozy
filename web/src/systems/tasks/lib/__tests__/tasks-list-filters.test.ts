import { describe, expect, it, vi } from "vitest";

import {
  applyTaskFilterChips,
  buildTaskFilterFields,
  taskOwnerFilterValue,
  taskFiltersToChips,
} from "../tasks-list-filters";

describe("taskFiltersToChips", () => {
  it("Should omit chips when filters are unset", () => {
    expect(
      taskFiltersToChips({
        statusFilter: null,
        ownerFilter: null,
        priorityFilter: null,
      })
    ).toEqual([]);
  });

  it("Should emit one chip per active filter with operator 'is' and a single value", () => {
    const chips = taskFiltersToChips({
      statusFilter: "in_progress",
      ownerFilter: { kind: "agent_session", ref: "Coder" },
      priorityFilter: "high",
    });

    expect(chips).toHaveLength(3);
    const status = chips.find(chip => chip.field === "status");
    expect(status?.operator).toBe("is");
    expect(status?.values).toEqual(["in_progress"]);
    expect(chips.find(chip => chip.field === "owner")?.values).toEqual([
      taskOwnerFilterValue({ kind: "agent_session", ref: "Coder" }),
    ]);
    expect(chips.find(chip => chip.field === "priority")?.values).toEqual(["high"]);
  });

  it("Should keep chip ids stable across renders for the same field", () => {
    const first = taskFiltersToChips({
      statusFilter: "ready",
      ownerFilter: null,
      priorityFilter: null,
    });
    const second = taskFiltersToChips({
      statusFilter: "ready",
      ownerFilter: null,
      priorityFilter: null,
    });

    expect(first[0]?.id).toBe(second[0]?.id);
  });
});

describe("applyTaskFilterChips", () => {
  it("Should reset every slot to its default when no chips are present", () => {
    const handlers = {
      onStatusChange: vi.fn(),
      onOwnerChange: vi.fn(),
      onPriorityChange: vi.fn(),
    };

    applyTaskFilterChips([], handlers);
    expect(handlers.onStatusChange).toHaveBeenCalledWith(null);
    expect(handlers.onOwnerChange).toHaveBeenCalledWith(null);
    expect(handlers.onPriorityChange).toHaveBeenCalledWith(null);
  });

  it("Should route chip values to the matching typed setter", () => {
    const handlers = {
      onStatusChange: vi.fn(),
      onOwnerChange: vi.fn(),
      onPriorityChange: vi.fn(),
    };

    applyTaskFilterChips(
      [
        { id: "task-filter-status", field: "status", operator: "is", values: ["blocked"] },
        { id: "task-filter-priority", field: "priority", operator: "is", values: ["urgent"] },
        {
          id: "task-filter-owner",
          field: "owner",
          operator: "is",
          values: [taskOwnerFilterValue({ kind: "agent_session", ref: "Coder" })],
        },
      ],
      handlers
    );

    expect(handlers.onStatusChange).toHaveBeenCalledWith("blocked");
    expect(handlers.onPriorityChange).toHaveBeenCalledWith("urgent");
    expect(handlers.onOwnerChange).toHaveBeenCalledWith({
      kind: "agent_session",
      ref: "Coder",
    });
  });

  it("Should drop invalid values from the typed setters", () => {
    const handlers = {
      onStatusChange: vi.fn(),
      onOwnerChange: vi.fn(),
      onPriorityChange: vi.fn(),
    };

    applyTaskFilterChips(
      [
        { id: "task-filter-status", field: "status", operator: "is", values: ["bogus"] },
        { id: "task-filter-priority", field: "priority", operator: "is", values: ["bogus"] },
      ],
      handlers
    );

    expect(handlers.onStatusChange).toHaveBeenCalledWith(null);
    expect(handlers.onPriorityChange).toHaveBeenCalledWith(null);
  });
});

describe("buildTaskFilterFields", () => {
  it("Should expose status, priority, and owner as single-select chip fields", () => {
    const fields = buildTaskFilterFields([{ ref: "Coder", kind: "agent_session" }]);
    expect(fields).toHaveLength(3);
    const keys = (fields as Array<{ key?: string }>).map(field => field.key);
    expect(keys).toEqual(["status", "priority", "owner"]);
  });

  it("Should expose needs_attention (distinct from blocked) as a filterable status option", () => {
    const fields = buildTaskFilterFields([]);
    const status = (fields as Array<{ key?: string; options?: Array<{ value: string }> }>).find(
      field => field.key === "status"
    );
    const values = status?.options?.map(option => option.value) ?? [];
    expect(values).toContain("needs_attention");
    expect(values).toContain("blocked");
  });

  it("Should mirror the live owner options inside the owner field", () => {
    const fields = buildTaskFilterFields([
      { ref: "pedro@", kind: "human" },
      { ref: "Coder", kind: "agent_session" },
    ]);

    const owner = (fields as Array<{ key?: string; options?: Array<{ value: string }> }>).find(
      field => field.key === "owner"
    );
    expect(owner?.options?.map(option => option.value)).toEqual([
      taskOwnerFilterValue({ kind: "human", ref: "pedro@" }),
      taskOwnerFilterValue({ kind: "agent_session", ref: "Coder" }),
    ]);
  });

  it("Should preserve owner kind when distinct facets share the same ref", () => {
    const fields = buildTaskFilterFields([
      { kind: "human", ref: "operator" },
      { kind: "agent_session", ref: "operator" },
    ]);
    const owner = fields.find(field => "key" in field && field.key === "owner");
    if (!owner || !("key" in owner)) throw new Error("expected the owner filter field");
    const options = owner?.options ?? [];

    expect(options.map(option => option.value)).toEqual([
      taskOwnerFilterValue({ kind: "human", ref: "operator" }),
      taskOwnerFilterValue({ kind: "agent_session", ref: "operator" }),
    ]);
    expect(new Set(options.map(option => option.value)).size).toBe(2);
    expect(options.map(option => option.label)).toEqual(["operator · Human", "operator · Agent"]);

    const selectedOwner = options[1];
    if (!selectedOwner) throw new Error("expected the agent owner option");
    const onOwnerChange = vi.fn();
    applyTaskFilterChips(
      [
        {
          id: "task-filter-owner",
          field: "owner",
          operator: "is",
          values: [selectedOwner.value],
        },
      ],
      { onOwnerChange, onPriorityChange: vi.fn(), onStatusChange: vi.fn() }
    );
    expect(onOwnerChange).toHaveBeenCalledWith({ kind: "agent_session", ref: "operator" });
  });
});
