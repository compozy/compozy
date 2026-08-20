// Suite: live palette client context
// Invariant: chrome publishes the current scope, focused session, destination
// route, and a fail-closed trust echo instead of the registration snapshot.
// Owning layer: unit — the chrome context helper.
import { describe, expect, it } from "vitest";

import { resolveLivePaletteClientContext } from "../desktop-chrome-client-context";

describe("resolveLivePaletteClientContext", () => {
  it("Should publish live shell fields and fail closed when trust is unknown", () => {
    expect(
      resolveLivePaletteClientContext({
        scope: "global",
        focusedSessionState: "waiting-for-input",
        registeredWorkspaceTrusted: undefined,
        destinationRoute: { pathname: "/tasks", search: { query: "review" } },
        globalShortcuts: [],
      })
    ).toEqual({
      scopeGlobal: true,
      focusedSessionState: "waiting-for-input",
      workspaceTrusted: false,
      destinationIntent: { pathname: "/tasks", search: { query: "review" } },
      globalShortcuts: [],
    });
  });

  it("Should echo the registered trust bit when the daemon already supplied one", () => {
    expect(
      resolveLivePaletteClientContext({
        scope: "workspace",
        focusedSessionState: null,
        registeredWorkspaceTrusted: true,
        destinationRoute: null,
        globalShortcuts: [],
      }).workspaceTrusted
    ).toBe(true);
  });
});
