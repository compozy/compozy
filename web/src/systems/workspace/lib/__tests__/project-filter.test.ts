// Suite: shared listing Project filter field
// Invariant: the field lists projects in switcher order with User always last, offers only the
// inclusive operator, and its chip round-trips the selected scopes; an empty filter is no chip
// (every project), and a removed chip is the empty filter.
// Owning layer: workspace lib. Canonical suite: this file (the params are owned by breadth-query).
import { describe, expect, it } from "vitest";

import { USER_SCOPE } from "../breadth-query";
import {
  PROJECT_FILTER_FIELD,
  projectFilterChip,
  projectFilterField,
  projectFilterValuesFromChips,
} from "../project-filter";

describe("projectFilterField", () => {
  it("Should list the projects in order, then User last, as one inclusive multi-select", () => {
    const field = projectFilterField([
      { id: "ws_b", name: "Beta" },
      { id: "ws_a", name: "Alpha" },
    ]);

    expect(field).toMatchObject({
      key: PROJECT_FILTER_FIELD,
      label: "Project",
      type: "multiselect",
    });
    expect(field.options?.map(option => [option.value, option.label])).toEqual([
      ["ws_b", "Beta"],
      ["ws_a", "Alpha"],
      [USER_SCOPE, "User"],
    ]);
    expect(field.operators?.map(operator => operator.value)).toEqual(["is_any_of"]);
  });

  it("Should keep User as the only option with zero projects", () => {
    expect(projectFilterField([]).options?.map(option => option.value)).toEqual([USER_SCOPE]);
  });
});

describe("projectFilterChip", () => {
  it("Should round-trip the selected scopes through the chip array", () => {
    const chip = projectFilterChip(["ws_a", USER_SCOPE]);

    expect(chip).not.toBeNull();
    expect(projectFilterValuesFromChips(chip ? [chip] : [])).toEqual(["ws_a", USER_SCOPE]);
  });

  it("Should render no chip for the empty filter and read a removed chip as empty", () => {
    expect(projectFilterChip([])).toBeNull();
    expect(
      projectFilterValuesFromChips([
        { id: "status", field: "status", operator: "is", values: ["ready"] },
      ])
    ).toEqual([]);
  });
});
