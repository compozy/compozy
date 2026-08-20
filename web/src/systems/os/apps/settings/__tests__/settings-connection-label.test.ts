// Suite: Settings connection label
// Invariant: every daemon connection state maps to its truthful Settings status label.
// Boundary IN: the pure ConnectionStatus-to-label presentation mapping.
// Boundary OUT: ConnectionIndicator rendering and daemon connection state acquisition.
import { describe, expect, it } from "vitest";

import { settingsConnectionLabel } from "../settings-connection-label";

describe("settingsConnectionLabel", () => {
  it.each([
    ["connected", "CompozyOS running"],
    ["connecting", "CompozyOS connecting"],
    ["disconnected", "CompozyOS unreachable"],
    ["error", "CompozyOS unreachable"],
  ] as const)("maps %s to truthful copy", (state, label) => {
    expect(settingsConnectionLabel(state)).toBe(label);
  });
});
