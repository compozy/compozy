import { describe, expect, it } from "vitest";

import { terminalKeys } from "../query-keys";

/** Canonical suite for terminal query identity. */
describe("terminalKeys", () => {
  const scope = { workspaceId: "ws-atlas", profileKey: "work" };

  it("Should isolate live recordings from a downloaded recording artifact", () => {
    const recordings = terminalKeys.recordings(scope);
    const artifact = terminalKeys.recording(scope, "rec-1");

    expect(recordings).toEqual(["terminal", "recordings", "ws-atlas", "work"]);
    expect(artifact).not.toEqual(recordings);
    expect(terminalKeys.recordings({ workspaceId: "ws-other", profileKey: "work" })).not.toEqual(
      recordings
    );
    expect(
      terminalKeys.recordings({ workspaceId: "ws-atlas", profileKey: "personal" })
    ).not.toEqual(recordings);
  });

  it("Should give each normalized journal filter set its own cache identity", () => {
    const unfiltered = terminalKeys.journal(scope, {});
    const byActor = terminalKeys.journal(scope, { actor: "agent" });
    const byActorAndResult = terminalKeys.journal(scope, { actor: "agent", failed: true });

    expect(byActor).not.toEqual(unfiltered);
    expect(byActorAndResult).not.toEqual(byActor);
    expect(terminalKeys.journal(scope, { failed: true, actor: "agent" })).toEqual(byActorAndResult);
  });
});
