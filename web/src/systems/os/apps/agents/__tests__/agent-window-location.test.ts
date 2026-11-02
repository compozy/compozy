// Suite: agents window location
// Invariant: every URL the agents window can hold lands somewhere, including the old `section` and
//   `tab` links from before the profile (UT-075).
// Boundary IN: `parseAgentWindowLocation`. Boundary OUT: the locations it selects.
import { describe, expect, it } from "vitest";

import { parseAgentWindowLocation } from "../agent-window-location";

const at = (pathname: string, search: Record<string, unknown> = {}) =>
  parseAgentWindowLocation({ pathname, search });

describe("parseAgentWindowLocation", () => {
  it("Should open the focused pages", () => {
    expect(at("/agents/triage/check-ins")).toMatchObject({
      kind: "page",
      name: "triage",
      page: "check-ins",
    });
    expect(at("/agents/triage/where")).toMatchObject({ kind: "page", page: "where" });
  });

  it("Should keep a panel on the home", () => {
    expect(at("/agents/triage", { panel: "tools" })).toMatchObject({
      kind: "detail",
      search: { panel: "tools" },
    });
  });

  it.each([
    ["runtime", { kind: "detail", search: { panel: "model" } }],
    ["access", { kind: "detail", search: { panel: "tools" } }],
    ["mcp", { kind: "detail", search: { panel: "tools" } }],
    ["instructions", { kind: "page", page: "what-it-does" }],
    ["basics", { kind: "detail", search: {} }],
  ])("Should land the old %s section on its panel", (section, expected) => {
    expect(at("/agents/triage/settings", { section })).toMatchObject({
      name: "triage",
      ...expected,
    });
  });

  it("Should keep the window binding across an old link", () => {
    expect(
      at("/agents/triage/settings", { section: "runtime", workspace: "ws-compozy" })
    ).toMatchObject({ search: { panel: "model", workspace: "ws-compozy" } });
  });

  it.each([
    ["instructions", { kind: "page", page: "what-it-does" }],
    ["configuration", { kind: "detail", search: { panel: "model" } }],
    ["sessions", { kind: "detail", search: {} }],
  ])("Should land the old %s tab", (tab, expected) => {
    expect(at("/agents/triage", { tab })).toMatchObject(expected);
  });

  it("Should fall back to the list", () => {
    expect(at("/agents", { view: "rows" })).toMatchObject({
      kind: "catalog",
      search: { view: "rows" },
    });
  });
});
