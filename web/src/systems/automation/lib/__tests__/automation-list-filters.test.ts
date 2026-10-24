// Suite: Automation list filter chips
// Invariant: the strip offers Does · Status · Source · Loop (no Event, no Location), one value
//   each, plus the shared multi-select Project filter only when the listing is under All projects;
//   its selection round-trips to the params the aggregate sends (`workspace_ids`, `@user` for User).
// Boundary IN: URL filter state, the Project filter selection, and FiltersWithSearch chip changes.
// Boundary OUT: FiltersWithSearch rendering (packages/ui).
import { describe, expect, it, vi } from "vitest";

import {
  automationBreadthParams,
  breadthQuery,
  projectFilterField,
  USER_SCOPE,
} from "@/systems/workspace";

import {
  applyAutomationFilterChips,
  automationFiltersToChips,
  buildAutomationFilterFields,
  type AutomationFilterState,
} from "../automation-list-filters";

const empty: AutomationFilterState = {
  target: null,
  enabled: null,
  source: null,
  loop: null,
  projects: [],
};

function handlers() {
  return {
    onTargetChange: vi.fn(),
    onEnabledChange: vi.fn(),
    onSourceChange: vi.fn(),
    onLoopChange: vi.fn(),
    onProjectsChange: vi.fn(),
  };
}

const labels = (fields: ReturnType<typeof buildAutomationFilterFields>) =>
  fields.map(field => ("label" in field ? field.label : ""));

describe("automation list filters", () => {
  it("Should offer four facets and no Event or Location filter outside All projects", () => {
    expect(labels(buildAutomationFilterFields())).toEqual(["Does", "Status", "Source", "Loop"]);
  });

  it("Should label Does in plain words", () => {
    const [does] = buildAutomationFilterFields();
    expect("options" in does ? does.options?.map(option => option.label) : []).toEqual([
      "Ask an agent",
      "Start a Loop",
      "Create a task",
    ]);
  });

  it("Should offer the Project filter under All projects with User last", () => {
    const fields = buildAutomationFilterFields(
      projectFilterField([
        { id: "ws_a", name: "checkout-api" },
        { id: "ws_b", name: "billing" },
      ])
    );
    expect(labels(fields)).toEqual(["Does", "Status", "Project", "Source", "Loop"]);
    const project = fields[2];
    expect("type" in project ? project.type : null).toBe("multiselect");
    expect("options" in project ? project.options?.map(option => option.label) : []).toEqual([
      "checkout-api",
      "billing",
      "User",
    ]);
  });

  it("W-23 sends the selected projects and User as workspace_ids", () => {
    const chips = automationFiltersToChips({ ...empty, projects: ["ws_a", USER_SCOPE] });
    const changes = handlers();
    applyAutomationFilterChips(chips, changes);
    const selected = changes.onProjectsChange.mock.calls[0]?.[0];

    expect(selected).toEqual(["ws_a", "@user"]);
    expect(automationBreadthParams(breadthQuery({ kind: "all" }, selected))).toEqual({
      all_workspaces: true,
      workspace_ids: ["ws_a", "@user"],
    });
    expect(
      automationBreadthParams(breadthQuery({ kind: "project", workspaceId: "ws_a" }, selected))
    ).toEqual({ scope: "workspace", workspace_id: "ws_a" });
  });

  it("Should round-trip applied chips and clear a removed one", () => {
    const state = { ...empty, target: "loop" as const, loop: "software-delivery" };
    const chips = automationFiltersToChips(state);
    expect(chips.map(chip => [chip.field, chip.values[0]])).toEqual([
      ["target", "loop"],
      ["loop", "software-delivery"],
    ]);
    const changes = handlers();
    applyAutomationFilterChips(
      chips.filter(chip => chip.field !== "loop"),
      changes
    );
    expect(changes.onTargetChange).toHaveBeenCalledWith("loop");
    expect(changes.onLoopChange).toHaveBeenCalledWith(null);
    expect(changes.onEnabledChange).toHaveBeenCalledWith(null);
    expect(changes.onProjectsChange).toHaveBeenCalledWith([]);
  });
});
