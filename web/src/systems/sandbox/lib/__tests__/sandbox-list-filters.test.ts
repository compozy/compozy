// Suite: Sandbox list filter chips
// Invariant: URL-backed backend and persistence filters round-trip through the
// chip bar, with cleared or invalid chips restoring the unfiltered state.
// Boundary IN: sandbox filter state <-> chip projection/application.
// Boundary OUT: URL persistence (useSandboxPage) and @agh/ui Filters rendering.
import { describe, expect, it, vi } from "vitest";

import {
  applySandboxFilterChips,
  buildSandboxFilterFields,
  sandboxFiltersToChips,
  type SandboxFilterHandlers,
} from "../sandbox-list-filters";

function createHandlers(): SandboxFilterHandlers {
  return {
    onBackendChange: vi.fn(),
    onPersistenceChange: vi.fn(),
  };
}

describe("buildSandboxFilterFields", () => {
  it("Should expose every supported backend and persistence value", () => {
    const fields = buildSandboxFilterFields();

    const selectFields = fields.map(field => {
      if (!("key" in field) || !("options" in field)) {
        throw new Error("sandbox filters must be flat select fields");
      }
      return field;
    });

    expect(selectFields.map(field => field.key)).toEqual(["backend", "persistence"]);
    expect(selectFields[0]?.options?.map(option => option.value)).toEqual([
      "local",
      "daytona",
      "e2b",
    ]);
    expect(selectFields[1]?.options?.map(option => option.value)).toEqual([
      "transient",
      "reuse",
      "archive",
    ]);
  });
});

describe("sandboxFiltersToChips", () => {
  it("Should project only concrete backend and persistence filters", () => {
    expect(sandboxFiltersToChips({ backend: "daytona", persistence: "reuse" })).toEqual([
      {
        field: "backend",
        id: "sandbox-filter-backend",
        operator: "is",
        values: ["daytona"],
      },
      {
        field: "persistence",
        id: "sandbox-filter-persistence",
        operator: "is",
        values: ["reuse"],
      },
    ]);
    expect(sandboxFiltersToChips({ backend: "all", persistence: "all" })).toEqual([]);
  });
});

describe("applySandboxFilterChips", () => {
  it("Should dispatch both chips to their typed handlers", () => {
    const handlers = createHandlers();

    applySandboxFilterChips(
      [
        {
          field: "backend",
          id: "sandbox-filter-backend",
          operator: "is",
          values: ["e2b"],
        },
        {
          field: "persistence",
          id: "sandbox-filter-persistence",
          operator: "is",
          values: ["archive"],
        },
      ],
      handlers
    );

    expect(handlers.onBackendChange).toHaveBeenCalledWith("e2b");
    expect(handlers.onPersistenceChange).toHaveBeenCalledWith("archive");
  });

  it("Should reset both filters when chips are cleared or invalid", () => {
    const handlers = createHandlers();

    applySandboxFilterChips(
      [{ field: "backend", id: "bad", operator: "is", values: ["unknown"] }],
      handlers
    );

    expect(handlers.onBackendChange).toHaveBeenCalledWith("all");
    expect(handlers.onPersistenceChange).toHaveBeenCalledWith("all");
  });
});
