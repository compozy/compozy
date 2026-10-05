import { describe, expect, it, vi } from "vitest";

import { LOOP_RUN_STATUSES } from "@/generated/loop-enums";

import {
  applyLoopFilterChips,
  applyLoopRunFilterChips,
  buildLoopFilterFields,
  buildLoopRunFilterFields,
  loopFiltersToChips,
  loopRunFiltersToChips,
  loopStatusFilterOptions,
  parseLoopCategoryFilter,
  parseLoopKindFilter,
  parseLoopStatusFilter,
  type LoopFilterHandlers,
  type LoopRunFilterHandlers,
} from "../loop-list-filters";

describe("loop-list-filters", () => {
  it("Should offer every daemon status, including canceled, regardless of the loaded page", () => {
    const options = loopStatusFilterOptions();
    expect(options.map(option => option.value)).toEqual([...LOOP_RUN_STATUSES]);
    expect(options.map(option => option.value)).toContain("canceled");
    expect(options.find(option => option.value === "canceled")?.label).toBe("Canceled");
    expect(options.find(option => option.value === "needs-approval")?.label).toBe("Needs approval");
  });

  it("Should parse URL search values and reject unknowns", () => {
    expect(parseLoopKindFilter("workspace")).toBe("workspace");
    expect(parseLoopKindFilter("all")).toBeUndefined();
    expect(parseLoopKindFilter("builtin")).toBeUndefined();
    expect(parseLoopCategoryFilter(" delivery ")).toBe("delivery");
    expect(parseLoopCategoryFilter("")).toBeUndefined();
    expect(parseLoopStatusFilter("running")).toBe("running");
    expect(parseLoopStatusFilter("canceled")).toBe("canceled");
    expect(parseLoopStatusFilter("stop")).toBeUndefined();
    expect(parseLoopStatusFilter("mystery")).toBeUndefined();
  });
});

describe("buildLoopFilterFields", () => {
  // Invariant: all supported catalog filters are selectable; categories come from
  // server facets and status options retain the full daemon vocabulary.
  // Owner: Loop catalog chip projection, independent of query transport and page size.
  it("Should expose kind, server categories, and the full daemon status vocabulary", () => {
    const fields = buildLoopFilterFields(["Engineering", "Operations"], null);
    const [kind, category, status] = fields;
    if (
      !kind ||
      !("key" in kind) ||
      !category ||
      !("key" in category) ||
      !status ||
      !("key" in status)
    ) {
      throw new Error("loop filters must be flat select fields");
    }

    expect(fields).toHaveLength(3);
    expect(kind.key).toBe("kind");
    expect(kind.options).toEqual([
      { value: "read-only", label: "Built-in" },
      { value: "workspace", label: "Custom" },
    ]);
    expect(category.key).toBe("category");
    expect(category.options?.map(option => option.value)).toEqual(["Engineering", "Operations"]);
    expect(status.key).toBe("status");
    expect(status.type).toBe("select");
    expect(status.options?.map(option => option.value)).toEqual([...LOOP_RUN_STATUSES]);
    expect(status.options?.find(option => option.value === "canceled")?.label).toBe("Canceled");
  });

  it("Should retain the selected category when other filters exclude its facet", () => {
    const category = buildLoopFilterFields([], "Operations")[1];
    if (!category || !("key" in category)) throw new Error("category filter must be a select");
    expect(category.options).toEqual([{ value: "Operations", label: "Operations" }]);
  });
});

describe("loopFiltersToChips", () => {
  it("Should project concrete filters and omit unfiltered values", () => {
    expect(
      loopFiltersToChips({ kind: "workspace", category: "Operations", status: "canceled" })
    ).toEqual([
      { field: "kind", id: "loop-filter-kind", operator: "is", values: ["workspace"] },
      { field: "category", id: "loop-filter-category", operator: "is", values: ["Operations"] },
      { field: "status", id: "loop-filter-status", operator: "is", values: ["canceled"] },
    ]);
    expect(loopFiltersToChips({ kind: "all", category: null, status: null })).toEqual([]);
  });
});

describe("applyLoopFilterChips", () => {
  function createHandlers(): LoopFilterHandlers {
    return { onFiltersChange: vi.fn() };
  }

  it("Should apply all selected filters in one update", () => {
    const handlers = createHandlers();

    applyLoopFilterChips(
      [
        { field: "kind", id: "kind", operator: "is", values: ["workspace"] },
        { field: "category", id: "category", operator: "is", values: [" Operations "] },
        { field: "status", id: "status", operator: "is", values: ["canceled"] },
      ],
      handlers
    );

    expect(handlers.onFiltersChange).toHaveBeenCalledExactlyOnceWith({
      kind: "workspace",
      category: "Operations",
      status: "canceled",
    });
  });

  it("Should clear removed chips and reject invalid filter values", () => {
    const handlers = createHandlers();

    applyLoopFilterChips([], handlers);
    expect(handlers.onFiltersChange).toHaveBeenLastCalledWith({
      kind: "all",
      category: null,
      status: null,
    });

    applyLoopFilterChips(
      [
        { field: "kind", id: "kind", operator: "is", values: ["builtin"] },
        { field: "category", id: "category", operator: "is", values: [" "] },
        { field: "status", id: "status", operator: "is", values: ["stop"] },
      ],
      handlers
    );
    expect(handlers.onFiltersChange).toHaveBeenLastCalledWith({
      kind: "all",
      category: null,
      status: null,
    });
  });
});

describe("buildLoopRunFilterFields", () => {
  it("Should expose origin, session id, and a full-vocabulary outcome select without counts", () => {
    const fields = buildLoopRunFilterFields();
    expect(fields).toHaveLength(3);
    const [origin, session, outcome] = fields;
    if (!origin || !("key" in origin) || !session || !("key" in session)) {
      throw new Error("run filters must be flat fields");
    }
    if (!outcome || !("key" in outcome)) throw new Error("run filters must be flat fields");

    expect(origin.key).toBe("origin");
    expect(origin.type).toBe("select");
    expect(origin.options?.map(option => option.value)).toEqual(["catalog", "session"]);

    expect(session.key).toBe("origin_session");
    expect(session.type).toBe("text");
    expect(session.label).toBe("Session id");

    expect(outcome.key).toBe("outcome");
    expect(outcome.type).toBe("select");
    // The full daemon vocabulary stays selectable regardless of the loaded page (SD-007).
    expect(outcome.options?.map(option => option.value)).toEqual([...LOOP_RUN_STATUSES]);
    expect(outcome.options?.find(option => option.value === "needs-approval")?.label).toBe(
      "Needs approval"
    );
  });
});

describe("loopRunFiltersToChips", () => {
  it("Should project a session id as the single session chip that implies its origin", () => {
    expect(
      loopRunFiltersToChips({ origin: "session", originSession: "session_42", outcome: "all" })
    ).toEqual([
      {
        field: "origin_session",
        id: "loop-run-filter-session",
        operator: "is",
        values: ["session_42"],
      },
    ]);
  });

  it("Should project origin and outcome chips only when they filter", () => {
    expect(loopRunFiltersToChips({ origin: "catalog", outcome: "done" })).toEqual([
      { field: "origin", id: "loop-run-filter-origin", operator: "is", values: ["catalog"] },
      { field: "outcome", id: "loop-run-filter-outcome", operator: "is", values: ["done"] },
    ]);
    expect(loopRunFiltersToChips({ outcome: "all" })).toEqual([]);
  });
});

describe("applyLoopRunFilterChips", () => {
  function createRunHandlers(): LoopRunFilterHandlers {
    return { onOriginFilterChange: vi.fn(), onOutcomeChange: vi.fn() };
  }

  it("Should force the session origin while a session chip exists, even with empty text", () => {
    const handlers = createRunHandlers();

    const resolved = applyLoopRunFilterChips(
      [{ field: "origin_session", id: "chip-1", operator: "is", values: [""] }],
      handlers
    );

    expect(handlers.onOriginFilterChange).toHaveBeenCalledWith({
      origin: "session",
      originSession: undefined,
    });
    expect(resolved).toEqual({ origin: "session", originSession: undefined, outcome: "all" });
  });

  it("Should dispatch a typed session id and outcome to their handlers", () => {
    const handlers = createRunHandlers();

    applyLoopRunFilterChips(
      [
        { field: "origin_session", id: "chip-1", operator: "is", values: ["session_42"] },
        { field: "outcome", id: "chip-2", operator: "is", values: ["done"] },
      ],
      handlers
    );

    expect(handlers.onOriginFilterChange).toHaveBeenCalledWith({
      origin: "session",
      originSession: "session_42",
    });
    expect(handlers.onOutcomeChange).toHaveBeenCalledWith("done");
  });

  it("Should clear both origin params when no origin chips remain and reject unknown values", () => {
    const handlers = createRunHandlers();

    applyLoopRunFilterChips(
      [{ field: "outcome", id: "chip-1", operator: "is", values: ["stop"] }],
      handlers
    );

    expect(handlers.onOriginFilterChange).toHaveBeenCalledWith({
      origin: undefined,
      originSession: undefined,
    });
    expect(handlers.onOutcomeChange).toHaveBeenCalledWith("all");
  });
});
