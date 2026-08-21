// Suite: window-manager query keys
// Invariant: an absent client is a structural null in the config key, never a
// string that could collide with a real client id.
// Owning layer: unit — the query-key factory.
import { describe, expect, it, vi } from "vitest";

import { fetchWindowManagerSnapshot } from "../../adapters/window-manager-api";
import { windowManagerKeys, windowManagerSnapshotOptions } from "../window-manager-query";

vi.mock("../../adapters/window-manager-api", () => ({
  fetchWindowManagerSnapshot: vi.fn(),
}));

describe("windowManagerKeys.config", () => {
  it("Should encode a missing client as null instead of a sentinel string", () => {
    expect(windowManagerKeys.config()).toEqual([
      "settings",
      "section",
      "window-manager",
      "global",
      null,
    ]);
    expect(windowManagerKeys.config("ws-a", "  ")).toEqual([
      "settings",
      "section",
      "window-manager",
      "ws-a",
      null,
    ]);
    expect(windowManagerKeys.config("ws-a", "client:web")).toEqual([
      "settings",
      "section",
      "window-manager",
      "ws-a",
      "client:web",
    ]);
  });
});

describe("windowManagerSnapshotOptions", () => {
  it("Should bind the normalized workspace and profile to its key and request", async () => {
    const signal = new AbortController().signal;
    vi.mocked(fetchWindowManagerSnapshot).mockResolvedValue({} as never);
    const options = windowManagerSnapshotOptions(" workspace:alpha ", " marketing ");

    expect(options.queryKey).toEqual([
      "os",
      "window-manager",
      "snapshot",
      "workspace:alpha",
      "marketing",
    ]);
    expect(options.enabled).toBe(true);
    await options.queryFn?.({ signal } as never);
    expect(fetchWindowManagerSnapshot).toHaveBeenCalledExactlyOnceWith(
      "workspace:alpha",
      "marketing",
      signal
    );
  });

  it("Should disable a snapshot missing either ownership axis", () => {
    expect(windowManagerSnapshotOptions("", "marketing").enabled).toBe(false);
    expect(windowManagerSnapshotOptions("workspace:alpha", " ").enabled).toBe(false);
  });
});
