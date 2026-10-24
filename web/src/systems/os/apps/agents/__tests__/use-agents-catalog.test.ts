// Suite: agents fleet visibility
// Invariant: the fleet shows a loading state only while it has no answer, keeps the previous
//   search's rows on screen while an edited search loads, and never reads "no answer yet" (idle
//   or a placeholder with no rows) as an empty fleet.
// Boundary IN: the pure visibility projection of a catalog query's state.
// Boundary OUT: query fetching and list rendering, owned by their suites.
import { describe, expect, it } from "vitest";

import { projectFleetVisibility } from "../use-agents-catalog";

const READY = {
  agents: [{}],
  facets: { total: 3 },
  isError: false,
  isPending: false,
  isPlaceholderData: false,
  total: 3,
};

describe("projectFleetVisibility", () => {
  it("Should read a catalog with no data yet as loading, even while its read is idle", () => {
    const idle = { ...READY, agents: [], facets: undefined, isPending: true, total: 0 };

    expect(projectFleetVisibility(idle, false, {}, false)).toMatchObject({
      isLoading: true,
      isFirstRunEmpty: false,
      showFacets: false,
    });
    expect(projectFleetVisibility(READY, true, {}, false).isLoading).toBe(true);
  });

  it("Should keep the previous search's rows and facets on screen while an edited search loads", () => {
    const placeholder = { ...READY, isPlaceholderData: true };

    expect(projectFleetVisibility(placeholder, false, { q: "rev" }, false)).toMatchObject({
      isLoading: false,
      isFilteredEmpty: false,
      showFacets: true,
    });
  });

  it("Should read a placeholder with no rows as loading, not as no matches", () => {
    const emptyPlaceholder = {
      ...READY,
      agents: [],
      isPlaceholderData: true,
      total: 0,
    };

    expect(projectFleetVisibility(emptyPlaceholder, false, { q: "rev" }, false)).toMatchObject({
      isLoading: true,
      isFilteredEmpty: false,
    });
  });

  it("Should tell a settled empty fleet from a filtered empty one", () => {
    const empty = { ...READY, agents: [], facets: { total: 0 }, total: 0 };

    expect(projectFleetVisibility(empty, false, {}, false).isFirstRunEmpty).toBe(true);
    expect(projectFleetVisibility(empty, false, { q: "zzz" }, false)).toMatchObject({
      isFirstRunEmpty: false,
      isFilteredEmpty: true,
    });
  });
});
