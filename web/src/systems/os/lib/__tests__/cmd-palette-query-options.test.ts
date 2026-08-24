// Suite: command-palette query option contract
// Invariant: every palette read carries the same workspace/profile lens in its cache key and request.
// Owning layer: web/src/systems/os/lib/cmd-palette-query-options.ts.
// Boundary OUT: adapter response decoding and React hook lifecycle.
import { describe, expect, it, vi } from "vitest";

import {
  getCmdPaletteRankSignals,
  getCmdPaletteView,
  listCmdPaletteCommands,
} from "../../adapters/cmd-palette-api";
import {
  cmdPaletteCatalogOptions,
  cmdPaletteRankSignalsOptions,
  cmdPaletteViewOptions,
} from "../cmd-palette-query-options";

vi.mock("../../adapters/cmd-palette-api", () => ({
  getCmdPaletteRankSignals: vi.fn(),
  getCmdPaletteView: vi.fn(),
  listCmdPaletteCommands: vi.fn(),
}));

describe("command-palette query options", () => {
  it("Should forward the profile lens through catalog, view, and rank-signal reads", async () => {
    const signal = new AbortController().signal;
    vi.mocked(listCmdPaletteCommands).mockResolvedValue({} as never);
    vi.mocked(getCmdPaletteView).mockResolvedValue({} as never);
    vi.mocked(getCmdPaletteRankSignals).mockResolvedValue({} as never);

    const catalog = cmdPaletteCatalogOptions(" workspace:alpha ", "marketing", " client:web ");
    const view = cmdPaletteViewOptions(" workspace:alpha ", "marketing", " profiles ");
    const rankSignals = cmdPaletteRankSignalsOptions(" workspace:alpha ", "marketing");

    expect(catalog.queryKey).toEqual([
      "cmd-palette",
      "catalog",
      "workspace:alpha",
      "marketing",
      "client:web",
    ]);
    expect(view.queryKey).toEqual([
      "cmd-palette",
      "views",
      "workspace:alpha",
      "marketing",
      "profiles",
    ]);
    expect(rankSignals.queryKey).toEqual([
      "cmd-palette",
      "rank-signals",
      "workspace:alpha",
      "marketing",
    ]);

    await catalog.queryFn?.({ signal } as never);
    await view.queryFn?.({ signal } as never);
    await rankSignals.queryFn?.({ signal } as never);

    expect(listCmdPaletteCommands).toHaveBeenCalledExactlyOnceWith(
      "workspace:alpha",
      "marketing",
      "client:web",
      signal
    );
    expect(getCmdPaletteView).toHaveBeenCalledExactlyOnceWith(
      "workspace:alpha",
      "marketing",
      "profiles",
      signal
    );
    expect(getCmdPaletteRankSignals).toHaveBeenCalledExactlyOnceWith(
      "workspace:alpha",
      "marketing",
      signal
    );
  });
});
