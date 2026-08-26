import { describe, expect, it } from "vitest";

import { terminalKeys } from "../query-keys";

/** Canonical suite for terminal query identity. */
describe("terminalKeys", () => {
  const scope = { workspaceId: "ws-atlas", profileKey: "work" };

  it("Should give each normalized journal filter set its own cache identity", () => {
    const unfiltered = terminalKeys.journal(scope, {});
    const byActor = terminalKeys.journal(scope, { actor: "agent" });
    const byActorAndResult = terminalKeys.journal(scope, { actor: "agent", failed: true });

    expect(byActor).not.toEqual(unfiltered);
    expect(byActorAndResult).not.toEqual(byActor);
    expect(terminalKeys.journal(scope, { failed: true, actor: "agent" })).toEqual(byActorAndResult);
  });
});
