// Suite: Automation list filter chips
// Invariant: URL-backed filter state round-trips through the chip bar — state
// projects to one chip per active field, and applying chips dispatches the
// typed handler per field (invalid values fall back to null). Ports the
// filter-forwarding coverage lost with the master-detail list panel.
// Boundary IN: filter state <-> chip projection/application.
// Boundary OUT: URL persistence (page-base) and chip rendering (@agh/ui Filters).
import { describe, expect, it, vi } from "vitest";

import {
  applyAutomationFilterChips,
  automationFiltersToChips,
  buildAutomationFilterFields,
  type AutomationFilterHandlers,
} from "../automation-list-filters";

function createHandlers(): AutomationFilterHandlers {
  return {
    onEnabledChange: vi.fn(),
    onEventChange: vi.fn(),
    onScopeChange: vi.fn(),
    onSourceChange: vi.fn(),
  };
}

describe("buildAutomationFilterFields", () => {
  it("Should expose the event field only for triggers", () => {
    const fieldKeys = (kind: "jobs" | "triggers") =>
      buildAutomationFilterFields(kind).map(field => ("key" in field ? field.key : null));

    expect(fieldKeys("jobs")).toEqual(["scope", "source", "enabled"]);
    expect(fieldKeys("triggers")).toEqual(["scope", "source", "enabled", "event"]);
  });
});

describe("automationFiltersToChips", () => {
  it("Should project only active filters as chips and skip defaults", () => {
    const chips = automationFiltersToChips({
      enabled: false,
      event: "ext.github.push",
      scope: "workspace",
      source: null,
    });

    expect(chips.map(chip => [chip.field, chip.values[0]])).toEqual([
      ["scope", "workspace"],
      ["enabled", "false"],
      ["event", "ext.github.push"],
    ]);
  });

  it("Should project no chips for the default state", () => {
    expect(
      automationFiltersToChips({ enabled: null, event: null, scope: "all", source: null })
    ).toEqual([]);
  });
});

describe("applyAutomationFilterChips", () => {
  it("Should dispatch each chip to its typed handler", () => {
    const handlers = createHandlers();
    applyAutomationFilterChips(
      [
        { field: "scope", id: "automation-filter-scope", operator: "is", values: ["global"] },
        { field: "source", id: "automation-filter-source", operator: "is", values: ["dynamic"] },
        { field: "enabled", id: "automation-filter-enabled", operator: "is", values: ["true"] },
        {
          field: "event",
          id: "automation-filter-event",
          operator: "is",
          values: [" ext.github.push "],
        },
      ],
      handlers
    );

    expect(handlers.onScopeChange).toHaveBeenCalledWith("global");
    expect(handlers.onSourceChange).toHaveBeenCalledWith("dynamic");
    expect(handlers.onEnabledChange).toHaveBeenCalledWith(true);
    expect(handlers.onEventChange).toHaveBeenCalledWith("ext.github.push");
  });

  it("Should reset every handler to null when chips are cleared or invalid", () => {
    const handlers = createHandlers();
    applyAutomationFilterChips(
      [{ field: "scope", id: "automation-filter-scope", operator: "is", values: ["bogus"] }],
      handlers
    );

    expect(handlers.onScopeChange).toHaveBeenCalledWith(null);
    expect(handlers.onSourceChange).toHaveBeenCalledWith(null);
    expect(handlers.onEnabledChange).toHaveBeenCalledWith(null);
    expect(handlers.onEventChange).toHaveBeenCalledWith(null);
  });
});
