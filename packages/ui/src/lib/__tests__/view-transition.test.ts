import { afterEach, describe, expect, it, vi } from "vitest";

import { runViewTransition, viewTransitionName } from "../view-transition";

type Doc = Document & { startViewTransition?: unknown };

function stubTransition() {
  const handle = {
    updateCallbackDone: Promise.resolve(),
    finished: Promise.resolve(),
    ready: Promise.resolve(),
  };
  const start = vi.fn((arg: (() => void) | { update: () => void }) => {
    if (typeof arg === "function") arg();
    else arg.update();
    return handle;
  });
  (document as Doc).startViewTransition = start;
  vi.spyOn(window, "matchMedia").mockReturnValue({ matches: false } as MediaQueryList);
  return start;
}

afterEach(() => {
  delete (document as Doc).startViewTransition;
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("runViewTransition", () => {
  it("Should run the update directly when the engine has no view transitions", async () => {
    const update = vi.fn();
    await runViewTransition(update, { types: ["drill-in"] });
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("Should skip the transition but still update when motion is reduced", async () => {
    const start = stubTransition();
    const update = vi.fn();
    await runViewTransition(update, { reduced: true });
    expect(start).not.toHaveBeenCalled();
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("Should pass types when the engine supports transition types", async () => {
    const start = stubTransition();
    vi.stubGlobal("CSS", { supports: () => true });
    const update = vi.fn();
    await runViewTransition(update, { types: ["drill-in"] });
    expect(start).toHaveBeenCalledWith(expect.objectContaining({ types: ["drill-in"] }));
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("Should fall back to a bare callback when transition types are unsupported", async () => {
    const start = stubTransition();
    vi.stubGlobal("CSS", { supports: () => false });
    const update = vi.fn();
    await runViewTransition(update, { types: ["drill-in"] });
    expect(typeof start.mock.calls[0]?.[0]).toBe("function");
    expect(update).toHaveBeenCalledTimes(1);
  });
});

describe("viewTransitionName", () => {
  it("Should build a valid custom ident from arbitrary parts", () => {
    expect(viewTransitionName("agent", "my agent/1", 2)).toBe("vt-agent-my_agent_1-2");
  });
});
