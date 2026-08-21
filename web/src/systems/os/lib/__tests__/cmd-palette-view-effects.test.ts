// Suite: command palette programmable-view effect delivery
// Invariant: every failed client effect is correlated by effect_id while the whole settled batch
// remains acknowledgeable, preserving the at-most-once fence.
// Boundary IN: settled browser effect executions.
// Boundary OUT: effect implementations, HTTP acknowledgement transport, and daemon replay.

import { describe, expect, it, vi } from "vitest";

import type { CmdPaletteViewEffect } from "../cmd-palette-types";
import { finalizeCmdPaletteViewEffects } from "../cmd-palette-view-effects";

describe("command palette programmable-view effect delivery", () => {
  it("Should log a failed effect with its stable id and acknowledge the settled batch [IT-033]", () => {
    const effects: CmdPaletteViewEffect[] = [
      { id: "ef_copy", copy: { content: "copy me" } },
      { id: "ef_toast", toast: { message: "Saved", tone: "success" } },
    ];
    const failure = new Error("clipboard denied");
    const reportFailure = vi.fn();

    const acknowledged = finalizeCmdPaletteViewEffects(
      effects,
      [
        { status: "rejected", reason: failure },
        { status: "fulfilled", value: undefined },
      ],
      reportFailure
    );

    expect(reportFailure).toHaveBeenCalledExactlyOnceWith("Command palette view effect failed", {
      effect_id: "ef_copy",
      error: failure,
    });
    expect(acknowledged).toEqual(["ef_copy", "ef_toast"]);
  });
});
