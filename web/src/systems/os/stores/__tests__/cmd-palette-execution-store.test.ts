// Suite: command palette execution pending map
// Invariant: two overlapping invokes of the same command stay pending until both settle.
// Boundary IN: pendingStarted / pendingSettled transitions.
// Boundary OUT: toast UI and the dispatch seam that emits the events.
import { describe, expect, it } from "vitest";

import { cmdPaletteExecutionStore } from "../cmd-palette-execution-store";

describe("cmd-palette execution pending refcount", () => {
  it("Should keep a command pending until every overlapping invoke settles [RD0067]", () => {
    const start = {
      type: "pendingStarted" as const,
      pending: { commandId: "tools.toggle", title: "Toggle" },
    };
    let snapshot = cmdPaletteExecutionStore.getInitialSnapshot();
    [snapshot] = cmdPaletteExecutionStore.transition(snapshot, start);
    [snapshot] = cmdPaletteExecutionStore.transition(snapshot, start);
    expect(snapshot.context.pending["tools.toggle"]?.inFlight).toBe(2);

    [snapshot] = cmdPaletteExecutionStore.transition(snapshot, {
      type: "pendingSettled",
      commandId: "tools.toggle",
    });
    expect(snapshot.context.pending["tools.toggle"]?.inFlight).toBe(1);

    [snapshot] = cmdPaletteExecutionStore.transition(snapshot, {
      type: "pendingSettled",
      commandId: "tools.toggle",
    });
    expect(snapshot.context.pending["tools.toggle"]).toBeUndefined();
  });

  it("Should ignore a settle for a command that is not pending [RA0256]", () => {
    const snapshot = cmdPaletteExecutionStore.getInitialSnapshot();
    const [next] = cmdPaletteExecutionStore.transition(snapshot, {
      type: "pendingSettled",
      commandId: "missing",
    });
    expect(next.context.pending).toEqual({});
  });
});
