// Suite: command-palette cache identity.
// Invariant (UT-099): every palette query key carries the profile lens, so two
// lenses can never share a cache entry and the SSE prefix walk still sweeps all
// of them in one invalidation.
// Boundary IN: the key factory and its lens normalisation.
// Boundary OUT: what the daemon returns for a lens, and how a switch is driven.
import { describe, expect, it } from "vitest";

import {
  CMD_PALETTE_AGGREGATE_LENS_KEY,
  CMD_PALETTE_NO_CLIENT_KEY,
  cmdPaletteKeys,
  cmdPaletteProfileKey,
} from "../cmd-palette-query-keys";

const WORKSPACE = "ws_alpha";

describe("cmdPaletteKeys (UT-099)", () => {
  it("Should place the profile lens between the workspace and the client", () => {
    expect(cmdPaletteKeys.catalog(WORKSPACE, "marketing", "client-a")).toEqual([
      "cmd-palette",
      "catalog",
      WORKSPACE,
      "marketing",
      "client-a",
    ]);
  });

  it("Should keep two profiles' catalogs on separate entries", () => {
    expect(cmdPaletteKeys.catalog(WORKSPACE, "marketing", "client-a")).not.toEqual(
      cmdPaletteKeys.catalog(WORKSPACE, "consulting", "client-a")
    );
  });

  it("Should give the aggregate its own reserved identity rather than default's", () => {
    expect(
      cmdPaletteKeys.catalog(WORKSPACE, CMD_PALETTE_AGGREGATE_LENS_KEY, "client-a")
    ).not.toEqual(cmdPaletteKeys.catalog(WORKSPACE, "default", "client-a"));
    expect(cmdPaletteProfileKey("")).toBe(CMD_PALETTE_AGGREGATE_LENS_KEY);
  });

  it("Should keep the workspace prefix a parent of every lens and client", () => {
    const prefix = cmdPaletteKeys.workspaceCatalogs(WORKSPACE);
    for (const key of [
      cmdPaletteKeys.catalog(WORKSPACE, "marketing", "client-a"),
      cmdPaletteKeys.catalog(WORKSPACE, "consulting", null),
      cmdPaletteKeys.catalog(WORKSPACE, CMD_PALETTE_AGGREGATE_LENS_KEY, "client-b"),
    ]) {
      expect(key.slice(0, prefix.length)).toEqual([...prefix]);
    }
  });

  it("Should qualify rank signals and views by the lens too", () => {
    expect(cmdPaletteKeys.rankSignals(WORKSPACE, "marketing")).not.toEqual(
      cmdPaletteKeys.rankSignals(WORKSPACE, "default")
    );
    expect(cmdPaletteKeys.view(WORKSPACE, "marketing", "ext.notes.browser")).not.toEqual(
      cmdPaletteKeys.view(WORKSPACE, "default", "ext.notes.browser")
    );
  });

  it("Should still fall back to the unattached client sentinel", () => {
    expect(cmdPaletteKeys.catalog(WORKSPACE, "default", null).at(-1)).toBe(
      CMD_PALETTE_NO_CLIENT_KEY
    );
  });
});
