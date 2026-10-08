import { describe, expect, it } from "vitest";

import { resolveCompactionCommand } from "../session-compaction";

const named = (...names: string[]) => names.map(name => ({ name }));

// Suite: compaction command selection (UT-W09, daemon parity with `session.ResolveCompactionCommand`).
// Invariant: the action is offered for exactly the names the daemon would accept, so a request
// the daemon would refuse as `compaction_unsupported` is never invited. `compact` outranks
// `compress` whatever the advertised order; names are matched exactly after trimming.
// Boundary IN: a session's advertised commands. Boundary OUT: the button and the request.
describe("resolveCompactionCommand", () => {
  it.each([
    { commands: ["compact"], expected: "compact" },
    { commands: ["compress"], expected: "compress" },
    { commands: ["compress", "compact"], expected: "compact" },
    { commands: ["compact", "compress"], expected: "compact" },
    { commands: [" compact "], expected: "compact" },
    { commands: ["review"], expected: null },
    { commands: [], expected: null },
  ])("Should resolve $commands to $expected", ({ commands, expected }) => {
    expect(resolveCompactionCommand(named(...commands))).toBe(expected);
  });

  it("Should not match a spelling the daemon would not accept", () => {
    expect(
      resolveCompactionCommand(named("/compact", "Compact", "compact-now", "compression"))
    ).toBe(null);
  });
});
