// Suite: Automation list filter chips
// Invariant: the strip offers Does · Status · Location · Source · Loop (no Event), one value each.
// Boundary IN: URL filter state and FiltersWithSearch chip changes.
// Boundary OUT: FiltersWithSearch rendering (packages/ui).
import { describe, expect, it, vi } from "vitest";

import {
  applyAutomationFilterChips,
  automationFiltersToChips,
  buildAutomationFilterFields,
  type AutomationFilterState,
} from "../automation-list-filters";

const empty: AutomationFilterState = {
  target: null,
  enabled: null,
  scope: null,
  source: null,
  loop: null,
};

describe("automation list filters", () => {
  it("Should offer the five facets and no Event filter", () => {
    expect(
      buildAutomationFilterFields().map(field => ("label" in field ? field.label : ""))
    ).toEqual(["Does", "Status", "Location", "Source", "Loop"]);
  });

  it("Should label Does and Location in plain words", () => {
    const [does, , location] = buildAutomationFilterFields();
    expect("options" in does ? does.options?.map(option => option.label) : []).toEqual([
      "Ask an agent",
      "Start a Loop",
      "Create a task",
    ]);
    expect("options" in location ? location.options?.map(option => option.label) : []).toEqual([
      "This project",
      "Global",
    ]);
  });

  it("Should round-trip applied chips and clear a removed one", () => {
    const state = { ...empty, target: "loop" as const, loop: "software-delivery" };
    const chips = automationFiltersToChips(state);
    expect(chips.map(chip => [chip.field, chip.values[0]])).toEqual([
      ["target", "loop"],
      ["loop", "software-delivery"],
    ]);
    const handlers = {
      onTargetChange: vi.fn(),
      onEnabledChange: vi.fn(),
      onScopeChange: vi.fn(),
      onSourceChange: vi.fn(),
      onLoopChange: vi.fn(),
    };
    applyAutomationFilterChips(
      chips.filter(chip => chip.field !== "loop"),
      handlers
    );
    expect(handlers.onTargetChange).toHaveBeenCalledWith("loop");
    expect(handlers.onLoopChange).toHaveBeenCalledWith(null);
    expect(handlers.onEnabledChange).toHaveBeenCalledWith(null);
  });
});
