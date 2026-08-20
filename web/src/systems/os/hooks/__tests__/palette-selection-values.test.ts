// Suite: palette selection keys
// Invariant: selection values are derived from rowSources plus the fallback row.
// Boundary IN: paletteSelectionValues.
// Boundary OUT: action-panel subject resolution.
import { describe, expect, it } from "vitest";

import { paletteSelectionValues } from "../os-palette-selection-values";

describe("palette selection values", () => {
  it("Should derive keys from row sources and append only the fallback [RA0302]", () => {
    const values = paletteSelectionValues({
      rowSources: {
        commands: [{ id: "window.close" }],
        sessions: [{ sessionId: "s1" }],
        tabs: [{ windowId: "w1" }],
        worktrees: [{ key: "wt1" }],
        domainRows: [{ key: "task:1" }],
      },
      fallback: { value: "ask:general" },
    });

    expect(values).toEqual([
      "window.close",
      "ask:general",
      "session:s1",
      "tab:w1",
      "worktree:wt1",
      "task:1",
    ]);
  });
});
