// Suite: Agents window location parsing
// Invariant: every path the Agents app owns resolves to exactly one location
// kind, and the static Activity/call routes are never swallowed by the dynamic
// `/agents/$name` segment.
// Owning layer: OS app window-location parser. Canonical suite: this file.
import { describe, expect, it } from "vitest";

import { parseAgentWindowLocation } from "../agent-window-location";

function at(pathname: string, search: Record<string, unknown> = {}) {
  return parseAgentWindowLocation({ pathname, search });
}

describe("parseAgentWindowLocation", () => {
  it("Should resolve the catalog as the total fallback", () => {
    expect(at("/agents").kind).toBe("catalog");
    // An unrecognized path still lands somewhere renderable rather than nowhere.
    expect(at("/agents/some/deep/unknown/path").kind).toBe("catalog");
  });

  it("Should resolve Activity before the agent-name segment", () => {
    // The whole reason ordering matters: `/agents/$name` matches "activity" too,
    // and losing this race would render a detail page for an agent that does not
    // exist every time the operator opens Activity.
    expect(at("/agents/activity")).toEqual({ kind: "activity", search: {} });
    expect(at("/agents/activity", { root: "ses_root", call: "call_1", ignored: true })).toEqual({
      kind: "activity",
      search: { root: "ses_root", call: "call_1" },
    });
  });

  it("Should resolve a call record before the agent-name segment", () => {
    expect(at("/agents/calls/call_01JBD8G2K7Q9")).toEqual({
      kind: "call",
      callId: "call_01JBD8G2K7Q9",
    });
  });

  it("Should still resolve an agent literally named like a location word", () => {
    // `/agents/activity` is Activity, but a nested path under that name is not
    // ambiguous and must keep working.
    expect(at("/agents/activity/settings")).toMatchObject({
      kind: "settings",
      name: "activity",
    });
  });

  it("Should decode a percent-encoded call id", () => {
    expect(at("/agents/calls/call%2Fodd")).toMatchObject({ kind: "call", callId: "call/odd" });
  });

  it("Should leave a malformed escape intact rather than throwing", () => {
    expect(at("/agents/calls/call%ZZ")).toMatchObject({ kind: "call", callId: "call%ZZ" });
  });

  it("Should keep resolving the locations that existed before", () => {
    expect(at("/agents/reviewer")).toMatchObject({ kind: "detail", name: "reviewer" });
    expect(at("/agents/reviewer/settings")).toMatchObject({ kind: "settings", name: "reviewer" });
  });
});
